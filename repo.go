package main

import (
	"fmt"
	"log"
	"net/url"
	"os"
	"os/exec"
	"path"
	"reflect"
	"regexp"
	"strings"
)

var repoRegexp *regexp.Regexp

func init() {
	var err error
	if repoRegexp, err = regexp.Compile(`\s+`); err != nil {
		log.Fatalf("Unable to compile regexp: %s", err)
	}
}

// Repository represents a Debian repository
type Repository struct {
	Architecture string
	URL          *url.URL
	Version      string
	Components   []string
	archPath     string
	PkgList      string
}

// ParseRepo parses repo-specifications in configuration into Repository structure
func ParseRepo(conf, defaultArch string) (ret []Repository, err error) {
	conf = strings.TrimSpace(conf)
	conf = repoRegexp.ReplaceAllString(conf, ` `)
	tokens := repoRegexp.Split(conf, -1)
	var arch, ver string
	var uri *url.URL

	// parse arch
	if tokens[0] == "deb" {
		arch = defaultArch
	} else if tokens[0][0:4] == "deb-" {
		arch = tokens[0][4:]
	}
	if arch == "" {
		return ret, fmt.Errorf("Unable to parse architacture: %s", tokens[0])
	}

	archPath := "binary-" + arch
	pkgList := "Packages"
	if arch == "src" {
		archPath = "source"
		pkgList = "Sources"
	}

	// parse uri
	if tokens[1][len(tokens[1])-1:] != "/" {
		tokens[1] += "/"
	}
	if uri, err = url.Parse(tokens[1]); err != nil {
		return
	}

	// parse version (stable, unstable, testing...)
	ver = tokens[2]

	ret = make([]Repository, 1, 2)
	ret[0] = Repository{
		Architecture: arch,
		URL:          uri,
		Version:      ver,
		Components:   tokens[3:],
		archPath:     archPath,
		PkgList:      pkgList,
	}
	if arch != "src" {
		// we have to download binary-all also
		ret = append(ret, Repository{
			Architecture: "all",
			URL:          uri,
			Version:      ver,
			Components:   tokens[3:],
			archPath:     archPath,
			PkgList:      pkgList,
		})
	}
	return
}

// Equals tests if two Repository structures are same.
func (r Repository) Equals(a Repository) bool {
	return r.Architecture == a.Architecture &&
		r.URL.String() == a.URL.String() &&
		r.Version == a.Version &&
		reflect.DeepEqual(r.Components, a.Components)
}

// File returns the url of the file path in this repository.
func (r Repository) File(path string) (ret *url.URL) {
	ret, err := r.URL.Parse(path)
	if err != nil {
		log.Fatalf("It should not happend! URL parsing error with path %s: %s", path, err)
	}
	return
}

// InfoFiles returns url of info files (Contents and Release files)
func (r Repository) InfoFiles() (ret []*url.URL) {
	comps := len(r.Components)
	ret = make([]*url.URL, 2+comps*3)
	idx := 2
	ret[0] = r.File(fmt.Sprintf("dists/%s/Release", r.Version))
	ret[1] = r.File(fmt.Sprintf("dists/%s/Release.gpg", r.Version))
	for _, c := range r.Components {
		ret[idx] = r.File(fmt.Sprintf(
			"dists/%s/%s/Contents-%s",
			r.Version, c, r.Architecture))
		idx++
		ret[idx] = r.File(fmt.Sprintf(
			"dists/%s/%s/Contents-%s.gz",
			r.Version, c, r.Architecture))
		idx++
		ret[idx] = r.File(fmt.Sprintf(
			"dists/%s/%s/%s/Release",
			r.Version, c, r.archPath))
		idx++
	}
	return
}

// Packages returns url of Debian package list file (Packages or Sources file).
func (r Repository) Packages(c string) *url.URL {
	return r.File(fmt.Sprintf(
		"dists/%s/%s/%s/%s",
		r.Version, c, r.archPath, r.PkgList))
}

// PackagesGZ gzipped-file version of Packages() method.
func (r Repository) PackagesGZ(c string) *url.URL {
	return r.File(fmt.Sprintf(
		"dists/%s/%s/%s/%s.gz",
		r.Version, c, r.archPath, r.PkgList))
}

// I18N returns i18n file of the language.
func (r Repository) I18N(lang string) []*url.URL {
	ret := make([]*url.URL, len(r.Components))
	idx := 0
	for _, c := range r.Components {
		ret[idx] = r.File(fmt.Sprintf(
			"dists/%s/%s/i18n/Translation-%s.bz2",
			r.Version, c, lang))
		idx++
	}
	return ret
}

// DownloadInfoFiles downloads all info files.
// It will return as soon as Debian package list files are downloaded,
// leave other files download in background.
func (r Repository) DownloadInfoFiles(cfg *Config, dlMgr *DownloadManager) {
	// do the download work, and return decompressing tool needed to decompress downloaded data
	down := func(u *url.URL) (ret string, ext string) {
		dst := cfg.SkelPath(u)
		resp, err := dlMgr.Dispatch(u).Download(u, dst)
		if err != nil {
			return
		}
		log.Printf("Info file %s downloaded", u)

		switch resp.Header.Get("Content-Type") {
		case "application/x-gzip":
			ret = "gzip"
			ext = ".gz"
		case "application/x-xz":
			ret = "xz"
			ext = ".xz"
		case "application/x-bzip2":
			ret = "bzip2"
			ext = ".bz2"
		}
		return
	}
	// do decompressing job
	decomp := func(u *url.URL, fn, tool, ext string) {
		path := cfg.SkelPath(u)
		log.Printf("Decompressing %s with %s", fn, tool)
		nf := path + ext
		if err := os.Rename(path, nf); err != nil {
			// no matter success or not, run further
			log.Printf("Cannot rename %s to %s, ignored: %s", path, nf, err)
		}

		if err := exec.Command(tool, "-dfkq", nf).Run(); err != nil {
			// no matter success or not, run further
			log.Printf("Cannot decompress %s using %s, ignored: %s", nf, tool, err)
		}
	}

	// download info files in background
	go func() {
		for _, u := range r.InfoFiles() {
			tool, ext := down(u)

			// some mirror sites will send compressed file instead of plain files.
			// decompress it when such situation.
			if fn := path.Base(u.Path); ext != "" && fn[len(fn)-len(ext):] != ext {
				decomp(u, fn, tool, ext)
			}
		}

		// download translations
		transStr := cfg.Variables["translations"]
		if transStr == "" {
			return
		}

		arr := repoRegexp.Split(transStr, -1)
		for _, t := range arr {
			if t == "" {
				continue
			}
			for _, u := range r.I18N(t) {
				down(u)
			}
		}
	}()

	// process Packages or Sources file.
	for _, c := range r.Components {
		u := r.Packages(c)
		tool, ext := down(u)
		if tool != "" {
			decomp(u, path.Base(u.Path), tool, ext)
		}
		down(r.PackagesGZ(c))
	}
}

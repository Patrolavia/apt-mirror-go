package main

import (
	"bufio"
	"io"
	"net/url"
	"os"
	"path"
	"strconv"
	"strings"
)

// Package denotes a remote Debian package file.
type Package struct {
	URL    *url.URL
	Size   int64
	MD5Sum string
}

// ParsePackage parses Debian Packages or Sources file to find out all package files.
func ParsePackage(repo Repository, r io.Reader) (ret []Package, err error) {
	// src is how we parse Debian Sources file
	src := func(p string) (err error) {
		c := ParseControlFile(p)
		fs, fsok := c["Files"]
		dir := strings.TrimSpace(c.Get("Directory"))
		if !fsok || dir == "" {
			return
		}
		for _, f := range fs {
			data := strings.Split(strings.TrimSpace(f), " ")
			if len(data) != 3 {
				continue
			}
			u := repo.File(path.Join(dir, data[2]))

			sz, err := strconv.ParseInt(data[1], 10, 64)
			if err != nil {
				return err
			}
			ret = append(ret, Package{u, sz, data[0]})
		}
		return
	}
	// bin is how we parse Debian Packages file
	bin := func(p string) (err error) {
		c := ParseControlFile(p)
		f := strings.TrimSpace(c.Get("Filename"))
		s := strings.TrimSpace(c.Get("Size"))
		m := strings.TrimSpace(c.Get("MD5sum"))
		if f == "" || s == "" || m == "" {
			return
		}
		u := repo.File(f)
		sz, err := strconv.ParseInt(s, 10, 64)
		if err != nil {
			return
		}
		ret = append(ret, Package{u, sz, m})
		return
	}

	do := bin
	if repo.Architecture == "src" {
		do = src
	}
	ret = make([]Package, 0)

	// process line by line to save memory
	scanner := bufio.NewScanner(r)
	pkgStr := ""
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" { // empty line delimits different packages
			pkgStr += "\n" + line
			continue
		}

		if pkgStr == "" { // no package difination, skip
			continue
		}

		if err = do(pkgStr); err != nil {
			return
		}
		pkgStr = ""
	}

	if pkgStr != "" {
		err = do(pkgStr)
	}
	return
}

func (p Package) test(path string) bool {
	f, err := os.Stat(path)
	if err != nil {
		return false
	}

	if f.Size() != p.Size {
		return false
	}

	/* temporarily comment out md5 check, it's too slow
	res, err := exec.Command("md5sum", path).Output()
	if err != nil {
		return false
	}

	return string(res)[0:32] == p.MD5Sum
	*/

	return true
}

// Test tests if we have this Debian package on disk now.
func (p Package) Test(cfg *Config) bool {
	mirrorPath := cfg.MirrorPath(p.URL)
	skelPath := cfg.SkelPath(p.URL)

	return p.test(mirrorPath) || p.test(skelPath)
}

// Download will download the Debian package into temporary (skel) directory
func (p Package) Download(cfg *Config, agent Downloader) error {
	skelPath := cfg.SkelPath(p.URL)
	_, err := agent.Download(p.URL, skelPath)
	return err
}

package main

import (
	"bufio"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path"
	"strconv"
	"strings"
	"github.com/juju/ratelimit"
)

type Package struct {
	URL    *url.URL
	Size   int64
	MD5Sum string
}

func ParsePackage(repo Repository, r io.Reader) (ret []Package, err error) {
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
	scanner := bufio.NewScanner(r)
	pkgStr := ""
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" {
			pkgStr += "\n" + line
			continue
		}

		if pkgStr == "" {
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

func (p Package) Test(cfg *Config) bool {
	mirrorPath := cfg.MirrorPath(p.URL)
	skelPath := cfg.SkelPath(p.URL)

	return p.test(mirrorPath) || p.test(skelPath)
}

func (p Package) downloadHTTP(cfg *Config) error {
	req, err := http.NewRequest("GET", p.URL.String(), nil)
	if err != nil {
		return err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	skelPath := cfg.SkelPath(p.URL)

	if err = os.MkdirAll(path.Dir(skelPath), 0755); err != nil {
		return err
	}

	f, err := os.Create(skelPath)
	if err != nil {
		return err
	}
	defer f.Close()

	var src io.Reader = resp.Body

	if bucket != nil {
		src = ratelimit.Reader(resp.Body, bucket)
	}
	_, err = io.Copy(f, src)
	return err
}

func (p Package) Download(cfg *Config) error {
	if p.URL.Scheme == "http" {
		return p.downloadHTTP(cfg)
	}
	log.Printf("URL scheme %s is not supported.", p.URL.Scheme)
	return nil
}

func (p Package) Move(cfg *Config) error {
	src := cfg.SkelPath(p.URL)
	dst := cfg.MirrorPath(p.URL)

	if err := os.MkdirAll(path.Dir(dst), 0755); err != nil {
		return err
	}
	return os.Rename(src, dst)
}

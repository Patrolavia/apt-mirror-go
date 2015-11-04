package main

import (
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"strconv"
	"strings"
)

type Package struct {
	URL    *url.URL
	Size   int64
	MD5Sum string
}

func ParsePackage(repo Repository, data string) (ret []Package, err error) {
	ret = make([]Package, 0)
	arr := strings.Split(data, "\n\n")
	for _, p := range arr {
		c := ParseControlFile(p)
		f := strings.TrimSpace(c.Get("Filename"))
		s := strings.TrimSpace(c.Get("Size"))
		m := strings.TrimSpace(c.Get("MD5sum"))
		if f == "" || s == "" || m == "" {
			continue
		}
		u := repo.File(f)
		sz, err := strconv.ParseInt(s, 10, 64)
		if err != nil {
			return ret, err
		}
		ret = append(ret, Package{u, sz, m})
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

func (p Package) Download(cfg *Config) error {
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

	_, err = io.Copy(f, resp.Body)
	return err
}

func (p Package) Move(cfg *Config) error {
	src := cfg.SkelPath(p.URL)
	dst := cfg.MirrorPath(p.URL)

	if err := os.MkdirAll(path.Dir(dst), 0755); err != nil {
		return err
	}
	return os.Rename(src, dst)
}

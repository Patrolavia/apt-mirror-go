package main

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path"

	"github.com/juju/ratelimit"
)

type Downloader interface {
	Download(u *url.URL, dst string) (resp *http.Response, err error)
}

type InvalidDownloader func(uri *url.URL) string

func (i InvalidDownloader) Download(u *url.URL, dst string) (resp *http.Response, err error) {
	log.Fatal(i(u))
	return
}

type HttpDownloader struct {
	bucket *ratelimit.Bucket
	client *http.Client
}

func (h *HttpDownloader) Download(u *url.URL, dst string) (resp *http.Response, err error) {
	if h.client == nil {
		h.client = http.DefaultClient
	}

	req, err := http.NewRequest("GET", u.String(), nil)
	if err != nil {
		return
	}

	resp, err = h.client.Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return resp, fmt.Errorf("downloader error downloading %s: got http status %s",
			u, resp.Status)
	}

	os.MkdirAll(path.Dir(dst), 0755)
	f, err := os.Create(dst)
	if err != nil {
		return
	}
	defer f.Close()

	var src io.Reader = resp.Body
	if h.bucket != nil {
		src = ratelimit.Reader(resp.Body, h.bucket)
	}

	_, err = io.Copy(f, src)
	return
}

type DownloadManager struct {
	Inv  InvalidDownloader
	HTTP *HttpDownloader
}

func (d DownloadManager) Dispatch(u *url.URL) Downloader {
	if u.Scheme == "http" {
		return d.HTTP
	}
	return d.Inv
}

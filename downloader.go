package main

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path"

	"github.com/Patrolavia/ratelimit"
)

// Downloader is an agent to download some kind of url
type Downloader interface {
	Download(u *url.URL, dst string) (resp *http.Response, err error)
}

type invalidDownloader struct {
	logger func(uri *url.URL) string
	ch     chan int
}

func (i *invalidDownloader) Download(u *url.URL, dst string) (resp *http.Response, err error) {
	log.Fatal(i.logger(u))
	<-i.ch
	return
}

type httpDownloader struct {
	bucket *ratelimit.Bucket
	client *http.Client
	ch     chan int
}

func (h *httpDownloader) Download(u *url.URL, dst string) (resp *http.Response, err error) {
	defer func() { <-h.ch }()
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
		src = ratelimit.NewReader(resp.Body, h.bucket)
	}

	_, err = io.Copy(f, src)
	return
}

// DownloadManager dispatches url to correct downloader, and manages
// the number of concurrent downloads.
type DownloadManager struct {
	inv  *invalidDownloader
	http *httpDownloader
	ch   chan int
}

/*
NewManager creates a new DownloadManager.

  logger is a function produce error message when there's unsupported url.
  bucker is rate limiter.
  client is http client to download data via http protocol.
  max is max number of concurrent downloads.
*/
func NewManager(
	logger func(uri *url.URL) string,
	bucket *ratelimit.Bucket,
	client *http.Client,
	max int,
) *DownloadManager {

	ch := make(chan int, max)

	return &DownloadManager{
		inv:  &invalidDownloader{logger, ch},
		http: &httpDownloader{bucket, client, ch},
		ch:   ch,
	}
}

// Dispatch returns correct download agnet for the url.
func (d DownloadManager) Dispatch(u *url.URL) Downloader {
	d.ch <- 1
	if u.Scheme == "http" {
		return d.http
	}
	return d.inv
}

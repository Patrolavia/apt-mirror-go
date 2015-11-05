package main

import (
	"io/ioutil"
	"net/url"
	"testing"
)

func TestParseConf(t *testing.T) {
	expectVar := map[string]string{
		"base_path":    "/data/apt-mirror",
		"mirror_path":  "/data/apt-mirror/mirror",
		"skel_path":    "/data/apt-mirror/skel",
		"nthreads":     "5",
		"defaultarch":  "myarch",
		"translations": "en zh_TW",
	}

	str, err := ioutil.ReadFile("conf.sample")
	if err != nil {
		t.Fatalf("Cannot read config sample from filesystem: %s", err)
	}

	cfg, err := ParseConfig(string(str))
	if err != nil {
		t.Fatalf("Parse error: %s", err)
	}

	for k, v := range expectVar {
		if cfg.Variables[k] != v {
			t.Errorf("Expected var %#v is %#v, got %#v", k, v, cfg.Variables[k])
		}
	}

	if len(cfg.Repositories) != 4 {
		t.Errorf("Expected 4 repositories, got %d", len(cfg.Repositories))
	}

	if len(cfg.Clean) != 2 {
		t.Errorf("Expected 2 clean, got %d", len(cfg.Clean))
	}
}

func TestPath(t *testing.T) {
	str, err := ioutil.ReadFile("conf.sample")
	if err != nil {
		t.Fatalf("Cannot read config sample from filesystem: %s", err)
	}

	cfg, err := ParseConfig(string(str))
	if err != nil {
		t.Fatalf("Parse error: %s", err)
	}

	urls := []string{
		"example.com/debian",
		"example.com/debian/",
	}

	for _, uri := range urls {
		u, err := url.Parse("http://" + uri)
		if err != nil {
			t.Fatalf("Cannot parse url %s: %s", uri, err)
		}
		expect := "/data/apt-mirror/mirror" + "/" + uri
		actual := cfg.MirrorPath(u)
		if actual != expect {
			t.Errorf("Expected %s, got %s", expect, actual)
		}

		expect = "/data/apt-mirror/skel" + "/" + uri
		actual = cfg.SkelPath(u)
		if actual != expect {
			t.Errorf("Expected %s, got %s", expect, actual)
		}
	}
}

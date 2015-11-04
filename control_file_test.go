package main

import (
	"io/ioutil"
	"testing"
)

func TestParseControlFile(t *testing.T) {
	data, err := ioutil.ReadFile("SiteRelease.sample")
	if err != nil {
		t.Fatalf("Cannot read sample file from filesystem: %s", err)
	}

	c := ParseControlFile(string(data))
	f := func(tag string, expect []string) {
		if _, ok := c[tag]; !ok {
			t.Errorf("Expected tag %#v not found", tag)
			return
		}
		if len(c[tag]) != len(expect) {
			t.Errorf("Expected %d elements with tag %#v, got %d", len(c[tag]), tag, len(expect))
			return
		}

		comp := make(map[string]bool)
		for _, v := range c[tag] {
			comp[v] = true
		}

		for _, v := range expect {
			if !comp[v] {
				t.Errorf("Expected element %#v in tag %#v not found", v, tag)
			}
		}
	}

	f("Origin", []string{"Debian"})
	f("Label", []string{"Debian"})
	f("Suite", []string{"stable"})
	f("Version", []string{"8.2"})
	f("Codename", []string{"jessie"})
	f("Date", []string{"Sat, 05 Sep 2015 09:41:57 UTC"})
	f("Architectures", []string{"amd64 arm64 armel armhf i386 mips mipsel powerpc ppc64el s390x"})
	f("Components", []string{"main contrib non-free"})
	f("Description", []string{"Debian 8.2 Released 05 September 2015"})

	if len(c["MD5Sum"]) != 538 {
		t.Errorf("Expected 538 elements in md5sums, got %d", len(c["MD5Sum"]))
	}
	if len(c["SHA1"]) != 538 {
		t.Errorf("Expected 538 elements in sha1, got %d", len(c["SHA1"]))
	}
}

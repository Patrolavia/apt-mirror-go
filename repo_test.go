package main

import (
	"reflect"
	"strings"
	"testing"
)

func TestParseRepoExtremeCases(t *testing.T) {
	defaultArch := "amd64"
	data := []string{"deb", "http://ftp.tw.debian.org/debian", "stable", "main"}
	strs := []string{
		strings.Join(data, "  "),                  // multiple spaces
		" \t" + strings.Join(data, " \t") + " \t", // trim!
	}

	for _, str := range strs {
		repos, err := ParseRepo(str, defaultArch)
		if err != nil {
			t.Errorf("Parse error when parsing [%s]: %s", str, err)
		}
		if len(repos) != 2 {
			t.Fatalf("%s sould generate 2 record, got %d", str, len(repos))
		}
		repo := repos[0]
		if repo.Architecture != defaultArch {
			t.Errorf("Architecture mismatch in [%s]: %s", str, repo.Architecture)
		}
		if repo.Version != data[2] {
			t.Errorf("Version mismatch in [%s]: %s", str, repo.Version)
		}
		if !reflect.DeepEqual(repo.Components, []string{data[3]}) {
			t.Errorf("Component mismatch in [%s]: %s", str, repo.Components)
		}

		repo = repos[1]
		if repo.Architecture != "all" {
			t.Errorf("Architecture mismatch in [%s]: %s", str, repo.Architecture)
		}
		if repo.Version != data[2] {
			t.Errorf("Version mismatch in [%s]: %s", str, repo.Version)
		}
		if !reflect.DeepEqual(repo.Components, []string{data[3]}) {
			t.Errorf("Component mismatch in [%s]: %s", str, repo.Components)
		}
	}
}

func TestParseRepoNormalCases(t *testing.T) {
	defaultArch := "amd64"
	data := [][]string{
		[]string{"deb", "http://ftp.tw.debian.org/debian", "stable", "main", "contrib", "non-free"},
		[]string{"deb", "ftp://ftp.tw.debian.org/debian", "stable", "main", "contrib", "non-free"},
		[]string{"deb", "http://ftp.tw.debian.org/debian", "stable", "main"},
		[]string{"i386", "http://ftp.tw.debian.org/debian", "stable", "main"},
	}

	for _, data := range data {
		str := strings.Join(data, " ")
		arch := data[0]
		if data[0] == "deb" {
			arch = defaultArch
		} else {
			str = "deb-" + str
		}

		repos, err := ParseRepo(str, defaultArch)
		if err != nil {
			t.Errorf("Parse error when parsing %s: %s", str, err)
		}

		if len(repos) != 2 {
			t.Fatalf("%s sould generate %d records, got %d", str, 2, len(repos))
		}

		for idx, repo := range repos {
			a := arch
			if idx%2 == 1 {
				a = "all"
			}
			if repo.Architecture != a {
				t.Errorf("Architecture mismatch: need %s, got %s", a, repo.Architecture)
			}
			if repo.Version != data[2] {
				t.Errorf("Version mismatch: need %s, got %s", data[2], repo.Version)
			}
			if !reflect.DeepEqual(repo.Components, data[3:]) {
				t.Errorf("Component mismatch: need %s, got %s", data[3:], repo.Components)
			}
		}
	}
}

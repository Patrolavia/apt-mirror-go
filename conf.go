package main

import (
	"log"
	"net/url"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)

var varRegexp *regexp.Regexp
var defaultArch []byte

func init() {
	var err error
	if varRegexp, err = regexp.Compile(`^set [a-zA-Z_]([a-zA-Z0-9_]+) `); err != nil {
		log.Fatalf("Cannot compile regexp for parsing variables in config file: %s", err)
	}
	defaultArch, err = exec.Command("dpkg", "--print-architecture").Output()
	if err != nil {
		log.Printf("Cannot run `dpkg --print-architecture` to fetch current architecture, use i386")
		defaultArch = []byte("i386")
	}
}

type Config struct {
	Variables    map[string]string
	Repositories []Repository
	Clean        map[string]bool
}

func ParseConfig(cfgString string) (ret *Config, err error) {
	ret = &Config{
		map[string]string{
			"defaultarch":       string(defaultArch),
			"base_path":         "/var/spool/apt-mirror",
			"mirror_path":       "/var/spool/apt-mirror/mirror",
			"skel_path":         "/var/spool/apt-mirror/skel",
			"postmirror_script": "",
			"run_postmirror":    "0",
			"nthreads":          "20",
			"translations":      "en",
		},
		make([]Repository, 0),
		make(map[string]bool),
	}
	arr := strings.Split(cfgString, "\n")
	for _, line := range arr {
		line = strings.TrimSpace(line)
		if line == "" || line[0:1] == "#" {
			// comment or empty line, skip
			continue
		}

		if match := varRegexp.Find([]byte(line)); match != nil {
			// setting variable
			varName := string(match[4 : len(match)-1])
			val := strings.TrimSpace(string(line[len(match):]))

			// replace the variables in value
			for k, v := range ret.Variables {
				val = strings.Replace(val, "$"+k, v, -1)
			}

			ret.Variables[varName] = val
			continue
		}

		if line[0:3] == "deb" {
			// repository specification
			repos, err := ParseRepo(line, ret.Variables["defaultarch"])
			if err != nil {
				return ret, err
			}

			for _, repo := range repos {
				exist := false
				for _, old := range ret.Repositories {
					if old.Equals(repo) {
						exist = true
						break
					}
				}
				if exist {
					continue
				}
				ret.Repositories = append(ret.Repositories, repo)
			}
		}

		if line[0:6] == "clean " {
			val := strings.TrimSpace(line[6:])
			ret.Clean[val] = true
		}
	}
	return
}

func (c Config) SkelPath(u *url.URL) string {
	return strings.Join([]string{
		c.Variables["skel_path"],
		u.Host + u.Path,
	}, "/")
}

func (c Config) MirrorPath(u *url.URL) string {
	return strings.Join([]string{
		c.Variables["mirror_path"],
		u.Host + u.Path,
	}, "/")
}

func (c Config) GetInt(tag string) int {
	s := c.Variables[tag]
	ret, err := strconv.Atoi(s)
	if err != nil {
		ret = 0
	}
	return ret
}

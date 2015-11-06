package main

import (
	"log"
	"net/url"
	"regexp"
	"strings"
)

var ctrlRegexp *regexp.Regexp

func init() {
	var err error
	if ctrlRegexp, err = regexp.Compile(`^[a-zA-Z0-9_-]+:`); err != nil {
		log.Fatalf("Cannot compile regexp for parsing control file: %s", err)
	}
}

// ParseControlFile reads debian control file format into map[string][]string
func ParseControlFile(data string) url.Values {
	ret := url.Values(make(map[string][]string))
	arr := strings.Split(data, "\n")
	cur := ""

	for lineno, line := range arr {
		if line == "" {
			// skip empty line
			continue
		}
		lineno++
		tag := ctrlRegexp.Find([]byte(line))
		if tag == nil {
			if _, ok := ret[cur]; !ok {
				log.Printf("Ignoring format error at line#%d: %#v", lineno, line)
				continue
			}
			ret[cur] = append(ret[cur], line)
			continue
		}
		cur = string(tag[0 : len(tag)-1])
		ret[cur] = make([]string, 0)
		if len(line) > len(tag)+1 {
			ret.Add(cur, line[len(tag)+1:])
		}
	}
	return ret
}

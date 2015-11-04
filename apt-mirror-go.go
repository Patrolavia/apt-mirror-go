package main

import (
	"flag"
	"io/ioutil"
	"log"
	"net/url"
	"os"
	"os/exec"
	"path"
)

var (
	dryRun bool
	cfgFile string
)

func init() {
	flag.BoolVar(&dryRun, "n", false, "Log message only, not to download package files")
	flag.Parse()
	cfgFile = flag.Arg(0)
	if cfgFile == "" {
		cfgFile = "/etc/apt/mirror.list"
	}
}

func main() {
	log.Printf("Reading config file from %s", cfgFile)
	cfgStr, err := ioutil.ReadFile(cfgFile)
	if err != nil {
		log.Fatalf("Cannot read config from %s: %s", cfgFile, err)
	}

	cfg, err := ParseConfig(string(cfgStr))
	if err != nil {
		log.Fatalf("Error parsing config files: %s", err)
	}

	nthreads := cfg.GetInt("nthreads")
	if nthreads < 1 {
		nthreads = 1
	}

	log.Printf("Path holding temp files(skel_path): %s", cfg.Variables["skel_path"])
	log.Printf("Path holding mirrored files(mirror_path): %s", cfg.Variables["mirror_path"])
	log.Printf("Will spawn %d goroutines to download packages.", nthreads)
	log.Printf("Default architecture: %s", cfg.Variables["defaultarch"])

	// download info files, process packages file and generate file list
	debs := make(map[string]Package)
	for _, repo := range cfg.Repositories {
		if repo.Architecture == "src" {
			// TODO: support source packages
			continue
		}
		repo.DownloadInfoFiles(cfg)

		for _, comp := range repo.Components {
			pkgFile := cfg.SkelPath(repo.Packages(comp))
			pkgStr, err := ioutil.ReadFile(pkgFile)
			if err != nil {
				// maybe file not found, use gzipped file instead
				pkgFile = cfg.SkelPath(repo.PackagesGZ(comp))
				if _, err := os.Stat(pkgFile); err != nil {
					log.Fatalf("Cannot find Packages file: %s", err)
				}
				pkgStr, err = exec.Command("gzip", "-cdfq", pkgFile).Output()
				if err != nil {
					log.Fatalf("Cannot open Packages file: %s", err)
				}
			}
			pkgs, err := ParsePackage(repo, string(pkgStr))
			if err != nil {
				log.Fatalf("Cannot parse Packages file %s: %s", pkgFile, err)
			}

			for _, p := range pkgs {
				m := cfg.MirrorPath(p.URL)
				debs[m] = p
			}
		}
	}
	log.Printf("Got %d package files ... ", len(debs))

	ch := make(chan Package)
	finish := make(chan int)

	for i := 0; i < nthreads; i++ {
		go worker(i, cfg, ch, finish)
	}

	for _, pkg := range debs {
		if pkg.Test(cfg) {
			// file exists, skip
			continue
		}
		// debug: print out why
		fn := cfg.SkelPath(pkg.URL)
		if stat, err := os.Stat(fn); err == nil {
			log.Printf("%s size is %d, not %d", fn, stat.Size(), pkg.Size)
		} else {
			log.Printf("%s not found", fn)
		}
		ch <- pkg
	}
	close(ch)

	// wait for download finish
	for i := 0; i < nthreads; i++ {
		<-finish
	}

	for c := range cfg.Clean {
		log.Printf("Cleaning %s", c)
		clean(c, cfg, debs)
	}

	if !dryRun {
		movefiles(cfg.Variables["skel_path"], cfg.Variables["mirror_path"])
	}

}

func worker(id int, cfg *Config, ch chan Package, finish chan int) {
	for p := range ch {
		log.Printf("Worker#%d downloading %s", id, p.URL)
		if !dryRun {
			if err := p.Download(cfg); err != nil {
				log.Fatalf("Error downloading %s: %s", p.URL, err)
			}
		}
	}
	finish <- 1
}

func clean(urlStr string, cfg *Config, debs map[string]Package) {
	u, err := url.Parse(urlStr)
	if err != nil {
		log.Fatalf("%s is not a valid url: %s", urlStr, err)
	}

	mirrorPath := cfg.MirrorPath(u)
	doClean(mirrorPath, debs)
}

func doClean(dir string, debs map[string]Package) {
	log.Printf("Cleaning %s", dir)
	base, err := os.Open(dir)
	if err != nil {
		return
	}
	defer base.Close()

	abs := func(path string) string {
		return dir + "/" + path
	}

	children, err := base.Readdir(-1)
	if err != nil {
		return
	}

	for _, child := range children {
		if child.IsDir() {
			doClean(path.Join(dir, child.Name()), debs)
			continue
		}

		p := abs(child.Name())
		if _, ok := debs[p]; !ok {
			log.Printf("Remove out-dated file %s", p)
			if !dryRun {
				os.Remove(p)
			}
		}
	}
}

func movefiles(src, dst string) {
	f, err := os.Open(src)
	if err != nil {
		log.Fatalf("Cannot open dir %s for read: %s", src, err)
	}
	defer f.Close()

	dirs, err := f.Readdirnames(-1)
	if err != nil {
		log.Fatalf("Cannot read contents of dir %s for read: %s", src, err)
	}
	
	for _, dir := range dirs {
		fn := path.Join(src, dir)
		if err := exec.Command("mv", "-f", fn, dst).Run(); err != nil {
			log.Fatalf("Cannot move %s to %s: %s", fn, dst, err)
		}
	}
}
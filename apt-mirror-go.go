// See https://github.com/Patrolavia/apt-mirror-go for detail.
package main

import (
	"compress/gzip"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path"

	"github.com/juju/ratelimit"
)

var (
	dryRun  bool
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

	var bucket *ratelimit.Bucket = nil
	rate := cfg.GetInt("ratelimit")
	if rate > 0 {
		bucket = ratelimit.NewBucketWithRate(float64(rate*1024), int64(rate*1024))
	}

	dlMgr := NewManager(
		func(u *url.URL) string {
			return fmt.Sprintf("URL scheme %s of %s is not supported", u.Scheme, u)
		},
		bucket,
		http.DefaultClient,
		nthreads,
	)

	log.Printf("Path holding temp files(skel_path): %s", cfg.Variables["skel_path"])
	log.Printf("Path holding mirrored files(mirror_path): %s", cfg.Variables["mirror_path"])
	log.Printf("Default architecture: %s", cfg.Variables["defaultarch"])
	log.Printf("Spawning %d goroutines to download packages.", nthreads)

	ch := make(chan Package)
	finish := make(chan int)

	for i := 0; i < nthreads; i++ {
		go worker(i, cfg, dlMgr, ch, finish)
	}

	// download info files, process packages file and generate file list
	debs := make(map[string]bool)
	for _, repo := range cfg.Repositories {
		repo.DownloadInfoFiles(cfg, dlMgr)

		for _, comp := range repo.Components {
			pkgFile := cfg.SkelPath(repo.Packages(comp))
			var f io.ReadCloser
			f, err := os.Open(pkgFile)
			if err != nil {
				// maybe file not found, use gzipped file instead
				pkgFile = cfg.SkelPath(repo.PackagesGZ(comp))
				f, err = os.Open(pkgFile)
				if err == nil {
					f, err = gzip.NewReader(f)
				}
			}
			if err != nil {
				log.Fatalf("Cannot open package file %s: %s", pkgFile, err)
			}
			pkgs, err := ParsePackage(repo, f)
			if err != nil {
				log.Fatalf("Cannot parse Packages file %s: %s", pkgFile, err)
			}
			f.Close()

			for _, p := range pkgs {
				m := cfg.MirrorPath(p.URL)
				debs[m] = true
				ch <- p
			}
		}
	}
	close(ch)
	log.Printf("Got %d package files ... ", len(debs))

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

func worker(id int, cfg *Config, dlMgr *DownloadManager, ch chan Package, finish chan int) {
	for p := range ch {
		if p.Test(cfg) {
			continue
		}

		debugMsg := ""
		// ========== debug log
		debugMsg = " :"
		m := cfg.MirrorPath(p.URL)
		s := cfg.SkelPath(p.URL)
		statM, errM := os.Stat(m)
		statS, errS := os.Stat(s)
		switch {
		case errM == nil:
			debugMsg += fmt.Sprintf("Size %d not %d", statM.Size(), p.Size)
		case errS == nil:
			debugMsg += fmt.Sprintf("Size %d not %d", statS.Size(), p.Size)
		default:
			debugMsg += "File not found"
		}
		// ========== end of debug

		log.Printf("Worker#%d downloading %s%s", id, p.URL, debugMsg)
		if !dryRun {
			// max retry 3 times
			maxRetry := 3
			var err error
			for i := 0; i < maxRetry; i++ {
				if err = p.Download(cfg, dlMgr.Dispatch(p.URL)); err == nil {
					break
				}
			}
			if err != nil {
				log.Fatalf("Error downloading %s: %s", p.URL, err)
			}
		}
	}
	finish <- 1
}

func clean(urlStr string, cfg *Config, debs map[string]bool) {
	u, err := url.Parse(urlStr)
	if err != nil {
		log.Fatalf("%s is not a valid url: %s", urlStr, err)
	}

	mirrorPath := cfg.MirrorPath(u)
	doClean(mirrorPath, debs)
}

func doClean(dir string, debs map[string]bool) {
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
			dirn := path.Join(dir, child.Name())
			doClean(dirn, debs)
			if c, err := os.Open(dirn); err == nil {
				remove := false
				if children, err := c.Readdirnames(-1); err == nil && len(children) == 0 {
					remove = true
				}

				c.Close()
				if remove {
					log.Printf("Remove empty directory %s", dirn)
					if !dryRun {
						os.Remove(dirn)
					}
				}
			}

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
		if err := exec.Command("cp", "-r", fn, dst).Run(); err != nil {
			log.Fatalf("Cannot move %s to %s: %s", fn, dst, err)
		}
		exec.Command("rm", "-fr", fn).Run()
	}
}

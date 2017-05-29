// Binary reaper cleans files that have not been accessed past a provided
// expiry. It works by reading the file's access time ("atime") from its
// filesystem descriptor.
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/matttproud/reaper"

	extraflag "github.com/matttproud/reaper/flag"
)

func demo(e *reaper.Entry) {
	if e.IsDir() {
		fmt.Printf("rmdir %q\n", e.Path)
		return
	}
	fmt.Printf("rm %q\n", e.Path)
}

func remove(e *reaper.Entry) error { return os.Remove(e.Path) }

func visit(path string, dryRun, showCmd bool, opts ...reaper.Option) {
	r, err := reaper.New(path, opts...)
	if err != nil {
		log.Printf("walk on %q failed: %v", path, err)
		return
	}
	defer r.Stop()
	for r.Scan() {
		fi := r.File()
		if showCmd {
			demo(fi)
		}
		if !dryRun {
			if err := remove(fi); err != nil {
				log.Printf("deletion of %q failed: %v", fi.Path, err)
			}
		}
	}
	if err := r.Err(); err != nil {
		log.Printf("walk on %q failed: %v", path, err)
	}
}

func main() {
	var (
		expiry    time.Duration
		protect   []string
		dryRun    bool
		showCmd   bool
		irregular bool
		force     bool
	)
	flag.DurationVar(&expiry, "expiry", 0, "files are considered expired if older than this duration")
	extraflag.StringSliceVar(&protect, "protect", fmt.Sprintf("%q separated globs for data to protect", os.PathListSeparator))
	flag.BoolVar(&dryRun, "dry-run", true, "whether to actually perform the operation")
	flag.BoolVar(&showCmd, "show-command", true, "show what command would be run")
	flag.BoolVar(&irregular, "irregular", false, "whether to consider irregular files types, like fifo, devices, etc.")
	flag.BoolVar(&force, "force", false, "whether to ignore candidate permissions")
	flag.Parse()
	if expiry <= 0 {
		fmt.Fprintln(os.Stderr, "expiry must be a duration > 0")
		flag.Usage()
		os.Exit(1)
	}
	opts := []reaper.Option{reaper.ExpungeAfter(expiry), reaper.Protect(protect...)}
	if irregular {
		opts = append(opts, reaper.ExpungeIrregular())
	}
	if force {
		opts = append(opts, reaper.Force())
	}
	for _, p := range flag.Args() {
		visit(p, dryRun, showCmd, opts...)
	}
}

// Package reaper provides the business logic for safely identifying stale
// files for deletion.
package reaper

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"time"

	"github.com/djherbis/atime"
)

// Entry models a file eligible for deletion.
type Entry struct {
	os.FileInfo
	// Path is the file's full-path relative to the search root.
	Path string
}

func uid(fi os.FileInfo) int { return int(fi.Sys().(*syscall.Stat_t).Uid) }
func gid(fi os.FileInfo) int { return int(fi.Sys().(*syscall.Stat_t).Gid) }

func onSameDevice(a, b os.FileInfo) bool {
	aSys, bSys := a.Sys().(*syscall.Stat_t), b.Sys().(*syscall.Stat_t)
	return aSys.Dev == bSys.Dev
}
func isSymlink(a os.FileInfo) bool { return a.Mode()&os.ModeSymlink != 0 }

func isEmpty(path string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()
	fi, err := f.Readdir(1)
	return len(fi) == 0
}

// Option defines a runtime parameter for the reaper.
type Option func(*Reaper)

// ExpungeAfter identifies files for deletion whose access time surpasses d.
func ExpungeAfter(d time.Duration) Option { return func(r *Reaper) { r.expungeTTL = d } }

// ExpungeIrregular allows irregular files to be deleted --- i.e., non-regular
// files and directories.
func ExpungeIrregular() Option { return func(r *Reaper) { r.expungeIrregular = true } }

// Protect applies a blacklist pattern the Reaper, meaning no files or
// directories that match it will be visited nor marked as eligible for
// deletion.
func Protect(globs ...string) Option {
	return func(r *Reaper) { r.protect = append(r.protect, globs...) }
}

// Force instructs the reaper to ignore permissions and report candidates
// eligible by age.
func Force() Option { return func(r *Reaper) { r.force = true } }

// New creates a new Reaper for scanning the root. The root directory must exist.
func New(root string, opts ...Option) (*Reaper, error) {
	info, err := os.Lstat(root)
	if err != nil {
		return nil, err
	}
	grps, err := os.Getgroups()
	if err != nil {
		return nil, err
	}
	groups := make(map[int]bool, len(grps))
	for _, id := range grps {
		groups[id] = true
	}
	r := Reaper{
		root:    info,
		euid:    os.Geteuid(),
		egid:    os.Getegid(),
		stop:    make(chan struct{}),
		results: make(chan *Entry),
		groups:  groups,
	}
	for _, o := range opts {
		o(&r)
	}
	go func() {
		defer close(r.results)
		if err := filepath.Walk(root, r.walk); err != nil && err != errStop {
			r.err = err
		}
	}()
	return &r, nil
}

type Reaper struct {
	root             os.FileInfo
	euid             int
	egid             int
	expungeTTL       time.Duration
	expungeIrregular bool
	protect          []string
	groups           map[int]bool
	force            bool

	stop    chan struct{}
	results chan *Entry

	cur *Entry

	err error
}

func (r *Reaper) Err() error { return r.err }

func (r *Reaper) Stop() { close(r.stop) }

func (r *Reaper) Scan() bool {
	select {
	case res, ok := <-r.results:
		r.cur = res
		if ok {
			return true
		}
	case <-r.stop:
	}
	return false
}

func (r *Reaper) File() *Entry { return r.cur }

var errStop = errors.New("reaper: stop")

func isProtected(path string, globs []string) bool {
	for _, glob := range globs {
		if match, err := filepath.Match(glob, path); match {
			return true
		} else if err != nil {
			panic(fmt.Sprintf("could not determine protection %q: %v", glob, err))
		}
	}
	return false
}

func (r *Reaper) isProtected(path string) bool { return isProtected(path, r.protect) }

func isWritable(f os.FileInfo, euid, egid int, groups map[int]bool, perm os.FileMode) bool {
	// XXX: Report sticky parent directories?
	if euid == 0 {
		return true
	}
	const S_IWOTH = 0002
	if perm&S_IWOTH != 0 {
		return true
	}
	const S_IWUSR = 0200
	if uid := uid(f); (perm&S_IWUSR != 0) && uid == euid {
		return true
	}
	const S_IWGRP = 0020
	if gid := gid(f); perm&S_IWGRP != 0 {
		if gid == egid {
			return true
		}
		if groups[gid] {
			return true
		}
	}
	return false
}

func (r *Reaper) isWritable(f os.FileInfo) bool {
	return isWritable(f, r.euid, r.egid, r.groups, f.Mode().Perm())
}

func (r *Reaper) onSameDevice(info os.FileInfo) bool { return onSameDevice(r.root, info) }

func (r *Reaper) accept(fi os.FileInfo, path string) error {
	select {
	case r.results <- &Entry{fi, path}:
		return nil
	case <-r.stop:
		return errStop
	}
}

func hasExpired(f os.FileInfo, expiry time.Duration, since func(time.Time) time.Duration) bool {
	if since == nil {
		since = time.Since
	}
	return since(atime.Get(f)) > expiry
}

func (r *Reaper) visitDir(path string, info os.FileInfo, err error) error {
	if r.isProtected(path) {
		return filepath.SkipDir
	}
	if !r.onSameDevice(info) {
		return filepath.SkipDir
	}
	if !(r.isWritable(info) || r.force) {
		return nil
	}
	if isEmpty(path) && hasExpired(info, r.expungeTTL, nil) {
		return r.accept(info, path)
	}
	return nil
}

func (r *Reaper) visitFile(path string, info os.FileInfo, err error) error {
	if r.isProtected(path) {
		return nil
	}
	if !r.onSameDevice(info) {
		return nil
	}
	if !(info.Mode().IsRegular() || r.expungeIrregular) {
		return nil
	}
	if !(r.isWritable(info) || r.force) {
		return nil
	}
	if hasExpired(info, r.expungeTTL, nil) {
		return r.accept(info, path)
	}
	return nil
}

func (r *Reaper) walk(path string, info os.FileInfo, err error) error {
	select {
	case <-r.stop:
		return errStop
	default:
		if info.IsDir() {
			return r.visitDir(path, info, err)
		}
		return r.visitFile(path, info, err)
	}
	return nil
}

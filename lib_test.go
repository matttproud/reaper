package reaper

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

type fakeSys struct {
	os.FileInfo
	stat syscall.Stat_t
}

func (fi fakeSys) Sys() interface{} { return &fi.stat }

func TestOnSameDevice(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		lhs, rhs fakeSys
		want     bool
	}{
		{
			want: true,
		},
		{
			lhs:  fakeSys{stat: syscall.Stat_t{Dev: 6}},
			rhs:  fakeSys{stat: syscall.Stat_t{Dev: 6}},
			want: true,
		},
		{
			lhs:  fakeSys{stat: syscall.Stat_t{Dev: 5}},
			rhs:  fakeSys{stat: syscall.Stat_t{Dev: 6}},
			want: false,
		},
	} {
		if got, want := onSameDevice(test.lhs, test.rhs), test.want; got != want {
			t.Errorf("onSameDevice(%v, %v) = %v, want %v", test.lhs, test.rhs, got, want)
		}
	}
}

type fakeMode struct {
	os.FileInfo
	mode os.FileMode
}

func (fi fakeMode) Mode() os.FileMode { return fi.mode }

func TestIsSymlink(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		mode fakeMode
		want bool
	}{
		{
			want: false,
		},
		{
			mode: fakeMode{mode: ^os.ModeType},
			want: false,
		},
		{
			mode: fakeMode{mode: os.ModeSymlink},
			want: true,
		},
	} {
		if got, want := isSymlink(test.mode), test.want; got != want {
			t.Errorf("isSymlink(%v) = %v, want %v", test.mode, got, want)
		}
	}

}

func TestIsEmpty(t *testing.T) {
	t.Parallel()
	dir, err := ioutil.TempDir("", "TestIsEmpty")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := os.RemoveAll(dir); err != nil {
			t.Fatal(err)
		}
	}()
	if !isEmpty(dir) {
		t.Errorf("isEmpty(%q) = false, want = true", dir)
	}
	if err := ioutil.WriteFile(filepath.Join(dir, "dummy"), []byte{0, 1, 2, 3, 4}, 0644); err != nil {
		t.Fatal(err)
	}
	if isEmpty(dir) {
		t.Errorf("isEmpty(%q) = true, want = false", dir)
	}
}

func TestIsProtected(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		path  string
		globs []string
		want  bool
	}{
		{
			path: "path/to/foo",
			want: false,
		},
		{
			path:  "path/to/foo",
			globs: []string{"path/to/*"},
			want:  true,
		},
		{
			path:  "path/to/foo",
			globs: []string{"bar", "path/to/*"},
			want:  true,
		},
		{
			path:  "path/to/junk",
			globs: []string{"bar", "foo"},
			want:  false,
		},
	} {
		if got, want := isProtected(test.path, test.globs), test.want; got != want {
			t.Errorf("isProtected(%q, %v) = %v, want %v", test.path, test.globs, got, want)
		}
	}
}

func TestIsWritable(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		info       fakeSys
		euid, egid int
		groups     map[int]bool
		perm       os.FileMode
		want       bool
	}{
		{
			euid: 0,
			want: true,
		},
		{
			euid: 1,
			perm: 0007,
			want: true,
		},
		{
			info: fakeSys{stat: syscall.Stat_t{Uid: 1}},
			euid: 1,
			perm: 0700,
			want: true,
		},
		{
			info: fakeSys{stat: syscall.Stat_t{Uid: 1, Gid: 2}},
			euid: 1,
			egid: 2,
			perm: 0070,
			want: true,
		},
		{
			info:   fakeSys{stat: syscall.Stat_t{Uid: 1, Gid: 2}},
			euid:   1,
			egid:   1,
			groups: map[int]bool{2: true},
			perm:   0070,
			want:   true,
		},
		{
			info:   fakeSys{stat: syscall.Stat_t{Uid: 1, Gid: 3}},
			euid:   1,
			egid:   1,
			groups: map[int]bool{2: true},
			perm:   0070,
			want:   false,
		},
	} {
		if got, want := isWritable(test.info, test.euid, test.egid, test.groups, test.perm), test.want; got != want {
			t.Errorf("isWritable(%v, %v, %v, %v, %v) = %v, want %v", test.info, test.euid, test.egid, test.groups, test.perm, got, want)
		}
	}
}

func toTspec(t time.Time) syscall.Timespec { return syscall.NsecToTimespec(t.UnixNano()) }

func TestIntegration(t *testing.T) {
	t.Parallel()
	dir, err := ioutil.TempDir("", "TestIsEmpty")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := os.RemoveAll(dir); err != nil {
			t.Fatal(err)
		}
	}()
	if err := os.Mkdir(filepath.Join(dir, "old_empty"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(dir, "old_with_young_content"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := ioutil.WriteFile(filepath.Join(dir, "old_with_young_content", "old_content"), []byte{0, 1, 2, 3, 4}, 0644); err != nil {
		t.Fatal(err)
	}
	time.Sleep(5 * time.Second)
	if err := ioutil.WriteFile(filepath.Join(dir, "old_with_young_content", "new_content"), []byte{0, 1, 2, 3, 4}, 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(dir, "new_empty"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(dir, "new_with_young_content"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := ioutil.WriteFile(filepath.Join(dir, "new_with_young_content", "new_content"), []byte{0, 1, 2, 3, 4}, 0644); err != nil {
		t.Fatal(err)
	}
	r, err := New(dir, ExpungeAfter(5*time.Second))
	if err != nil {
		t.Fatal(err)
	}
	defer r.Stop()
	{
		if !r.Scan() {
			t.Fatal("expected first iteration to read something")
		}
		info := r.File()
		if !strings.HasSuffix(info.Path, "/old_empty") {
			t.Errorf("expected /old_empty, got %q", info.Path)
		}
	}
	{
		if !r.Scan() {
			t.Fatal("expected first iteration to read something")
		}
		info := r.File()
		if !strings.HasSuffix(info.Path, "/old_with_young_content/old_content") {
			t.Errorf("expected /old_with_young_content/old_content, got %q", info.Path)
		}
	}
	{
		if r.Scan() {
			t.Fatal("expected scan to say we're done")
		}
		if err := r.Err(); err != nil {
			t.Fatal(err)
		}
	}
}

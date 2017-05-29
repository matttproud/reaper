package reaper

import (
	"syscall"
	"testing"
	"time"
)

// +build linux

func TestHasExpired(t *testing.T) {
	now := time.Date(2017, 5, 28, 17, 13, 0, 0, time.UTC)
	since := func(t time.Time) time.Duration { return now.Sub(t) }
	t.Parallel()
	for _, test := range []struct {
		info   fakeSys
		expiry time.Duration
		want   bool
	}{
		{
			info:   fakeSys{stat: syscall.Stat_t{Atim: toTspec(now.Add(-30 * time.Second))}},
			want:   false,
			expiry: time.Minute,
		},
		{
			info:   fakeSys{stat: syscall.Stat_t{Atim: toTspec(now.Add(-90 * time.Second))}},
			want:   true,
			expiry: time.Minute,
		},
	} {
		if got, want := hasExpired(test.info, test.expiry, since), test.want; got != want {
			t.Errorf("hasExpired(%v, %v, time.Since) = %v, want %v", test.info, test.expiry, got, want)
		}
	}
}

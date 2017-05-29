# Reaper
A tool written exclusively in [Go](http://www.golang.org) to rid a file system
of stale files that may no longer be needed.  It is inspired by `tmpreaper` and
fits the use case of wanting a tool but not wanting a massive build toolchain to
be present to use it — e.g., Xcode on Mac OS for Homebrew.

# Installation
Initial compilation and installation require the Go toolchain.  It is an easy
and fast download.  Thereafter the binary may be distributed as a static linked
archive.

```
go get github.com/matttproud/reaper/cmd/reaper
cp ${GOPATH}/src/github.com/matttproud/reaper/cmd/reaper/reaper.1 \
    -t ${HOME}/.local/share/man/man1
```

# Useful Features and Characteristics

* Portable implementation

* 1/4 the lines of code of `tmpreaper`

* The core has unit and integration tests

* It is easy to extend; the core exists as a regular Go package (library) for
  other tools to build upon

# License
As described in LICENSE, it is Apache License version 2.

# Warranty
See WARRANTY.

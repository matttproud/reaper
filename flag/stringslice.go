// Package flag provides extra flag.Value implementations.
package flag

import (
	"flag"
	"os"
	"strings"
)

const sep = string(os.PathListSeparator)

type strSlice []string

func (f *strSlice) String() string { return strings.Join([]string(*f), sep) }

func (f *strSlice) Set(s string) error {
	*f = strings.Split(s, sep)
	return nil
}

func (f *strSlice) Get() interface{} { return []string(*f) }

// StringSliceVar declares a new flag of os.PathListSeparator delimited string
// values.
func StringSliceVar(s *[]string, name, usage string) {
	fl := (*strSlice)(s)
	flag.Var(fl, name, usage)
}

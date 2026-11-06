package x

import "regexp"

func f() { _ = "//nolint:perf"; regexp.MustCompile("a") }

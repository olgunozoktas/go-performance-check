package x

import "regexp"

var re = regexp.MustCompile(`[^a-z]`)

func clean(s string) string { return re.ReplaceAllString(s, "") }

func user(pattern string) { regexp.MustCompile(pattern) }

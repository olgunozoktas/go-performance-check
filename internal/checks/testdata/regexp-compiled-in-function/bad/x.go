package x

import "regexp"

func clean(s string) string { return regexp.MustCompile(`[^a-z]`).ReplaceAllString(s, "") }

func each(items []string) {
	for _, s := range items {
		regexp.MustCompile(`^a`).MatchString(s)
	}
}

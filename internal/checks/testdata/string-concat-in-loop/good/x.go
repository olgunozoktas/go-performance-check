package x

import "strings"

func join(items []string) string {
	var b strings.Builder
	n := 0
	for _, it := range items {
		b.WriteString(it)
		n += len(it)
	}
	return b.String()
}

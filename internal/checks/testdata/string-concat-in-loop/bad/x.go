package x

func join(items []string) string {
	s := ""
	for _, it := range items {
		s += it + ","
	}
	return s
}

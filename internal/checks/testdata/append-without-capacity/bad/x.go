package x

func double(in []int) []int {
	out := []int{}
	for _, v := range in {
		out = append(out, v*2)
	}
	return out
}

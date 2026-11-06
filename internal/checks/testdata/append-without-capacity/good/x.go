package x

func evens(in []int) []int {
	var out []int
	for _, v := range in {
		if v%2 == 0 {
			out = append(out, v)
		}
	}
	return out
}

func double(in []int) []int {
	out := make([]int, 0, len(in))
	for _, v := range in {
		out = append(out, v*2)
	}
	return out
}

package x

import "os"

func each(paths []string) {
	for _, p := range paths {
		func() {
			f, _ := os.Open(p)
			defer f.Close()
		}()
	}
}

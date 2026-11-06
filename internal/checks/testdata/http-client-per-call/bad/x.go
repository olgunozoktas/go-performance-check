package x

import "net/http"

func fetch(url string) {
	client := &http.Client{}
	client.Get(url)
	http.Get(url)
}

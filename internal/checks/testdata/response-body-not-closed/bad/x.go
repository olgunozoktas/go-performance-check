package x

import "net/http"

func fetch(client *http.Client, req *http.Request) int {
	resp, err := client.Do(req)
	if err != nil {
		return 0
	}
	return resp.StatusCode
}

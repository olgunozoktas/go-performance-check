package x

import "net/http"

type store struct{}

func (store) Get(id string) (int, error) { return 0, nil }

func fetch(client *http.Client, req *http.Request, s store) (int, error) {
	resp, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	v, err := s.Get("a")
	return resp.StatusCode + v, err
}

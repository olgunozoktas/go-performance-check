package x

import (
	"net/http"
	"time"
)

type Service struct{ client *http.Client }

func NewService() *Service { return &Service{client: &http.Client{Timeout: 10 * time.Second}} }

func (s *Service) build() { s.client = &http.Client{Timeout: time.Second} }

func newClient() *http.Client {
	c := &http.Client{Timeout: time.Second}
	return c
}

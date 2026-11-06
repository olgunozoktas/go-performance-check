package x

import "sync"

type Counter struct {
	mu sync.Mutex
	n  int
}

func (c Counter) Inc() { c.mu.Lock(); c.n++; c.mu.Unlock() }

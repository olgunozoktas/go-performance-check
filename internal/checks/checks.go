// Package checks holds one file for each check; each registers itself with core.Register in init.
package checks

import "github.com/olgunozoktas/go-performance-check/internal/core"

// fn is a check made of a name and a function.
type fn struct {
	name string
	run  func(m *core.Module) []core.Finding
}

func (c fn) Name() string                      { return c.name }
func (c fn) Run(m *core.Module) []core.Finding { return c.run(m) }
func register(name string, run func(m *core.Module) []core.Finding) {
	core.Register(fn{name, run})
}

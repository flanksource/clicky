package entity

import (
	"fmt"
	"sync"
)

var (
	filterRegistry   = map[string]NamedFilter{}
	filterRegistryMu sync.RWMutex
)

// RegisterFilter registers a named filter for reuse across entities. It panics
// on a duplicate name or an incomplete definition — registration happens at
// init/setup time, so a collision is a programming error that must fail loudly.
func RegisterFilter(f NamedFilter) {
	if f.Name == "" {
		panic("entity.RegisterFilter: filter name must not be empty")
	}
	if f.Source == nil {
		panic(fmt.Sprintf("entity.RegisterFilter: filter %q has no Source", f.Name))
	}

	filterRegistryMu.Lock()
	defer filterRegistryMu.Unlock()
	if _, exists := filterRegistry[f.Name]; exists {
		panic(fmt.Sprintf("entity.RegisterFilter: filter %q already registered", f.Name))
	}
	filterRegistry[f.Name] = f
}

// GetFilter returns the named filter and whether it was found.
func GetFilter(name string) (NamedFilter, bool) {
	filterRegistryMu.RLock()
	defer filterRegistryMu.RUnlock()
	f, ok := filterRegistry[name]
	return f, ok
}

// MustGetFilter returns the named filter, panicking if it is not registered.
// Used at attach time (Use/.As), where an unknown reference is a wiring bug.
func MustGetFilter(name string) NamedFilter {
	f, ok := GetFilter(name)
	if !ok {
		panic(fmt.Sprintf("entity: filter %q is not registered", name))
	}
	return f
}

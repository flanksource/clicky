package entity

import "fmt"

// ParseArgs parses HTTPie-style positional arguments (key=value, key:=json, …)
// into a body map for create/update commands. Root clicky wires it in init() to
// its ParseArgumentsAsMap; the default fails loudly so a missing wiring surfaces
// rather than silently dropping arguments.
var ParseArgs = func(args []string) (map[string]any, error) {
	return nil, fmt.Errorf("entity.ParseArgs not configured: the root clicky package must wire it")
}

// RenderResult renders a command result through the host application's format
// pipeline — the global --format/--output flags and any configured sinks. Root
// clicky wires it in init() (to ParseFormatSpec + PrintAndWriteSinks against its
// global Flags); the default no-op lets the entity package's own unit tests run
// without a renderer.
//
// It is a package-level hook rather than a direct dependency so the entity
// package stays free of the root clicky package (which owns the global flag
// state) — preserving the one-way entity → clicky-subpackages dependency.
var RenderResult = func(o any) error { return nil }

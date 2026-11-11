package telltale

import (
	"context"
	"encoding/json"
	"fmt"
	"runtime"
	"runtime/debug"

	sdk "github.com/flanksource/telltale/sdk"
	"github.com/flanksource/telltale/sdk/protocol"
)

// The gops agent's diagnostics as telltale ops, so an agent reachable only
// through a telltale hub answers what `gops <pid>` answers locally. Profiles
// and stack sampling stay with the SDK's goprof and stacks probes.

type op struct {
	spec protocol.OpSpec
	call func(json.RawMessage) (any, error)
}

func (o op) Spec() protocol.OpSpec { return o.spec }

func (o op) Call(_ context.Context, params json.RawMessage) (any, []protocol.Blob, error) {
	value, err := o.call(params)
	return value, nil, err
}

// MemStats is the subset of runtime.MemStats gops prints.
type MemStats struct {
	Alloc, TotalAlloc, Sys, Lookups, Mallocs, Frees    uint64
	HeapAlloc, HeapSys, HeapIdle, HeapInuse            uint64
	HeapReleased, HeapObjects, StackInuse, StackSys    uint64
	NextGC, LastGC, PauseTotalNs                       uint64
	NumGC, NumForcedGC                                 uint32
	GCCPUFraction                                      float64
	Goroutines, GOMAXPROCS, NumCPU                     int
}

// Version is the build identity gops reports, plus the module build info.
type Version struct {
	GoVersion string            `json:"goVersion"`
	Path      string            `json:"path,omitempty"`
	Main      string            `json:"main,omitempty"`
	Settings  map[string]string `json:"settings,omitempty"`
}

type setGCParams struct {
	Percent *int `json:"percent"`
}

func registerOps() {
	for _, o := range []op{
		{spec: protocol.OpSpec{Name: "memstats"}, call: func(json.RawMessage) (any, error) { return readMemStats(), nil }},
		{spec: protocol.OpSpec{Name: "version"}, call: func(json.RawMessage) (any, error) { return readVersion() }},
		{spec: protocol.OpSpec{Name: "stack"}, call: func(json.RawMessage) (any, error) { return goroutineDump(), nil }},
		{spec: protocol.OpSpec{Name: "gc", Mutates: true}, call: func(json.RawMessage) (any, error) {
			runtime.GC()
			return readMemStats(), nil
		}},
		{spec: protocol.OpSpec{Name: "setgc", Mutates: true}, call: setGC},
	} {
		sdk.RegisterOp(o)
	}
}

func readMemStats() MemStats {
	var s runtime.MemStats
	runtime.ReadMemStats(&s)
	return MemStats{Alloc: s.Alloc, TotalAlloc: s.TotalAlloc, Sys: s.Sys, Lookups: s.Lookups, Mallocs: s.Mallocs, Frees: s.Frees,
		HeapAlloc: s.HeapAlloc, HeapSys: s.HeapSys, HeapIdle: s.HeapIdle, HeapInuse: s.HeapInuse, HeapReleased: s.HeapReleased,
		HeapObjects: s.HeapObjects, StackInuse: s.StackInuse, StackSys: s.StackSys, NextGC: s.NextGC, LastGC: s.LastGC,
		PauseTotalNs: s.PauseTotalNs, NumGC: s.NumGC, NumForcedGC: s.NumForcedGC, GCCPUFraction: s.GCCPUFraction,
		Goroutines: runtime.NumGoroutine(), GOMAXPROCS: runtime.GOMAXPROCS(0), NumCPU: runtime.NumCPU()}
}

func readVersion() (Version, error) {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return Version{}, fmt.Errorf("version: binary has no build info")
	}
	v := Version{GoVersion: info.GoVersion, Path: info.Path, Main: info.Main.Path + "@" + info.Main.Version, Settings: map[string]string{}}
	for _, setting := range info.Settings {
		v.Settings[setting.Key] = setting.Value
	}
	return v, nil
}

func goroutineDump() string {
	buf := make([]byte, 1<<20)
	for {
		n := runtime.Stack(buf, true)
		if n < len(buf) {
			return string(buf[:n])
		}
		buf = make([]byte, 2*len(buf))
	}
}

// setGC sets the GC percentage like `gops setgc`, returning the previous one.
func setGC(raw json.RawMessage) (any, error) {
	var p setGCParams
	if err := json.Unmarshal(raw, &p); err != nil {
		return nil, fmt.Errorf("setgc params: %w", err)
	}
	if p.Percent == nil {
		return nil, fmt.Errorf("setgc params: percent is required (-1 disables GC)")
	}
	return map[string]int{"previous": debug.SetGCPercent(*p.Percent), "percent": *p.Percent}, nil
}

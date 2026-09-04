package flags

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/flanksource/commons/duration"
)

type reqOpts struct {
	Slice   []string          `flag:"slice"`
	IntList []int             `flag:"ints"`
	Name    string            `flag:"name"`
	Count   int               `flag:"count" default:"42"`
	Verbose bool              `flag:"verbose"`
	MaxAge  duration.Duration `flag:"max-age" default:"7d"`
	Since   time.Time         `flag:"since"`
}

func populate(t *testing.T, optsValue reflect.Value, flagMap map[string]string, args []string) {
	t.Helper()
	fields, err := ParseStructFields(reflect.TypeOf(reqOpts{}))
	if err != nil {
		t.Fatalf("ParseStructFields: %v", err)
	}
	if err := PopulateFromRequest(optsValue, fields, flagMap, args); err != nil {
		t.Fatalf("PopulateFromRequest: %v", err)
	}
}

// TestPopulateFromRequest_NoStateLeakAcrossCalls is the regression test for
// the bug that motivated this file: pflag's shared slice value appended on
// successive Set() calls, so a sequence of HTTP requests carried the union
// of every product/plan GUID ever supplied. The new path allocates a fresh
// opts struct per call, so a later call with no slice value must observe
// an empty slice rather than the prior call's tokens.
func TestPopulateFromRequest_NoStateLeakAcrossCalls(t *testing.T) {
	var first, second, third reqOpts
	populate(t, reflect.ValueOf(&first).Elem(), map[string]string{"slice": "A"}, nil)
	populate(t, reflect.ValueOf(&second).Elem(), map[string]string{"slice": "B"}, nil)
	populate(t, reflect.ValueOf(&third).Elem(), map[string]string{}, nil)

	if got, want := first.Slice, []string{"A"}; !reflect.DeepEqual(got, want) {
		t.Errorf("first.Slice = %v; want %v", got, want)
	}
	if got, want := second.Slice, []string{"B"}; !reflect.DeepEqual(got, want) {
		t.Errorf("second.Slice = %v; want %v (must replace, not append)", got, want)
	}
	if third.Slice != nil {
		t.Errorf("third.Slice = %v; want nil (must reset to zero)", third.Slice)
	}
}

func TestPopulateFromRequest_CSVExpansion(t *testing.T) {
	var opts reqOpts
	populate(t, reflect.ValueOf(&opts).Elem(), map[string]string{"slice": "A,B,C"}, nil)
	if got, want := opts.Slice, []string{"A", "B", "C"}; !reflect.DeepEqual(got, want) {
		t.Errorf("Slice = %v; want %v", got, want)
	}
}

func TestPopulateFromRequest_IntSliceCSV(t *testing.T) {
	var opts reqOpts
	populate(t, reflect.ValueOf(&opts).Elem(), map[string]string{"ints": "1,2,3"}, nil)
	if got, want := opts.IntList, []int{1, 2, 3}; !reflect.DeepEqual(got, want) {
		t.Errorf("IntList = %v; want %v", got, want)
	}
}

func TestPopulateFromRequest_ScalarParsing(t *testing.T) {
	var opts reqOpts
	populate(t, reflect.ValueOf(&opts).Elem(), map[string]string{
		"name":    "moshe",
		"count":   "7",
		"verbose": "true",
	}, nil)
	if opts.Name != "moshe" {
		t.Errorf("Name = %q; want moshe", opts.Name)
	}
	if opts.Count != 7 {
		t.Errorf("Count = %d; want 7", opts.Count)
	}
	if !opts.Verbose {
		t.Errorf("Verbose = false; want true")
	}
}

func TestPopulateFromRequest_DefaultsApply(t *testing.T) {
	var opts reqOpts
	populate(t, reflect.ValueOf(&opts).Elem(), nil, nil)
	if opts.Count != 42 {
		t.Errorf("Count = %d; want default 42", opts.Count)
	}
	expectedMaxAge, _ := duration.ParseDuration("7d")
	if opts.MaxAge != expectedMaxAge {
		t.Errorf("MaxAge = %v; want default 7d (%v)", opts.MaxAge, expectedMaxAge)
	}
}

// An `@` value off the wire names a file on the *server*. Expanding it for a
// field that never asked to read files is an arbitrary file read, so the
// default is to pass the value through as the literal string it is.
func TestPopulateFromRequest_AtFileIsLiteralWithoutOptIn(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "items.txt")
	if err := os.WriteFile(path, []byte("alpha\nbeta\ngamma\n"), 0o600); err != nil {
		t.Fatalf("write tmp file: %v", err)
	}

	var opts reqOpts
	populate(t, reflect.ValueOf(&opts).Elem(), map[string]string{"slice": "@" + path}, nil)
	if got, want := opts.Slice, []string{"@" + path}; !reflect.DeepEqual(got, want) {
		t.Errorf("Slice = %v; want %v (an un-opted-in field must not read the server's disk)", got, want)
	}

	var scalar reqOpts
	populate(t, reflect.ValueOf(&scalar).Elem(), map[string]string{"name": "@" + path}, nil)
	if scalar.Name != "@"+path {
		t.Errorf("Name = %q; want the literal %q", scalar.Name, "@"+path)
	}
}

// rpcFileOpts opts in, which is what a field genuinely naming a server-side
// document looks like.
type rpcFileOpts struct {
	Slice []string `flag:"slice" clicky:"rpc-file-read"`
	Name  string   `flag:"name" clicky:"cli-file-read rpc-file-read"`
}

func TestPopulateFromRequest_AtFileResolvesWhenOptedIn(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "items.txt")
	if err := os.WriteFile(path, []byte("alpha\nbeta\ngamma\n"), 0o600); err != nil {
		t.Fatalf("write tmp file: %v", err)
	}
	fields, err := ParseStructFields(reflect.TypeOf(rpcFileOpts{}))
	if err != nil {
		t.Fatalf("ParseStructFields: %v", err)
	}

	var opts rpcFileOpts
	if err := PopulateFromRequest(reflect.ValueOf(&opts).Elem(), fields,
		map[string]string{"slice": "@" + path, "name": "@" + path}, nil); err != nil {
		t.Fatalf("PopulateFromRequest: %v", err)
	}
	if got, want := opts.Slice, []string{"alpha", "beta", "gamma"}; !reflect.DeepEqual(got, want) {
		t.Errorf("Slice = %v; want %v", got, want)
	}
	if opts.Name != "alpha\nbeta\ngamma\n" {
		t.Errorf("Name = %q; want the file contents", opts.Name)
	}
}

// Opting in says "this field names a document", not "this field may read the
// host's secrets".
func TestPopulateFromRequest_OptInStillRefusesProtectedPaths(t *testing.T) {
	fields, err := ParseStructFields(reflect.TypeOf(rpcFileOpts{}))
	if err != nil {
		t.Fatalf("ParseStructFields: %v", err)
	}

	var opts rpcFileOpts
	err = PopulateFromRequest(reflect.ValueOf(&opts).Elem(), fields,
		map[string]string{"name": "@/etc/passwd"}, nil)
	if err == nil {
		t.Fatal("expected /etc/passwd to be refused even with rpc-file-read")
	}
	if !strings.Contains(err.Error(), "refusing to read") {
		t.Errorf("error = %v; want a refusal naming the protected path", err)
	}
}

// TestPopulateFromRequest_Concurrent fires the data-path under -race with
// interleaved payloads. Every result must reflect only its own input — if
// any goroutine sees another's tokens or scalar values, the race detector
// or the equality check will fail. 200 iterations × 2 workers is enough to
// catch a pflag-style shared-pointer regression every time on a typical
// laptop without dragging the suite out.
func TestPopulateFromRequest_Concurrent(t *testing.T) {
	const iterations = 200
	var wg sync.WaitGroup
	wg.Add(2)

	check := func(want []string, scalarName string, scalarCount int) {
		defer wg.Done()
		for i := 0; i < iterations; i++ {
			var opts reqOpts
			populate(t, reflect.ValueOf(&opts).Elem(), map[string]string{
				"slice": want[0],
				"name":  scalarName,
				"count": fmt.Sprintf("%d", scalarCount),
			}, nil)
			if !reflect.DeepEqual(opts.Slice, want) {
				t.Errorf("iter %d: Slice = %v; want %v", i, opts.Slice, want)
				return
			}
			if opts.Name != scalarName {
				t.Errorf("iter %d: Name = %q; want %q", i, opts.Name, scalarName)
				return
			}
			if opts.Count != scalarCount {
				t.Errorf("iter %d: Count = %d; want %d", i, opts.Count, scalarCount)
				return
			}
		}
	}

	go check([]string{"alpha"}, "worker-A", 1)
	go check([]string{"omega"}, "worker-B", 2)
	wg.Wait()
}

type argsOpts struct {
	IDs []string `flag:"ids" args:"true"`
}

func TestPopulateFromRequest_ArgsFallThrough(t *testing.T) {
	fields, err := ParseStructFields(reflect.TypeOf(argsOpts{}))
	if err != nil {
		t.Fatalf("ParseStructFields: %v", err)
	}
	var opts argsOpts
	if err := PopulateFromRequest(reflect.ValueOf(&opts).Elem(), fields, nil, []string{"a", "b"}); err != nil {
		t.Fatalf("PopulateFromRequest: %v", err)
	}
	if got, want := opts.IDs, []string{"a", "b"}; !reflect.DeepEqual(got, want) {
		t.Errorf("IDs = %v; want %v", got, want)
	}
}

// pathOpts mirrors the repomap `deps`/`images` path field: a positional arg
// with a non-empty default. The documented precedence is flag → args → default.
type pathOpts struct {
	Path string `flag:"path" args:"true" default:"."`
}

func populatePath(t *testing.T, flagMap map[string]string, args []string) pathOpts {
	t.Helper()
	fields, err := ParseStructFields(reflect.TypeOf(pathOpts{}))
	if err != nil {
		t.Fatalf("ParseStructFields: %v", err)
	}
	var opts pathOpts
	if err := PopulateFromRequest(reflect.ValueOf(&opts).Elem(), fields, flagMap, args); err != nil {
		t.Fatalf("PopulateFromRequest: %v", err)
	}
	return opts
}

func TestPopulateFromRequest_PositionalArgOverridesDefault(t *testing.T) {
	if got := populatePath(t, nil, []string{"/some/path"}).Path; got != "/some/path" {
		t.Errorf("Path = %q; want /some/path (positional arg must beat the default)", got)
	}
}

func TestPopulateFromRequest_DefaultWhenNoArg(t *testing.T) {
	if got := populatePath(t, nil, nil).Path; got != "." {
		t.Errorf("Path = %q; want . (default applies when no positional arg)", got)
	}
}

func TestPopulateFromRequest_ExplicitFlagBeatsArg(t *testing.T) {
	if got := populatePath(t, map[string]string{"path": "/from/flag"}, []string{"/from/arg"}).Path; got != "/from/flag" {
		t.Errorf("Path = %q; want /from/flag (explicit flag must beat the positional arg)", got)
	}
}

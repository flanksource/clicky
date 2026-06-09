package docs

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// newTestTree builds a synthetic CLI tree exercising flags, args, examples, a
// hidden command, a grouping command, and an entity-backed list operation with
// filter/limit parameters so the UI catalog has data.
func newTestTree() *cobra.Command {
	root := &cobra.Command{Use: "demo", Short: "Demo CLI", Long: "Demo CLI long description.", Version: "9.9.9"}

	run := func(cmd *cobra.Command, args []string) error { return nil }

	greet := &cobra.Command{
		Use:     "greet <name>",
		Short:   "Greet someone",
		Long:    "Print a greeting for the named person.",
		Example: "demo greet world",
		Args:    cobra.ExactArgs(1),
		RunE:    run,
	}
	greet.Flags().StringP("lang", "l", "en", "Language code")
	greet.Flags().String("mode", "false", "Mode named false")
	greet.Flags().Bool("shout", false, "Uppercase the greeting")
	_ = greet.MarkFlagRequired("lang")
	root.AddCommand(greet)

	hidden := &cobra.Command{Use: "secret", Short: "hidden", Hidden: true, RunE: run}
	root.AddCommand(hidden)

	// Grouping command (no Run) with a runnable child — group itself is skipped.
	group := &cobra.Command{Use: "stack", Short: "Manage stacks"}
	list := &cobra.Command{Use: "list", Short: "List stacks", RunE: run}
	list.Flags().String("env", "", "Filter by environment")
	list.Flags().Int("limit", 50, "Max results")
	// Annotate as a clicky-ui list surface with lookup support.
	setClickyMeta(list, "stack", "list", true)
	group.AddCommand(list)

	// A deeper sub-group so depth filtering has something to exclude:
	// "stack secret set" sits 2 levels below the "stack" controller.
	secret := &cobra.Command{Use: "secret", Short: "Manage stack secrets"}
	set := &cobra.Command{Use: "set", Short: "Set a stack secret", RunE: run}
	secret.AddCommand(set)
	group.AddCommand(secret)
	root.AddCommand(group)

	return root
}

// setClickyMeta tags a command with the stable clicky annotation keys the
// rpc.Converter reads to derive UI surface metadata.
func setClickyMeta(cmd *cobra.Command, entity, verb string, supportsLookup bool) {
	if cmd.Annotations == nil {
		cmd.Annotations = map[string]string{}
	}
	cmd.Annotations["clicky/entity-name"] = entity
	cmd.Annotations["clicky/operation-verb"] = verb
	if supportsLookup {
		cmd.Annotations["clicky/supports-lookup"] = "true"
	}
}

func buildTestModel(t *testing.T, cfg *DocsConfig) *Model {
	t.Helper()
	m, err := BuildModel(newTestTree(), cfg)
	if err != nil {
		t.Fatalf("BuildModel: %v", err)
	}
	return m
}

func findCommand(m *Model, path string) *CommandDoc {
	for i := range m.Commands {
		if m.Commands[i].Path == path {
			return &m.Commands[i]
		}
	}
	return nil
}

func findSurface(m *Model, command string) *SurfaceDoc {
	for i := range m.Surfaces {
		if m.Surfaces[i].Command == command {
			return &m.Surfaces[i]
		}
	}
	return nil
}

func TestBuildModelOmitsHiddenAndGroupCommands(t *testing.T) {
	m := buildTestModel(t, nil)

	if findCommand(m, "secret") != nil {
		t.Error("hidden command should be excluded from the reference")
	}
	if findCommand(m, "stack") != nil {
		t.Error("grouping command (no Run) should be excluded from the reference")
	}
	if findCommand(m, "greet") == nil {
		t.Error("runnable command 'greet' missing from the reference")
	}
	if findCommand(m, "stack list") == nil {
		t.Error("runnable command 'stack list' missing from the reference")
	}
}

func TestBuildModelExcludeConfig(t *testing.T) {
	m := buildTestModel(t, &DocsConfig{Exclude: []string{"greet"}})
	if findCommand(m, "greet") != nil {
		t.Error("excluded command 'greet' should not appear")
	}
}

func TestFlagDocsCaptureTypeDefaultRequired(t *testing.T) {
	m := buildTestModel(t, nil)
	greet := findCommand(m, "greet")
	if greet == nil {
		t.Fatal("greet command missing")
	}

	var lang, mode, shout *FlagDoc
	for i := range greet.Flags {
		switch greet.Flags[i].Name {
		case "lang":
			lang = &greet.Flags[i]
		case "mode":
			mode = &greet.Flags[i]
		case "shout":
			shout = &greet.Flags[i]
		}
	}
	if lang == nil || mode == nil || shout == nil {
		t.Fatalf("expected lang, mode, and shout flags, got %+v", greet.Flags)
	}
	if lang.Shorthand != "l" || lang.Type != "string" || lang.Default != "en" || !lang.Required {
		t.Errorf("lang flag metadata wrong: %+v", lang)
	}
	if mode.Type != "string" || mode.Default != "false" {
		t.Errorf("mode string default should keep literal false: %+v", mode)
	}
	if shout.Type != "bool" || shout.Required {
		t.Errorf("shout flag metadata wrong: %+v", shout)
	}
}

func TestSurfaceCatalogMapping(t *testing.T) {
	m := buildTestModel(t, nil)
	s := findSurface(m, "stack list")
	if s == nil {
		t.Fatalf("expected a 'stack list' surface, got surfaces: %+v", m.Surfaces)
	}
	if s.Entity != "stack" || s.Verb != "list" {
		t.Errorf("surface entity/verb wrong: %+v", s)
	}
	if s.Method != "GET" {
		t.Errorf("expected GET method for list op, got %q", s.Method)
	}
	if !strings.HasPrefix(s.Path, "/api/v1/") {
		t.Errorf("expected /api/v1 path, got %q", s.Path)
	}
	if !s.Lookup {
		t.Error("expected SupportsLookup to be true")
	}

	roles := map[string]string{}
	for _, p := range s.Parameters {
		roles[p.Name] = p.Role
	}
	if roles["limit"] != "limit" {
		t.Errorf("expected 'limit' param to have role 'limit', got %q", roles["limit"])
	}
	if roles["env"] != "filter" {
		t.Errorf("expected 'env' param to have role 'filter', got %q", roles["env"])
	}
}

func TestRenderCLIReferenceContent(t *testing.T) {
	m := buildTestModel(t, nil)
	out := RenderCLIReference(m)

	for _, want := range []string{"# CLI Reference", "## `greet`", "Greet someone", "--lang, -l", "demo greet world"} {
		if !strings.Contains(out, want) {
			t.Errorf("reference missing %q", want)
		}
	}
	if strings.Contains(out, "secret") {
		t.Error("reference should not mention hidden 'secret' command")
	}
}

func TestRenderUISurfacesContent(t *testing.T) {
	m := buildTestModel(t, nil)
	out := RenderUISurfaces(m)

	for _, want := range []string{"# UI Surface Catalog", "`stack list`", "filter (filter chip)", "limit (page size)"} {
		if !strings.Contains(out, want) {
			t.Errorf("surface catalog missing %q", want)
		}
	}
}

func TestRenderSingleFileFormats(t *testing.T) {
	m := buildTestModel(t, nil)

	md, err := RenderSingleFile(m, "markdown")
	if err != nil {
		t.Fatalf("markdown: %v", err)
	}
	if !strings.Contains(md, "# CLI Reference") || !strings.Contains(md, "# UI Surface Catalog") {
		t.Error("markdown single-file should contain both sections")
	}

	js, err := RenderSingleFile(m, "json")
	if err != nil {
		t.Fatalf("json: %v", err)
	}
	if !strings.Contains(js, "\"commands\"") || !strings.Contains(js, "\"surfaces\"") {
		t.Errorf("json single-file should serialize the model, got: %s", js[:min(120, len(js))])
	}

	if _, err := RenderSingleFile(m, "pdf"); err == nil {
		t.Error("expected unsupported --format pdf to fail loudly")
	}
}

func TestGenerateRejectsConflictingOutputFlags(t *testing.T) {
	root := newTestTree()
	root.AddCommand(NewCommand())
	root.SetArgs([]string{"docs", "generate", "--output", filepath.Join(t.TempDir(), "reference.md"), "--output-dir", t.TempDir()})
	if err := root.Execute(); err == nil {
		t.Fatal("expected conflicting --output and --output-dir to fail")
	}
}

func TestPageRelPathIsFlat(t *testing.T) {
	got := pageRelPath(Page{Key: controllerPageKey("stack")})
	if got != "stack.md" {
		t.Errorf("pageRelPath = %q, want stack.md", got)
	}
	if got := pageRelPath(Page{Key: pageIndex}); got != "index.md" {
		t.Errorf("pageRelPath = %q, want index.md", got)
	}
}

func TestScaffoldTwoTierWriteOnce(t *testing.T) {
	dir := t.TempDir()
	m := buildTestModel(t, nil)

	first, err := Scaffold(m, dir, false)
	if err != nil {
		t.Fatalf("first scaffold: %v", err)
	}
	for _, a := range first.Actions {
		if a.Status != "written" {
			t.Errorf("first run: expected all 'written', got %q for %s", a.Status, a.Path)
		}
	}

	// Edit a starter page and a generated page (a per-controller reference page).
	starter := filepath.Join(dir, pageRelPath(Page{Key: pageGettingStarted}))
	generated := filepath.Join(dir, pageRelPath(Page{Key: controllerPageKey("stack")}))
	if err := os.WriteFile(starter, []byte("EDITED STARTER"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(generated, []byte("EDITED GENERATED"), 0o644); err != nil {
		t.Fatal(err)
	}

	second, err := Scaffold(m, dir, false)
	if err != nil {
		t.Fatalf("second scaffold: %v", err)
	}
	statusByPath := map[string]string{}
	for _, a := range second.Actions {
		statusByPath[a.Path] = a.Status
	}
	if statusByPath[pageRelPath(Page{Key: pageGettingStarted})] != "skipped" {
		t.Error("starter page should be skipped on re-run without --force")
	}
	if statusByPath[pageRelPath(Page{Key: controllerPageKey("stack")})] != "regenerated" {
		t.Error("generated page should be regenerated on re-run")
	}

	if got := mustRead(t, starter); got != "EDITED STARTER" {
		t.Errorf("starter page was overwritten without --force: %q", got)
	}
	if got := mustRead(t, generated); got == "EDITED GENERATED" {
		t.Error("generated page should have been refreshed, but kept edited content")
	}
}

func TestScaffoldForceOverwritesStarter(t *testing.T) {
	dir := t.TempDir()
	m := buildTestModel(t, nil)

	if _, err := Scaffold(m, dir, false); err != nil {
		t.Fatal(err)
	}
	starter := filepath.Join(dir, pageRelPath(Page{Key: pageGettingStarted}))
	if err := os.WriteFile(starter, []byte("EDITED"), 0o644); err != nil {
		t.Fatal(err)
	}

	result, err := Scaffold(m, dir, true)
	if err != nil {
		t.Fatal(err)
	}
	for _, a := range result.Actions {
		if a.Skipped {
			t.Errorf("--force should not skip any page, but skipped %s", a.Path)
		}
	}
	if got := mustRead(t, starter); got == "EDITED" {
		t.Error("--force should overwrite the starter page")
	}
}

func findController(m *Model, name string) *ControllerDoc {
	for i := range m.Controllers {
		if m.Controllers[i].Name == name {
			return &m.Controllers[i]
		}
	}
	return nil
}

func TestControllersGroupByTopLevelCommand(t *testing.T) {
	m := buildTestModel(t, nil)

	// One controller per visible direct child of root; the hidden "secret"
	// top-level command is omitted.
	names := make([]string, 0, len(m.Controllers))
	for _, c := range m.Controllers {
		names = append(names, c.Name)
	}
	want := []string{"greet", "stack"}
	if strings.Join(names, ",") != strings.Join(want, ",") {
		t.Fatalf("controllers = %v, want %v", names, want)
	}

	greet := findController(m, "greet")
	if greet == nil || len(greet.Commands) != 1 || greet.Commands[0].Path != "greet" {
		t.Errorf("greet controller should hold exactly its own command, got %+v", greet)
	}
}

func TestControllerDepthDefaultExcludesGrandchildren(t *testing.T) {
	// Default depth (1) documents the controller + its direct subcommands, so
	// "stack list" appears but the depth-2 "stack secret set" does not.
	m := buildTestModel(t, nil)
	stack := findController(m, "stack")
	if stack == nil {
		t.Fatal("stack controller missing")
	}
	if findCommandIn(stack, "stack list") == nil {
		t.Error("depth=1 should include direct subcommand 'stack list'")
	}
	if findCommandIn(stack, "stack secret set") != nil {
		t.Error("depth=1 should exclude grandchild 'stack secret set'")
	}
}

func TestControllerDepthTwoIncludesGrandchildren(t *testing.T) {
	m := buildTestModel(t, &DocsConfig{Depth: 2})
	stack := findController(m, "stack")
	if stack == nil {
		t.Fatal("stack controller missing")
	}
	if findCommandIn(stack, "stack secret set") == nil {
		t.Error("depth=2 should include grandchild 'stack secret set'")
	}
}

func TestControllerDepthUnlimited(t *testing.T) {
	m := buildTestModel(t, &DocsConfig{Depth: unlimitedDepth})
	stack := findController(m, "stack")
	if findCommandIn(stack, "stack secret set") == nil {
		t.Error("unlimited depth should include all descendants")
	}
}

func findCommandIn(c *ControllerDoc, path string) *CommandDoc {
	for i := range c.Commands {
		if c.Commands[i].Path == path {
			return &c.Commands[i]
		}
	}
	return nil
}

func TestRenderControllerProducesOneDocument(t *testing.T) {
	m := buildTestModel(t, &DocsConfig{Depth: 2})
	stack := findController(m, "stack")
	out := RenderController(*stack)

	for _, want := range []string{"# stack", "Manage stacks", "## `stack list`", "## `stack secret set`"} {
		if !strings.Contains(out, want) {
			t.Errorf("controller page missing %q in:\n%s", want, out)
		}
	}
}

func TestScaffoldWritesControllerFilesDirectlyInOutputDir(t *testing.T) {
	dir := t.TempDir()
	m := buildTestModel(t, nil)

	if _, err := Scaffold(m, dir, false); err != nil {
		t.Fatalf("scaffold: %v", err)
	}
	// One file per controller, directly under the output dir — no commands/ or
	// src/content/docs/ subtree, and no frontmatter.
	for _, name := range []string{"greet", "stack"} {
		body := mustRead(t, filepath.Join(dir, name+".md"))
		if strings.HasPrefix(body, "---") {
			t.Errorf("%s.md should have no frontmatter, got:\n%s", name, body[:min(40, len(body))])
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "commands")); !os.IsNotExist(err) {
		t.Error("should not create a commands/ subdir")
	}
	if _, err := os.Stat(filepath.Join(dir, "src")); !os.IsNotExist(err) {
		t.Error("should not create a src/content/docs subtree")
	}
}

func TestScaffoldFailsOnPathCollision(t *testing.T) {
	pages := []Page{
		{Key: pageIndex},
		{Key: controllerPageKey("index")}, // a command literally named "index"
	}
	if err := assertNoPathCollisions(pages); err == nil {
		t.Error("expected a collision error when a controller's filename matches a starter page")
	}
}

func TestScaffoldRejectsPathTraversalPageKey(t *testing.T) {
	pages := []Page{
		{Key: "../outside"},
	}
	if err := assertNoPathCollisions(pages); err == nil {
		t.Fatal("expected path traversal page key to fail")
	}
}

func mustRead(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(b)
}

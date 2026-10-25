package clicky_test

import (
	"os"
	"os/exec"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// Library consumers compile these packages' full import closure. Heavy engines
// live behind opt-in packages (api/filter/celfilter, api/filter/gomplatefilter,
// flags/urlfetch, formatters/excel, formatters/pdf) so they only reach
// binaries that ask for them.
var _ = Describe("dependency closure", func() {
	heavy := []string{
		"k8s.io/",
		"github.com/aws/",
		"github.com/flanksource/gomplate/",
		"github.com/flanksource/commons/http",
		"github.com/robertkrimen/otto",
		"github.com/yuin/gopher-lua",
		"github.com/shirou/gopsutil/v3/",
		"modernc.org/",
		"github.com/google/cel-go",
		"github.com/xuri/excelize",
		"github.com/playwright-community/playwright-go",
		"github.com/flanksource/clicky/formatters/pdf",
		"github.com/flanksource/clicky/formatters/excel",
	}

	nonStandardDeps := func(pkg string) []string {
		cmd := exec.Command("go", "list", "-deps", "-f", "{{if not .Standard}}{{.ImportPath}}{{end}}", pkg)
		cmd.Env = append(os.Environ(), "GOWORK=off")
		out, err := cmd.Output()
		Expect(err).NotTo(HaveOccurred(), "go list -deps %s", pkg)
		return strings.Fields(string(out))
	}

	matching := func(deps, prefixes []string) []string {
		var found []string
		for _, dep := range deps {
			for _, prefix := range prefixes {
				if strings.HasPrefix(dep, prefix) {
					found = append(found, dep)
					break
				}
			}
		}
		return found
	}

	for _, pkg := range []string{".", "./api", "./api/filter", "./flags", "./formatters", "./schemaform", "./task", "./exec", "./entity", "./rpc"} {
		It(pkg+" does not compile heavy optional dependencies", func() {
			Expect(matching(nonStandardDeps(pkg), heavy)).To(BeEmpty())
		})
	}

})

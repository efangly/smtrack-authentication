package internal

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// These tests enforce the hexagonal dependency rule described in CLAUDE.md:
// internal/core knows nothing about frameworks, drivers or adapters.
//
// The allowlists are a ratchet for the refactor in
// docs/refactor/hexagonal-plan.md. Each entry is a violation that exists today
// and the phase that removes it. When a phase removes one, its entry must be
// deleted too — a stale entry fails the test — so the list can only shrink.

const module = "github.com/efangly/smtrack-authentication"

// forbiddenInCore are import-path prefixes no core package may import.
var forbiddenInCore = []string{
	module + "/internal/adapters",
	module + "/config",
	"github.com/gofiber/",
	"gorm.io/",
	"github.com/golang-jwt/",
	"golang.org/x/crypto/",
	"github.com/redis/",
	"github.com/rabbitmq/",
	"mime/multipart",
	"net/http",
}

// layerRules forbid inward packages from depending on outward ones.
var layerRules = map[string][]string{
	module + "/internal/core/domain": {module + "/internal/core/ports", module + "/internal/core/services"},
	module + "/internal/core/ports":  {module + "/internal/core/services"},
}

// importAllowlist maps "package -> import" to the phase that removes it.
var importAllowlist = map[string]string{
	"internal/core/ports -> mime/multipart":    "Phase 3",
	"internal/core/services -> mime/multipart": "Phase 3",
}

// tagAllowlist maps a domain file to the phase that strips its gorm/json tags.
var tagAllowlist = map[string]string{
	"audit.go":    "Phase 6",
	"hospital.go": "Phase 6",
	"user.go":     "Phase 6",
	"ward.go":     "Phase 6",
}

type pkgImports struct {
	path    string
	imports []string
}

func corePackages(t *testing.T) []pkgImports {
	t.Helper()
	out, err := exec.Command("go", "list", "-f", `{{.ImportPath}}{{range .Imports}} {{.}}{{end}}`, module+"/internal/core/...").Output()
	if err != nil {
		t.Fatalf("go list: %v", err)
	}
	var pkgs []pkgImports
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		fields := strings.Fields(line)
		pkgs = append(pkgs, pkgImports{path: fields[0], imports: fields[1:]})
	}
	return pkgs
}

func short(path string) string { return strings.TrimPrefix(path, module+"/") }

func TestArch_CoreImportsNoInfrastructure(t *testing.T) {
	seen := map[string]bool{}
	for _, pkg := range corePackages(t) {
		for _, imp := range pkg.imports {
			for _, bad := range forbiddenInCore {
				if imp != bad && !strings.HasPrefix(imp, bad) {
					continue
				}
				key := short(pkg.path) + " -> " + short(imp)
				if _, ok := importAllowlist[key]; ok {
					seen[key] = true
					continue
				}
				t.Errorf("%s imports %s: core must not depend on infrastructure; add a port instead", short(pkg.path), imp)
			}
		}
	}
	for key, phase := range importAllowlist {
		if !seen[key] {
			t.Errorf("allowlist entry %q (%s) no longer occurs; delete it from importAllowlist", key, phase)
		}
	}
}

func TestArch_CoreLayerDirection(t *testing.T) {
	for _, pkg := range corePackages(t) {
		for _, imp := range pkg.imports {
			for _, bad := range layerRules[pkg.path] {
				if imp == bad {
					t.Errorf("%s imports %s: dependencies must point inward (services -> ports -> domain)", short(pkg.path), short(imp))
				}
			}
		}
	}
}

// Domain types must not carry persistence or transport shape. go list cannot
// see struct tags, so this reads the source.
func TestArch_DomainHasNoInfrastructureTags(t *testing.T) {
	files, err := filepath.Glob(filepath.Join("core", "domain", "*.go"))
	if err != nil {
		t.Fatal(err)
	}
	tagged := map[string]bool{}
	fset := token.NewFileSet()
	for _, file := range files {
		if strings.HasSuffix(file, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, file, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		ast.Inspect(f, func(n ast.Node) bool {
			field, ok := n.(*ast.Field)
			if !ok || field.Tag == nil {
				return true
			}
			raw, err := strconv.Unquote(field.Tag.Value)
			if err != nil {
				return true
			}
			tag := reflect.StructTag(raw)
			for _, key := range []string{"gorm", "json"} {
				if _, ok := tag.Lookup(key); ok {
					tagged[filepath.Base(file)] = true
				}
			}
			return true
		})
	}

	var names []string
	for name := range tagged {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		if _, ok := tagAllowlist[name]; !ok {
			t.Errorf("core/domain/%s has gorm/json struct tags; map to a persistence model or DTO in the adapter instead", name)
		}
	}
	for name, phase := range tagAllowlist {
		if !tagged[name] {
			t.Errorf("tagAllowlist entry %q (%s) has no tags any more; delete it", name, phase)
		}
	}
}

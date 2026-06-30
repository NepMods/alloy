package main

import (
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"text/template"
)

type ModuleDef struct {
	Name        string           `json:"name"`
	Version     string           `json:"version"`
	Summary     string           `json:"summary"`
	GoPkg       string           `json:"go_pkg"`
	Provides    []PortDef        `json:"provides"`
	Requires    []PortDef        `json:"requires"`
	Permissions []PermissionDef  `json:"permissions"`
	Events      EventDef         `json:"events"`
	HTTP        *HTTPDef         `json:"http"`
}

type PortDef struct {
	Port  string `json:"port"`
	Iface string `json:"iface"`
}

type PermissionDef struct {
	Key         string `json:"key"`
	Description string `json:"description"`
}

type EventDef struct {
	Publishes  []TopicDef `json:"publishes"`
	Subscribes []TopicDef `json:"subscribes"`
}

type TopicDef struct {
	Topic   string `json:"topic"`
	Payload string `json:"payload"`
}

type HTTPDef struct {
	Mount string `json:"mount"`
}

func main() {
	if len(os.Args) >= 3 && os.Args[1] == "new" {
		cmdNew(os.Args[2])
		return
	}
	if len(os.Args) >= 3 && os.Args[1] == "delete" {
		cmdDelete(os.Args[2])
		return
	}
	if len(os.Args) == 2 && strings.Contains(os.Args[1], ":") {
		parts := strings.SplitN(os.Args[1], ":", 2)
		if parts[0] == "" || parts[1] == "" {
			fmt.Fprintf(os.Stderr, "usage: modgen [new <name>|delete <name>|<name>:<suffix>]\n")
			os.Exit(1)
		}
		cmdAdd(parts[0], parts[1])
		return
	}
	if len(os.Args) > 1 && os.Args[1] != "" {
		fmt.Fprintf(os.Stderr, "usage: modgen [new <name>|delete <name>|<name>:<suffix>]\n")
		os.Exit(1)
	}

	modulesDir := "internal/modules"
	entries, err := os.ReadDir(modulesDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error reading %s: %v\n", modulesDir, err)
		os.Exit(1)
	}

	var defs []ModuleDef
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		jsonPath := filepath.Join(modulesDir, e.Name(), "module.json")
		data, err := os.ReadFile(jsonPath)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			fmt.Fprintf(os.Stderr, "error reading %s: %v\n", jsonPath, err)
			os.Exit(1)
		}
		var def ModuleDef
		if err := json.Unmarshal(data, &def); err != nil {
			fmt.Fprintf(os.Stderr, "error parsing %s: %v\n", jsonPath, err)
			os.Exit(1)
		}
		if def.Name == "" {
			fmt.Fprintf(os.Stderr, "%s: missing name\n", jsonPath)
			os.Exit(1)
		}
		defs = append(defs, def)
	}

	sort.Slice(defs, func(i, j int) bool {
		return defs[i].Name < defs[j].Name
	})

	for _, d := range defs {
		genModule(d)
	}
	genBoot(defs)
}

var scaffoldSubdirs = []string{"domain", "dto", "fakes", "migrations", "models", "store", "testharness"}

func parseNameAndExtras(s string) (name, extras string) {
	if idx := strings.Index(s, ":"); idx >= 0 {
		return s[:idx], s[idx+1:]
	}
	return s, ""
}

func toPascalCase(s string) string {
	var result string
	for _, part := range strings.Split(s, "_") {
		if part != "" {
			result += strings.ToUpper(part[:1]) + part[1:]
		}
	}
	return result
}

func cmdAdd(name, suffix string) {
	valid := map[string]bool{
		"domain": true, "dto": true, "fakes": true, "migrations": true,
		"models": true, "store": true, "testharness": true, "test": true, "all": true,
	}
	if !valid[suffix] {
		fmt.Fprintf(os.Stderr, "error: unknown scaffold %q (valid: domain, dto, fakes, migrations, models, store, testharness, test, all)\n", suffix)
		os.Exit(1)
	}

	dirName := strings.ReplaceAll(strings.ToLower(name), "-", "_")
	modDir := filepath.Join("internal/modules", dirName)

	jsonPath := filepath.Join(modDir, "module.json")
	if _, err := os.Stat(jsonPath); os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "error: module %q not found\n", name)
		os.Exit(1)
	}

	typeName := toPascalCase(dirName)

	if suffix == "all" {
		for _, sub := range scaffoldSubdirs {
			createSubdir(modDir, dirName, typeName, sub)
		}
		createSubdir(modDir, dirName, typeName, "test")
	} else {
		createSubdir(modDir, dirName, typeName, suffix)
	}
}

func createDir(path string) {
	if err := os.MkdirAll(path, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "error creating %s: %v\n", path, err)
		os.Exit(1)
	}
	fmt.Println("created:", path+"/")
}

func writeFile(path, content string) {
	if _, err := os.Stat(path); err == nil {
		fmt.Printf("skip (exists): %s\n", path)
		return
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		fmt.Fprintf(os.Stderr, "error writing %s: %v\n", path, err)
		os.Exit(1)
	}
	fmt.Println("created:", path)
}

func createSubdir(modDir, dirName, typeName, sub string) {
	switch sub {
	case "domain":
		createDir(filepath.Join(modDir, "domain"))
	case "dto":
		const dtoSrc = `package dto

type Request struct {
	Name string ` + "`" + `json:"name"` + "`" + `
}

type Response struct {
	Message string ` + "`" + `json:"message"` + "`" + `
}
`
		createDir(filepath.Join(modDir, "dto"))
		writeFile(filepath.Join(modDir, "dto", "dto.go"), dtoSrc)
	case "fakes":
		createDir(filepath.Join(modDir, "fakes"))
		writeFile(filepath.Join(modDir, "fakes", "fakes.go"), fmt.Sprintf(`package fakes

import (
	"sync"

	"alloy/models/%s"
)

var _ %s.%sService = (*FakeService)(nil)

type FakeService struct {
	mu   sync.Mutex
	data map[string]string
}

func NewFakeService() *FakeService {
	return &FakeService{
		data: make(map[string]string),
	}
}

func (f *FakeService) Set(key, value string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.data[key] = value
}

func (f *FakeService) Get(key string) (string, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	v, ok := f.data[key]
	return v, ok
}
`, dirName, dirName, typeName))
	case "migrations":
		createDir(filepath.Join(modDir, "migrations"))
		writeFile(filepath.Join(modDir, "migrations", "migrations.go"), `package migrations

import "alloy/models/contract"

func Specs() []contract.MigrationSpec {
	return []contract.MigrationSpec{
		// {
		// 	Name: "create_example_table",
		// 	SQL:  "CREATE TABLE IF NOT EXISTS example (...) ;",
		// },
	}
}
`)
	case "models":
		createDir(filepath.Join(modDir, "models"))
		writeFile(filepath.Join(modDir, "models", "models.go"), `package models

type Model struct {
	ID   int64
	Name string
}

type Related struct {
	ID      int64
	ModelID int64
	Label   string
}

func (Model) TableName() string   { return "module_models" }
func (Related) TableName() string { return "module_related" }
`)
	case "store":
		createDir(filepath.Join(modDir, "store"))
		writeFile(filepath.Join(modDir, "store", "store.go"), `package store

import "sync"

type Store struct {
	mu   sync.Mutex
	data map[int64]string
}

func New() *Store {
	return &Store{
		data: make(map[int64]string),
	}
}

func (s *Store) Get(id int64) (string, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, ok := s.data[id]
	return v, ok
}

func (s *Store) Set(id int64, value string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data[id] = value
}
`)
	case "testharness":
		createDir(filepath.Join(modDir, "testharness"))
		writeFile(filepath.Join(modDir, "testharness", "harness.go"), fmt.Sprintf(`package testharness

import (
	"testing"

	"alloy/internal/modules/%s/fakes"
	"alloy/internal/modules/%s/service"
)

type Harness struct {
	Service *service.Service
	Fake    *fakes.FakeService
}

func New(t testing.TB) *Harness {
	t.Helper()
	return &Harness{
		Service: service.New(),
		Fake:    fakes.NewFakeService(),
	}
}
`, dirName, dirName))
	case "test":
		writeFile(filepath.Join(modDir, dirName+"_test.go"), fmt.Sprintf(`package %s_test

import (
	"testing"

	"alloy/internal/modules/%s/fakes"
	"alloy/internal/modules/%s/testharness"
)

func TestHarness(t *testing.T) {
	h := testharness.New(t)
	if h.Service == nil {
		t.Fatal("expected non-nil service")
	}
	if h.Fake == nil {
		t.Fatal("expected non-nil fake")
	}
}

func TestFakeService(t *testing.T) {
	fake := fakes.NewFakeService()
	fake.Set("hello", "world")
	v, ok := fake.Get("hello")
	if !ok {
		t.Fatal("expected key to exist")
	}
	if v != "world" {
		t.Fatalf("expected 'world', got %%q", v)
	}
}
`, dirName, dirName, dirName))
	}
}

func parseIface(s string) (pkgPath, typeName string) {
	lastDot := strings.LastIndex(s, ".")
	return s[:lastDot], s[lastDot+1:]
}

func pkgAlias(pkgPath string) string {
	return filepath.Base(pkgPath)
}

func payloadRef(payload string) string {
	if payload == "" || payload == "any" {
		return "reflect.TypeOf((*any)(nil)).Elem()"
	}
	if payload == "string" {
		return `reflect.TypeOf("")`
	}
	if !strings.Contains(payload, ".") {
		return fmt.Sprintf("reflect.TypeOf((*%s)(nil)).Elem()", payload)
	}
	pkg, typeName := parseIface(payload)
	alias := pkgAlias(pkg)
	return fmt.Sprintf("reflect.TypeOf((*%s.%s)(nil)).Elem()", alias, typeName)
}

func collectImports(d ModuleDef) []ImportSpec {
	seen := map[string]bool{}
	var imports []ImportSpec

	add := func(pkgPath string) {
		if pkgPath == "" || pkgPath == "alloy/models/contract" {
			return
		}
		if seen[pkgPath] {
			return
		}
		seen[pkgPath] = true
		alias := pkgAlias(pkgPath)
		imports = append(imports, ImportSpec{PkgPath: pkgPath, Alias: alias})
	}

	for _, p := range d.Provides {
		pkg, _ := parseIface(p.Iface)
		add(pkg)
	}
	for _, p := range d.Requires {
		pkg, _ := parseIface(p.Iface)
		add(pkg)
	}
	for _, e := range d.Events.Publishes {
		if e.Payload != "" && e.Payload != "string" && e.Payload != "any" && strings.Contains(e.Payload, ".") {
			pkg, _ := parseIface(e.Payload)
			add(pkg)
		}
	}
	for _, e := range d.Events.Subscribes {
		if e.Payload != "" && e.Payload != "string" && e.Payload != "any" && strings.Contains(e.Payload, ".") {
			pkg, _ := parseIface(e.Payload)
			add(pkg)
		}
	}

	sort.Slice(imports, func(i, j int) bool {
		if imports[i].Alias != imports[j].Alias {
			return imports[i].Alias < imports[j].Alias
		}
		return imports[i].PkgPath < imports[j].PkgPath
	})
	return imports
}

type ImportSpec struct {
	PkgPath string
	Alias   string
}

var moduleGenSrc = `// Code generated by modgen. DO NOT EDIT.
package {{ .PackageName }}

import (
	"alloy/models/contract"
	"reflect"
{{- range .Imports }}
	{{ .Alias }} "{{ .PkgPath }}"
{{- end }}
)

func ifaceOf[T any]() reflect.Type {
	return reflect.TypeOf((*T)(nil)).Elem()
}

func (m *Module) Manifest() contract.Manifest {
	return contract.Manifest{
		Name:    "{{ .Def.Name }}",
		Version: "{{ .Def.Version }}",
		Summary: "{{ .Def.Summary }}",
		Provides: []contract.PortSpec{
{{- range .Def.Provides }}
			{Name: "{{ .Port }}", Iface: ifaceOf[{{ pkgType .Iface }}]()},
{{- end }}
		},
		Requires: []contract.PortSpec{
{{- range .Def.Requires }}
			{Name: "{{ .Port }}", Iface: ifaceOf[{{ pkgType .Iface }}]()},
{{- end }}
		},
		Permissions: []contract.Permission{
{{- range .Def.Permissions }}
			{Key: "{{ .Key }}", Description: "{{ .Description }}"},
{{- end }}
		},
		Events: []contract.EventSpec{
{{- range .Def.Events.Publishes }}
			{Name: "{{ .Topic }}", Direction: contract.EventPublished, Payload: {{ payloadRef .Payload }}},
{{- end }}
{{- range .Def.Events.Subscribes }}
			{Name: "{{ .Topic }}", Direction: contract.EventSubscribed, Payload: {{ payloadRef .Payload }}},
{{- end }}
		},
		Migrations: nil,
	}
}
{{- range .Def.Provides }}
func Provide{{ .Port }}(reg *contract.Registry, impl {{ pkgType .Iface }}) {
	reg.Provide(ifaceOf[{{ pkgType .Iface }}](), impl)
}
{{- end }}
{{- range .Def.Requires }}
func Require{{ .Port }}(reg *contract.Registry) {{ pkgType .Iface }} {
	return contract.RequireT[{{ pkgType .Iface }}](reg)
}
{{- end }}
`

type moduleGenData struct {
	PackageName string
	Def         ModuleDef
	Imports     []ImportSpec
}

var serviceBaseSrc = `// Code generated by modgen. DO NOT EDIT.
package service

import (
	contract "alloy/models/contract"
	cache "alloy/internal/platform/cache"
{{- if .Imports }}
{{- range .Imports }}
	{{ .Alias }} "{{ .PkgPath }}"
{{- end }}
{{- end }}
)

type BaseService struct {
	Runtime contract.Runtime
	Cache   *cache.Cache
{{- range .Def.Requires }}
	{{ .Port }} {{ pkgType .Iface }}
{{- end }}
}

func NewBaseService(rt contract.Runtime{{ range $i, $r := .Def.Requires }}, {{ $r.Port }} {{ pkgType $r.Iface }}{{ end }}) BaseService {
	return BaseService{
		Runtime: rt,
{{- range .Def.Requires }}
		{{ .Port }}: {{ .Port }},
{{- end }}
	}
}
`

func pkgType(iface string) string {
	if !strings.Contains(iface, ".") {
		return iface
	}
	pkg, typeName := parseIface(iface)
	alias := pkgAlias(pkg)
	return alias + "." + typeName
}

func payloadRefFunc(payload string) string {
	return payloadRef(payload)
}

func collectRequireImports(d ModuleDef) []ImportSpec {
	seen := map[string]bool{}
	var imports []ImportSpec
	add := func(pkgPath string) {
		if pkgPath == "" || seen[pkgPath] || pkgPath == "alloy/models/contract" {
			return
		}
		seen[pkgPath] = true
		imports = append(imports, ImportSpec{PkgPath: pkgPath, Alias: pkgAlias(pkgPath)})
	}
	for _, p := range d.Requires {
		pkg, _ := parseIface(p.Iface)
		add(pkg)
	}
	sort.Slice(imports, func(i, j int) bool {
		if imports[i].Alias != imports[j].Alias {
			return imports[i].Alias < imports[j].Alias
		}
		return imports[i].PkgPath < imports[j].PkgPath
	})
	return imports
}

func generateImplStubs(d ModuleDef) {
	modDir := strings.TrimPrefix(d.GoPkg, "alloy/")
	svcFile := filepath.Join(modDir, "service", "service.go")

	existing := map[string]bool{}
	if src, err := os.ReadFile(svcFile); err == nil {
		fset := token.NewFileSet()
		f, err := parser.ParseFile(fset, svcFile, src, 0)
		if err == nil {
			for _, decl := range f.Decls {
				fd, ok := decl.(*ast.FuncDecl)
				if !ok || fd.Recv == nil {
					continue
				}
				for _, r := range fd.Recv.List {
					star, ok := r.Type.(*ast.StarExpr)
					if ok {
						if id, ok := star.X.(*ast.Ident); ok && id.Name == "Service" {
							existing[fd.Name.Name] = true
						}
					} else if id, ok := r.Type.(*ast.Ident); ok && id.Name == "Service" {
						existing[fd.Name.Name] = true
					}
				}
			}
		}
	}

	var stubs strings.Builder

	for _, p := range d.Provides {
		pkg, typeName := parseIface(p.Iface)
		modelFile := filepath.Join(strings.TrimPrefix(pkg, "alloy/"), "ports.go")

		fset := token.NewFileSet()
		f, err := parser.ParseFile(fset, modelFile, nil, 0)
		if err != nil {
			continue
		}

		for _, decl := range f.Decls {
			gd, ok := decl.(*ast.GenDecl)
			if !ok || gd.Tok != token.TYPE {
				continue
			}
			for _, spec := range gd.Specs {
				ts, ok := spec.(*ast.TypeSpec)
				if !ok || ts.Name.Name != typeName {
					continue
				}
				iface, ok := ts.Type.(*ast.InterfaceType)
				if !ok {
					continue
				}
				for _, m := range iface.Methods.List {
					if len(m.Names) == 0 {
						continue
					}
					mName := m.Names[0].Name
					if existing[mName] {
						continue
					}
					fnType, ok := m.Type.(*ast.FuncType)
					if !ok {
						continue
					}
					stubs.WriteString(genStub(fset, mName, fnType))
				}
			}
		}
	}

	// Always generate service_gen.go with New()
	svcGen := "// Code generated by modgen. DO NOT EDIT.\npackage service\n"
	svcGen += "\nfunc New() *Service {\n\treturn &Service{}\n}\n"

	svcGenPath := filepath.Join(modDir, "service", "service_gen.go")
	if err := os.MkdirAll(filepath.Dir(svcGenPath), 0755); err != nil {
		fmt.Fprintf(os.Stderr, "error creating dir for %s: %v\n", svcGenPath, err)
		os.Exit(1)
	}
	if err := os.WriteFile(svcGenPath, []byte(svcGen), 0644); err != nil {
		fmt.Fprintf(os.Stderr, "error writing %s: %v\n", svcGenPath, err)
		os.Exit(1)
	}
	fmt.Println("generated:", svcGenPath)

	// Generate impl_gen.go with stubs for missing methods
	outPath := filepath.Join(modDir, "service", "impl_gen.go")
	if stubs.Len() == 0 {
		if err := os.Remove(outPath); err == nil {
			fmt.Println("removed:", outPath)
		}
		return
	}

	content := fmt.Sprintf("// Code generated by modgen. DO NOT EDIT.\npackage service\n%s", stubs.String())
	if err := os.MkdirAll(filepath.Dir(outPath), 0755); err != nil {
		fmt.Fprintf(os.Stderr, "error creating dir for %s: %v\n", outPath, err)
		os.Exit(1)
	}
	if err := os.WriteFile(outPath, []byte(content), 0644); err != nil {
		fmt.Fprintf(os.Stderr, "error writing %s: %v\n", outPath, err)
		os.Exit(1)
	}
	fmt.Println("generated:", outPath)
}

func patchModuleGo(d ModuleDef) {
	modFile := filepath.Join(strings.TrimPrefix(d.GoPkg, "alloy/"), "module.go")
	src, err := os.ReadFile(modFile)
	if err != nil {
		return
	}
	lines := strings.Split(string(src), "\n")

	// Build Provide lines for Register
	svcAssign := "\tm.svc = service.New() // modgen:auto"
	var provideLines []string
	for _, p := range d.Provides {
		provideLines = append(provideLines, fmt.Sprintf("\tProvide%s(reg, m.svc) // modgen:auto", p.Port))
	}

	// Build Require lines for RequirementRegister — resolve deps into BaseService
	args := "rt"
	for _, r := range d.Requires {
		args += fmt.Sprintf(", Require%s(reg)", r.Port)
	}
	requireLines := []string{
		fmt.Sprintf("\tm.svc.BaseService = service.NewBaseService(%s) // modgen:auto", args),
		fmt.Sprintf("\tm.svc.BaseService.Cache = cache.New(%q, rt.Redis().RDB) // modgen:auto", d.Name),
	}

	// Patch Register: find m.svc = service.New() // modgen:auto and replace with svc + provides
	// Patch RequirementRegister: find return nil // modgen:auto and insert requires before it
	svcIdx := -1
	reqReturnIdx := -1
	for i, line := range lines {
		if strings.Contains(line, "// modgen:auto") && strings.Contains(line, "m.svc =") {
			svcIdx = i
		}
		if strings.Contains(line, "// modgen:auto") && strings.Contains(line, "return nil") {
			reqReturnIdx = i
		}
	}
	if svcIdx < 0 && reqReturnIdx < 0 {
		return
	}

	// Inject cache import if not already present
	hasCacheImport := false
	for _, line := range lines {
		if strings.Contains(line, `"alloy/internal/platform/cache"`) {
			hasCacheImport = true
			break
		}
	}
	if !hasCacheImport {
		// Find the closing paren of the import block and insert before it
		inImport := false
		var patched []string
		for _, line := range lines {
			patched = append(patched, line)
			if strings.TrimSpace(line) == "import (" {
				inImport = true
			} else if inImport && strings.TrimSpace(line) == ")" {
				patched = append(patched[:len(patched)-1], append([]string{`	"alloy/internal/platform/cache"`, ")"}, patched[len(patched):]...)...)
				inImport = false
			}
		}
		lines = patched
		// Recalculate indices after the edit
		svcIdx = -1
		reqReturnIdx = -1
		for i, line := range lines {
			if strings.Contains(line, "// modgen:auto") && strings.Contains(line, "m.svc =") {
				svcIdx = i
			}
			if strings.Contains(line, "// modgen:auto") && strings.Contains(line, "return nil") {
				reqReturnIdx = i
			}
		}
	}

	var newLines []string
	for i, line := range lines {
		if i == svcIdx {
			newLines = append(newLines, svcAssign)
			for _, pl := range provideLines {
				newLines = append(newLines, pl)
			}
			newLines = append(newLines, "\tm.svc.BaseService.Runtime = rt // modgen:auto")
			newLines = append(newLines, fmt.Sprintf("\tm.svc.BaseService.Cache = cache.New(%q, rt.Redis().RDB) // modgen:auto", d.Name))
		} else if i == reqReturnIdx {
			for _, rl := range requireLines {
				newLines = append(newLines, rl)
			}
			newLines = append(newLines, "\treturn nil // modgen:auto")
		} else if strings.Contains(line, "// modgen:auto") && (strings.HasPrefix(strings.TrimSpace(line), "Provide") || strings.HasPrefix(strings.TrimSpace(line), "Require") || strings.Contains(strings.TrimSpace(line), "m.svc.BaseService.Runtime = rt") || strings.Contains(strings.TrimSpace(line), "m.svc.BaseService.Cache =") || strings.Contains(strings.TrimSpace(line), "m.svc.BaseService = service.NewBaseService(")) {
			// Skip stale auto-generated lines (they'll be re-inserted above)
			continue
		} else {
			newLines = append(newLines, line)
		}
	}

	out := strings.Join(newLines, "\n")
	if err := os.WriteFile(modFile, []byte(out), 0644); err != nil {
		fmt.Fprintf(os.Stderr, "error writing %s: %v\n", modFile, err)
		os.Exit(1)
	}
	fmt.Println("patched:", modFile)
}

func genStub(fset *token.FileSet, name string, fn *ast.FuncType) string {
	params := formatFieldList(fset, fn.Params, true)
	results := formatFieldList(fset, fn.Results, false)
	retVals := returnStubValues(fset, fn.Results)

	if results == "" {
		return fmt.Sprintf("\nfunc (s *Service) %s(%s) {\n\t%s\n}\n", name, params, retVals)
	}
	return fmt.Sprintf("\nfunc (s *Service) %s(%s) %s {\n\t%s\n}\n", name, params, results, retVals)
}

func formatFieldList(fset *token.FileSet, fields *ast.FieldList, includeNames bool) string {
	if fields == nil || len(fields.List) == 0 {
		return ""
	}
	var parts []string
	for _, f := range fields.List {
		typeStr := typeExprString(fset, f.Type)
		if !includeNames || len(f.Names) == 0 {
			parts = append(parts, typeStr)
		} else {
			for _, n := range f.Names {
				parts = append(parts, n.Name+" "+typeStr)
			}
		}
	}
	if len(parts) > 1 {
		return "(" + strings.Join(parts, ", ") + ")"
	}
	return parts[0]
}

func typeExprString(fset *token.FileSet, expr ast.Expr) string {
	var buf strings.Builder
	printer.Fprint(&buf, fset, expr)
	return buf.String()
}

func returnStubValues(fset *token.FileSet, fields *ast.FieldList) string {
	if fields == nil || len(fields.List) == 0 {
		return ""
	}
	var parts []string
	for _, f := range fields.List {
		typeStr := typeExprString(fset, f.Type)
		parts = append(parts, "*new("+typeStr+")")
	}
	if len(parts) == 1 {
		return "return " + parts[0]
	}
	return "return " + strings.Join(parts, ", ")
}

func genModule(d ModuleDef) {
	basePkg := filepath.Base(d.GoPkg)
	imports := collectImports(d)
	reqImports := collectRequireImports(d)

	data := moduleGenData{
		PackageName: basePkg,
		Def:         d,
		Imports:     imports,
	}

	funcs := template.FuncMap{
		"pkgType":    pkgType,
		"payloadRef": payloadRefFunc,
	}

	t, err := template.New("moduleGen").Funcs(funcs).Parse(moduleGenSrc)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error parsing template: %v\n", err)
		os.Exit(1)
	}

	outPath := filepath.Join(strings.TrimPrefix(d.GoPkg, "alloy/"), "module_gen.go")

	var buf strings.Builder
	if err := t.Execute(&buf, data); err != nil {
		fmt.Fprintf(os.Stderr, "error generating %s: %v\n", outPath, err)
		os.Exit(1)
	}

	if err := os.MkdirAll(filepath.Dir(outPath), 0755); err != nil {
		fmt.Fprintf(os.Stderr, "error creating dir for %s: %v\n", outPath, err)
		os.Exit(1)
	}
	if err := os.WriteFile(outPath, []byte(buf.String()), 0644); err != nil {
		fmt.Fprintf(os.Stderr, "error writing %s: %v\n", outPath, err)
		os.Exit(1)
	}
	fmt.Println("generated:", outPath)

	// Generate service/base_gen.go
	reqData := moduleGenData{PackageName: basePkg, Def: d, Imports: reqImports}
	outPath = filepath.Join(strings.TrimPrefix(d.GoPkg, "alloy/"), "service", "base_gen.go")
	t2, err := template.New("serviceBase").Funcs(funcs).Parse(serviceBaseSrc)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error parsing service base template: %v\n", err)
		os.Exit(1)
	}
	var buf2 strings.Builder
	if err := t2.Execute(&buf2, reqData); err != nil {
		fmt.Fprintf(os.Stderr, "error generating %s: %v\n", outPath, err)
		os.Exit(1)
	}
	if err := os.MkdirAll(filepath.Dir(outPath), 0755); err != nil {
		fmt.Fprintf(os.Stderr, "error creating dir for %s: %v\n", outPath, err)
		os.Exit(1)
	}
	if err := os.WriteFile(outPath, []byte(buf2.String()), 0644); err != nil {
		fmt.Fprintf(os.Stderr, "error writing %s: %v\n", outPath, err)
		os.Exit(1)
	}
	fmt.Println("generated:", outPath)

	generateImplStubs(d)
	patchModuleGo(d)
}

var bootGenSrc = `// Code generated by modgen. DO NOT EDIT.
package boot

import (
	"alloy/internal/app/config"
	"alloy/models/contract"
{{- range .Imports }}
	{{ .Alias }} "{{ .PkgPath }}"
{{- end }}
)

func Modules(cfg config.Config, log func(string)) []contract.Module {
	return []contract.Module{
{{- range .Defs }}
		{{ moduleNew . }},
{{- end }}
	}
}
`

type bootGenData struct {
	Defs    []ModuleDef
	Imports []ImportSpec
}

func moduleNew(d ModuleDef) string {
	basePkg := filepath.Base(d.GoPkg)
	return fmt.Sprintf("%s.New(cfg, log)", basePkg)
}

func sortModulesByDeps(defs []ModuleDef) []ModuleDef {
	if len(defs) == 0 {
		return defs
	}
	// Build map: iface → module index that provides it
	providers := map[string]int{}
	for i, d := range defs {
		for _, p := range d.Provides {
			providers[p.Iface] = i
		}
	}
	// Build dependency graph
	graph := make([][]int, len(defs))
	inDegree := make([]int, len(defs))
	for i, d := range defs {
		for _, r := range d.Requires {
			if provider, ok := providers[r.Iface]; ok && provider != i {
				graph[provider] = append(graph[provider], i)
				inDegree[i]++
			}
		}
	}
	// Kahn's algorithm
	var queue []int
	for i := range defs {
		if inDegree[i] == 0 {
			queue = append(queue, i)
		}
	}
	var sorted []ModuleDef
	for len(queue) > 0 {
		u := queue[0]
		queue = queue[1:]
		sorted = append(sorted, defs[u])
		for _, v := range graph[u] {
			inDegree[v]--
			if inDegree[v] == 0 {
				queue = append(queue, v)
			}
		}
	}
	// If there's a cycle, append remaining modules in original order
	if len(sorted) < len(defs) {
		seen := map[int]bool{}
		for _, s := range sorted {
			for i := range defs {
				if defs[i].GoPkg == s.GoPkg {
					seen[i] = true
				}
			}
		}
		for i := range defs {
			if !seen[i] {
				sorted = append(sorted, defs[i])
			}
		}
	}
	return sorted
}

func genBoot(defs []ModuleDef) {
	defs = sortModulesByDeps(defs)
	seen := map[string]bool{}
	var imports []ImportSpec

	for _, d := range defs {
		if d.GoPkg == "" {
			continue
		}
		if seen[d.GoPkg] {
			continue
		}
		seen[d.GoPkg] = true
		alias := pkgAlias(d.GoPkg)
		imports = append(imports, ImportSpec{PkgPath: d.GoPkg, Alias: alias})
	}

	sort.Slice(imports, func(i, j int) bool {
		return imports[i].PkgPath < imports[j].PkgPath
	})

	data := bootGenData{
		Defs:    defs,
		Imports: imports,
	}

	funcs := template.FuncMap{
		"moduleNew": moduleNew,
	}

	t, err := template.New("bootGen").Funcs(funcs).Parse(bootGenSrc)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error parsing boot template: %v\n", err)
		os.Exit(1)
	}

	outPath := "internal/app/boot/modules_gen.go"
	var buf strings.Builder
	if err := t.Execute(&buf, data); err != nil {
		fmt.Fprintf(os.Stderr, "error generating %s: %v\n", outPath, err)
		os.Exit(1)
	}
	if err := os.WriteFile(outPath, []byte(buf.String()), 0644); err != nil {
		fmt.Fprintf(os.Stderr, "error writing %s: %v\n", outPath, err)
		os.Exit(1)
	}
	fmt.Println("generated:", outPath)
}

func cmdNew(raw string) {
	name, extras := parseNameAndExtras(raw)
	if name == "" {
		fmt.Fprintf(os.Stderr, "error: module name is required\n")
		os.Exit(1)
	}
	dirName := strings.ReplaceAll(strings.ToLower(name), "-", "_")
	typeName := toPascalCase(dirName)

	modDir := filepath.Join("internal/modules", dirName)
	modelDir := filepath.Join("models", dirName)

	for _, d := range []string{modDir, filepath.Join(modDir, "service"), filepath.Join(modDir, "http"), modelDir} {
		if err := os.MkdirAll(d, 0755); err != nil {
			fmt.Fprintf(os.Stderr, "error creating %s: %v\n", d, err)
			os.Exit(1)
		}
	}

	files := map[string]string{
		filepath.Join(modDir, "module.json"): fmt.Sprintf(`{
  "name": "%s",
  "version": "0.0.1",
  "summary": "%s module",
  "go_pkg": "alloy/internal/modules/%s",
  "provides": [],
  "requires": [],
  "permissions": [],
  "events": {
    "publishes": [],
    "subscribes": []
  },
  "http": {
    "mount": "%s"
  }
}
`, typeName, typeName, dirName, dirName),

		filepath.Join(modDir, "module.go"): fmt.Sprintf(`package %s

import (
	"alloy/internal/app/config"
	%shttp "alloy/internal/modules/%s/http"
	"alloy/internal/modules/%s/service"
	"alloy/models/apidocs"
	"alloy/models/contract"

	"github.com/NepMods/ember"
)

type Module struct {
	cfg config.Config
	log func(string)
	svc *service.Service
}

func New(cfg config.Config, log func(string)) *Module {
	return &Module{cfg: cfg, log: log}
}

func (m *Module) Register(reg *contract.Registry, rt contract.Runtime) error {
	_, ok := rt.DB().Raw().(*ember.DB)
	if !ok {
		return ErrKernelDB{"expected *ember.DB from runtime"}
	}

	m.svc = service.New() // modgen:auto
	h := %shttp.New(rt, rt.Context(), m.svc)
	rt.HTTPRoot().Mount("%s", func(r contract.Router) {
		h.Mount(r)
	}, func(s string) {
		rt.Logger()(s)
	})
	m.log(m.Manifest().Name + " module registered, " + " version: " + m.Manifest().Version)
	return nil
}

func (m *Module) RequirementRegister(reg *contract.Registry, rt contract.Runtime) error {
	m.svc.BaseService = service.NewBaseService(rt) // modgen:auto
	return nil // modgen:auto
}

func (m *Module) RouteDocs() []apidocs.RouteDoc {
	return %shttp.RouteDocs()
}

func (m *Module) Log() func(string) { return m.log }

type ErrKernelDB struct{ msg string }

func (e ErrKernelDB) Error() string { return "core: " + e.msg }
`, dirName, dirName, dirName, dirName, dirName, dirName, dirName),

		filepath.Join(modDir, "service", "service.go"): `package service

// Add custom fields to the Service struct as needed (e.g. mutex, counters).
// The New() constructor is generated in service_gen.go.
type Service struct {
	BaseService
}

// Add method implementations here. Example:
//
//	func (s *Service) GetCount() int {
//	    return 0
//	}
`,

		filepath.Join(modDir, "http", "http.go"): fmt.Sprintf(`package http

import (
	"alloy/internal/modules/%s/service"
	alloy "alloy/internal/platform/alloy"
	"alloy/models/contract"
	"context"
	"net/http"
)

type Handlers struct {
	app contract.Runtime
	ctx context.Context
	svc *service.Service
}

func New(app contract.Runtime, ctx context.Context, svc *service.Service) *Handlers {
	return &Handlers{app: app, ctx: ctx, svc: svc}
}

func (h *Handlers) Mount(r contract.Router) {
	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		alloy.OK(w, r, "%s module ready")
	})
}
`, dirName, typeName),

		filepath.Join(modDir, "http", "routes.go"): fmt.Sprintf(`package http

import (
	"alloy/internal/server"
	"alloy/models/apidocs"
)

func RouteDocs() []apidocs.RouteDoc {
	return []apidocs.RouteDoc{
		{
			Method:      "GET",
			Path:        "/v1/%s/",
			Summary:     "%s root",
			Auth:        "none",
			RequestBody: &server.CheckRequestHasNothing{},
			Response:    &server.CheckResponse{},
		},
	}
}
`, dirName, typeName),

		filepath.Join(modelDir, "ports.go"): fmt.Sprintf(`package %s

// %sService is the port interface for the %s module.
type %sService interface {
}
`, dirName, typeName, typeName, typeName),
	}

	for path, content := range files {
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			fmt.Fprintf(os.Stderr, "error writing %s: %v\n", path, err)
			os.Exit(1)
		}
		fmt.Println("created:", path)
	}

	if extras == "all" {
		for _, sub := range scaffoldSubdirs {
			createSubdir(modDir, dirName, typeName, sub)
		}
		createSubdir(modDir, dirName, typeName, "test")
	}
}

func cmdDelete(name string) {
	dirName := strings.ReplaceAll(strings.ToLower(name), "-", "_")
	modDir := filepath.Join("internal/modules", dirName)
	modelDir := filepath.Join("models", dirName)

	removed := false
	for _, d := range []string{modDir, modelDir} {
		if err := os.RemoveAll(d); err != nil {
			fmt.Fprintf(os.Stderr, "error removing %s: %v\n", d, err)
			os.Exit(1)
		}
		if _, err := os.Stat(d); os.IsNotExist(err) {
			fmt.Println("deleted:", d)
			removed = true
		}
	}

	if !removed {
		fmt.Printf("module %q not found\n", name)
		return
	}

	// Remove stale generated files
	os.Remove(filepath.Join("internal/modules", dirName, "module_gen.go"))

	// Regenerate boot modules list
	modulesDir := "internal/modules"
	entries, err := os.ReadDir(modulesDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error reading %s: %v\n", modulesDir, err)
		os.Exit(1)
	}

	var defs []ModuleDef
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		jsonPath := filepath.Join(modulesDir, e.Name(), "module.json")
		data, err := os.ReadFile(jsonPath)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			fmt.Fprintf(os.Stderr, "error reading %s: %v\n", jsonPath, err)
			os.Exit(1)
		}
		var def ModuleDef
		if err := json.Unmarshal(data, &def); err != nil {
			fmt.Fprintf(os.Stderr, "error parsing %s: %v\n", jsonPath, err)
			os.Exit(1)
		}
		if def.Name == "" {
			fmt.Fprintf(os.Stderr, "%s: missing name\n", jsonPath)
			os.Exit(1)
		}
		defs = append(defs, def)
	}
	genBoot(defs)
}

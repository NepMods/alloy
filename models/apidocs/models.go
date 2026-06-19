// Package apidocs provides TUI-style API route documentation printing.
// Modules declare RouteDoc entries describing their endpoints; PrintAPIRoutes
// renders them in a split-pane layout (logs placeholder left, routes right)
// using lipgloss for styling.
package apidocs

import (
	"fmt"
	"os"
	"reflect"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

// FieldDoc describes a single request/response field extracted from struct tags.
type FieldDoc struct {
	Name     string // JSON field name
	Type     string // Go type (string, int64, etc.)
	Location string // "body", "query", "header"
	Required bool
	Validate string // raw validate tag value (e.g. "required,email")
}

// RouteDoc describes a single API route for documentation purposes.
type RouteDoc struct {
	Method      string // GET, POST, PUT, PATCH, DELETE
	Path        string // e.g. /v1/core/auth/register
	Summary     string // one-line description
	Auth        string // e.g. "none", "bearer", "bearer + core.members.read"
	RequestBody any    // pointer to request DTO struct (nil if no body)
	Response    any    // pointer to response DTO struct (nil if no response)
	Fields      []FieldDoc
}

// ExtractFields uses reflect to pull field metadata from a DTO struct's
// json and validate tags. The struct must be passed as a pointer.
func ExtractFields(dto any) []FieldDoc {
	if dto == nil {
		return nil
	}
	t := reflect.TypeOf(dto)
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	if t.Kind() != reflect.Struct {
		return nil
	}
	var fields []FieldDoc
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if !f.IsExported() {
			continue
		}
		name := f.Tag.Get("json")
		if name == "-" {
			continue
		}
		// Handle "name,omitempty" → "name"
		if idx := strings.Index(name, ","); idx != -1 {
			name = name[:idx]
		}
		if name == "" {
			name = f.Name
		}
		validate := f.Tag.Get("validate")
		required := strings.Contains(validate, "required")

		fields = append(fields, FieldDoc{
			Name:     name,
			Type:     goTypeName(f.Type),
			Location: "body",
			Required: required,
			Validate: validate,
		})
	}
	return fields
}

func goTypeName(t reflect.Type) string {
	switch {
	case t == reflect.TypeOf(""):
		return "string"
	case t == reflect.TypeOf(int64(0)):
		return "int64"
	case t == reflect.TypeOf(int(0)):
		return "int"
	case t == reflect.TypeOf(float64(0)):
		return "float64"
	case t == reflect.TypeOf(false):
		return "bool"
	case t == reflect.TypeOf(time.Time{}):
		return "time"
	case t.Kind() == reflect.Slice:
		return "array"
	case t.Kind() == reflect.Ptr:
		return goTypeName(t.Elem()) + " (nullable)"
	case t.Kind() == reflect.Map:
		return "object"
	default:
		// Strip package prefix.
		name := t.String()
		pkg := t.PkgPath() + "."
		if strings.HasPrefix(name, pkg) {
			name = name[len(pkg):]
		}
		return name
	}
}

// PrintAPIRoutes renders all route docs in a split-pane TUI layout:
//   - Left panel: logPanel (or a static placeholder)
//   - Right panel: route documentation table
//
// Returns the rendered panel height so the log panel can match.
// Call this once at boot after all modules are registered.
func PrintAPIRoutes(docs []RouteDoc, log func(string)) int {
	// Sort routes: by path, then by method.
	sort.Slice(docs, func(i, j int) bool {
		if docs[i].Path != docs[j].Path {
			return docs[i].Path < docs[j].Path
		}
		return docs[i].Method < docs[j].Method
	})

	// ── Terminal width ──────────────────────────────────────────────

	docsWidth := termWidth() - 4
	if docsWidth < 70 {
		docsWidth = 70
	}

	// ── Colors ──────────────────────────────────────────────────────

	methodColors := map[string]string{
		"GET":    "#26A641",
		"POST":   "#0969DA",
		"PUT":    "#9A6700",
		"PATCH":  "#BF8700",
		"DELETE": "#CF222E",
	}

	border := lipgloss.Color("#30363D")
	header := lipgloss.Color("#8B949E")
	sep := lipgloss.Color("#21262D")
	dim := lipgloss.Color("#484F58")
	authC := lipgloss.Color("#F0883E")
	fieldName := lipgloss.Color("#79C0FF")
	fieldType := lipgloss.Color("#D2A8FF")
	fieldValidate := lipgloss.Color("#7EE787")
	reqMark := lipgloss.NewStyle().Foreground(lipgloss.Color("#F85149")).Render("required")

	// ── Build right panel content ────────────────────────────────────

	var routeLines []string
	for _, d := range docs {
		if log != nil {
			log(d.Method + " " + d.Path)
		}
		// Auto-extract fields from DTO if not manually set.
		if d.Fields == nil {
			d.Fields = ExtractFields(d.RequestBody)
		}

		mc, ok := methodColors[d.Method]
		if !ok {
			mc = "#C9D1D9"
		}

		// Method badge (fixed 6-char width).
		mBadge := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(mc)).Width(6).PaddingRight(1).Render(d.Method)

		pathS := lipgloss.NewStyle().Foreground(lipgloss.Color("#C9D1D9")).Bold(true)
		sumS := lipgloss.NewStyle().Foreground(lipgloss.Color("#E6EDF3"))
		authS := lipgloss.NewStyle().Foreground(authC).Faint(true)
		fnS := lipgloss.NewStyle().Foreground(fieldName)
		ftS := lipgloss.NewStyle().Foreground(fieldType)
		fvS := lipgloss.NewStyle().Foreground(fieldValidate)

		// Line 1: METHOD /path
		routeLines = append(routeLines, fmt.Sprintf("  %s %s", mBadge, pathS.Render(d.Path)))
		// Line 2: Summary
		if d.Summary != "" {
			routeLines = append(routeLines, fmt.Sprintf("        %s", sumS.Render(d.Summary)))
		}
		// Line 3: Auth
		if d.Auth != "" {
			routeLines = append(routeLines, fmt.Sprintf("        Auth: %s", authS.Render(d.Auth)))
		}
		// Lines 4+: Body fields
		if len(d.Fields) > 0 {
			routeLines = append(routeLines, fmt.Sprintf("        %s:",
				lipgloss.NewStyle().Foreground(header).Render("Body")))
			for _, f := range d.Fields {
				req := ""
				if f.Required {
					req = " " + reqMark
				}
				val := ""
				if f.Validate != "" {
					var nonReq []string
					for _, p := range strings.Split(f.Validate, ",") {
						p = strings.TrimSpace(p)
						if p != "required" && p != "omitempty" {
							nonReq = append(nonReq, p)
						}
					}
					if len(nonReq) > 0 {
						val = " " + fvS.Render(strings.Join(nonReq, ", "))
					}
				}
				line := fmt.Sprintf("          %-20s %-12s%s%s",
					fnS.Render(f.Name), ftS.Render(f.Type), val, req)
				routeLines = append(routeLines, line)
			}
		}
		// Response type
		if d.Response != nil {
			rn := typeName(d.Response)
			routeLines = append(routeLines, fmt.Sprintf("        Response: %s", ftS.Render(rn)))
		}

		routeLines = append(routeLines, "")
	}

	rightContent := strings.Join(routeLines, "\n")

	panelHeight := len(routeLines) + 2

	// ── Panel (Routes) ──────────────────────────────────────────────

	titleS := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#58A6FF"))
	countS := lipgloss.NewStyle().Foreground(dim).Render(fmt.Sprintf("%d routes", len(docs)))
	sepLine := lipgloss.NewStyle().Foreground(sep).Render(strings.Repeat("─", docsWidth-4))

	head := titleS.Render("API Routes") + "  " + countS + "\n" + sepLine + "\n"
	panel := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(border).
		Padding(0, 1).
		Width(docsWidth).
		Height(panelHeight).
		Render(head + rightContent)

	if log != nil {
		log(panel)
	}

	return panelHeight
}

// typeName extracts the short type name from a pointer-to-struct.
func typeName(v any) string {
	if v == nil {
		return ""
	}
	t := reflect.TypeOf(v)
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	name := t.String()
	if idx := strings.LastIndex(name, "."); idx != -1 {
		name = name[idx+1:]
	}
	return name
}

// termWidth returns the terminal width, falling back to 120.
func termWidth() int {
	if w := os.Getenv("COLUMNS"); w != "" {
		if n := atoi(w); n > 0 {
			return n
		}
	}
	return 120
}

func atoi(s string) int {
	n := 0
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0
		}
		n = n*10 + int(c-'0')
	}
	return n
}

// LogPanel is a writable log buffer for the server-logs panel in the TUI.
type LogPanel struct {
	width int
	lines []string
}

func NewLogPanel() *LogPanel {
	return &LogPanel{}
}

func (p *LogPanel) SetWidth(w int) {
	p.width = w
}

func (p *LogPanel) AppendContent(s string) {
	p.lines = append(p.lines, s)
}

package tui

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ─── TUI (root of the tree) ─────────────────────────────────────────────────

type TUI struct {
	root    *Pane
	program *tea.Program
	focused *Pane
	width   int
	height  int
	status  string
}

func New() *TUI {
	t := &TUI{}
	t.root = &Pane{isLeaf: true, tui: t}
	t.focused = t.root
	return t
}

func (t *TUI) SplitVertically(a, b string) (*Pane, *Pane) {
	return t.root.SplitVertically(a, b)
}

func (t *TUI) SplitHorizontally(a, b string) (*Pane, *Pane) {
	return t.root.SplitHorizontally(a, b)
}

func (t *TUI) Run() error {
	t.program = tea.NewProgram(t, tea.WithAltScreen(), tea.WithMouseCellMotion())
	_, err := t.program.Run()
	return err
}

// ─── Pane (a node in the tree) ──────────────────────────────────────────────

type Pane struct {
	title     string
	isLeaf    bool
	direction string // "v" or "h"
	children  []*Pane
	content   []string
	view      viewport.Model
	tui       *TUI
}

func (p *Pane) SetContent(s string) {
	p.content = []string{s}
	if p.tui != nil && p.tui.program != nil && p.isLeaf {
		p.tui.program.Send(updateContentMsg{pane: p})
	}
}

func (p *Pane) AppendContent(s string) {
	p.content = append(p.content, s)
	if p.tui != nil && p.tui.program != nil && p.isLeaf {
		p.tui.program.Send(appendContentMsg{pane: p, line: s})
	}
}

func (p *Pane) SetTitle(s string) {
	p.title = s
}

func (p *Pane) SplitVertically(a, b string) (*Pane, *Pane) {
	p.isLeaf = false
	p.direction = "v"
	p.children = []*Pane{
		{title: a, isLeaf: true, tui: p.tui},
		{title: b, isLeaf: true, tui: p.tui},
	}
	p.tui.focused = p.children[0]
	p.tui.triggerResize()
	return p.children[0], p.children[1]
}

func (p *Pane) SplitHorizontally(a, b string) (*Pane, *Pane) {
	p.isLeaf = false
	p.direction = "h"
	p.children = []*Pane{
		{title: a, isLeaf: true, tui: p.tui},
		{title: b, isLeaf: true, tui: p.tui},
	}
	p.tui.focused = p.children[0]
	p.tui.triggerResize()
	return p.children[0], p.children[1]
}

// ─── Messages ───────────────────────────────────────────────────────────────

type appendContentMsg struct {
	pane *Pane
	line string
}

type updateContentMsg struct {
	pane *Pane
}

// ─── tea.Model ──────────────────────────────────────────────────────────────

func (t *TUI) Init() tea.Cmd { return nil }

func (t *TUI) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch m := msg.(type) {
	case tea.WindowSizeMsg:
		t.width = m.Width
		t.height = m.Height
		t.resizePane(t.root, m.Width, m.Height-1)

	case appendContentMsg:
		if m.pane.isLeaf {
			m.pane.view.SetContent(strings.Join(m.pane.content, "\n"))
			m.pane.view.GotoBottom()
		}

	case updateContentMsg:
		if m.pane.isLeaf {
			m.pane.view.SetContent(strings.Join(m.pane.content, "\n"))
		}

	case tea.KeyMsg:
		switch m.String() {
		case "ctrl+c", "q":
			return t, tea.Quit
		case "tab":
			t.focusNext()
		case "c":
			t.copyFocused()
		case "up", "down", "pgup", "pgdown":
			if t.focused != nil && t.focused.isLeaf {
				var cmd tea.Cmd
				t.focused.view, cmd = t.focused.view.Update(msg)
				return t, cmd
			}
		case "home":
			if t.focused != nil && t.focused.isLeaf {
				t.focused.view.GotoTop()
			}
		case "end":
			if t.focused != nil && t.focused.isLeaf {
				t.focused.view.GotoBottom()
			}
		}

	default:
		if t.focused != nil && t.focused.isLeaf {
			var cmd tea.Cmd
			t.focused.view, cmd = t.focused.view.Update(msg)
			return t, cmd
		}
	}

	return t, nil
}

// ─── Layout ─────────────────────────────────────────────────────────────────

func (t *TUI) resizePane(p *Pane, w, h int) {
	if p.isLeaf {
		iw := max(10, w-2)
		ih := max(1, h-3)
		if p.view.Width == 0 && p.view.Height == 0 {
			p.view = viewport.New(iw, ih)
		} else {
			p.view.Width = iw
			p.view.Height = ih
		}
		p.view.SetContent(strings.Join(p.content, "\n"))
	} else if p.direction == "v" {
		t.resizePane(p.children[0], w/2, h)
		t.resizePane(p.children[1], w-w/2, h)
	} else {
		t.resizePane(p.children[0], w, h/2)
		t.resizePane(p.children[1], w, h-h/2)
	}
}

func (t *TUI) triggerResize() {
	if t.program != nil && t.width > 0 {
		t.program.Send(tea.WindowSizeMsg{Width: t.width, Height: t.height})
	}
}

func (t *TUI) focusNext() {
	leaves := t.collectLeaves()
	if len(leaves) < 2 {
		return
	}
	for i, l := range leaves {
		if l == t.focused {
			t.focused = leaves[(i+1)%len(leaves)]
			return
		}
	}
	t.focused = leaves[0]
}

func (t *TUI) collectLeaves() []*Pane {
	var leaves []*Pane
	collect(t.root, &leaves)
	return leaves
}

func collect(p *Pane, leaves *[]*Pane) {
	if p.isLeaf {
		*leaves = append(*leaves, p)
		return
	}
	for _, c := range p.children {
		collect(c, leaves)
	}
}

func (t *TUI) copyFocused() {
	if t.focused == nil || !t.focused.isLeaf {
		return
	}
	text := strings.Join(t.focused.content, "\n")
	if err := copyToClipboard(text); err != "" {
		t.status = err
	} else {
		t.status = "Copied to clipboard"
	}
}

// ─── Rendering ──────────────────────────────────────────────────────────────

func (t *TUI) View() string {
	if t.width == 0 || t.height == 0 {
		return "Starting..."
	}

	focusedColor := lipgloss.Color("212")
	unfocusedColor := lipgloss.Color("240")

	var renderPane func(p *Pane, w, h int) string
	renderPane = func(p *Pane, w, h int) string {
		if p.isLeaf {
			borderColor := unfocusedColor
			title := " " + p.title + " "
			if p == t.focused {
				borderColor = focusedColor
				title = "▎" + p.title + " "
			}
			innerW := w - 2
			innerH := max(0, h-2)
			headerSty := lipgloss.NewStyle().Bold(true).Padding(0, 1).Foreground(lipgloss.Color("255")).Background(lipgloss.Color("63"))
			return lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(borderColor).
				Width(w).
				Height(innerH).
				Render(lipgloss.JoinVertical(lipgloss.Top,
					headerSty.Width(innerW).Render(title),
					p.view.View(),
				))
		}
		if p.direction == "v" {
			return lipgloss.JoinHorizontal(lipgloss.Top,
				renderPane(p.children[0], w/2, h),
				renderPane(p.children[1], w-w/2, h),
			)
		}
		return lipgloss.JoinVertical(lipgloss.Top,
			renderPane(p.children[0], w, h/2),
			renderPane(p.children[1], w, h-h/2),
		)
	}

	body := renderPane(t.root, t.width, t.height-1)

	statusText := " [Tab] Switch pane  [↑/↓/PgUp/PgDn] Scroll  [c] Copy  [q] Quit"
	if t.status != "" {
		statusText = " " + t.status
		t.status = ""
	}
	status := lipgloss.NewStyle().
		Height(1).Padding(0, 1).
		Foreground(lipgloss.Color("255")).
		Background(lipgloss.Color("236")).
		Width(t.width).
		Render(statusText)

	return lipgloss.JoinVertical(lipgloss.Top, body, status)
}

// ─── Clipboard ──────────────────────────────────────────────────────────────

func copyToClipboard(text string) string {
	if text == "" {
		return ""
	}
	var cmd *exec.Cmd
	switch {
	case hasCmd("pbcopy"):
		cmd = exec.Command("pbcopy")
	case hasCmd("xclip"):
		cmd = exec.Command("xclip", "-selection", "clipboard")
	case hasCmd("xsel"):
		cmd = exec.Command("xsel", "--clipboard", "--input")
	case hasCmd("wl-copy"):
		cmd = exec.Command("wl-copy")
	default:
		return "Install xclip/xsel/wl-copy for clipboard"
	}
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return fmt.Sprintf("Clipboard error: %v", err)
	}
	if err := cmd.Start(); err != nil {
		return fmt.Sprintf("Clipboard error: %v", err)
	}
	stdin.Write([]byte(text))
	stdin.Close()
	cmd.Wait()
	return ""
}

func hasCmd(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

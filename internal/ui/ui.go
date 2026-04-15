package ui

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"github.com/notfilippo/flak/internal/diff"
)

var (
	sAdd             = lipgloss.NewStyle().Foreground(lipgloss.Color("#3fb950"))
	sDel             = lipgloss.NewStyle().Foreground(lipgloss.Color("#f85149"))
	sLineNo          = lipgloss.NewStyle().Foreground(lipgloss.Color("#768390"))
	sHunk            = lipgloss.NewStyle().Foreground(lipgloss.Color("#388bfd")).Faint(true)
	sFile            = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#e6edf3"))
	sCursor          = lipgloss.NewStyle().Background(lipgloss.Color("#1e3a5f"))
	sCursorBar       = lipgloss.NewStyle().Foreground(lipgloss.Color("#388bfd"))
	sMatchBar        = lipgloss.NewStyle().Foreground(lipgloss.Color("#ffa657"))
	sSearchHighlight = lipgloss.NewStyle().Background(lipgloss.Color("#b58900")).Foreground(lipgloss.Color("#000000")).Bold(true)

	sComment = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#f0883e")).
			Background(lipgloss.Color("#1a1209"))
	sCommentBorder = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#f0883e")).
			Padding(0, 1)
	sHeader = lipgloss.NewStyle().
		Background(lipgloss.Color("#161b22")).
		Foreground(lipgloss.Color("#8b949e"))
	sHeaderTitle = lipgloss.NewStyle().
			Background(lipgloss.Color("#161b22")).
			Foreground(lipgloss.Color("#58a6ff")).
			Bold(true)
	sKey = lipgloss.NewStyle().
		Background(lipgloss.Color("#30363d")).
		Foreground(lipgloss.Color("#e6edf3")).
		PaddingLeft(1).PaddingRight(1)
	sStatus = lipgloss.NewStyle().
		Background(lipgloss.Color("#0d1117")).
		Foreground(lipgloss.Color("#8b949e"))
	sFuzzyPanel = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#388bfd")).
			Padding(0, 1)
	sFuzzySelected = lipgloss.NewStyle().
			Background(lipgloss.Color("#1c2128")).
			Foreground(lipgloss.Color("#e6edf3"))
	sFuzzyNormal = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#8b949e"))
	sFuzzySep = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#30363d"))
)

type lineKind int

const (
	kindFileHeader lineKind = iota
	kindHunk
	kindContext
	kindAdd
	kindRemove
	kindComment
)

// uiLine is one rendered row in the diff view.
type uiLine struct {
	kind       lineKind
	base       string // complete pre-rendered ANSI line (gutter + syntax-highlighted content)
	gutter     string // gutter portion only, used to rebuild with in-line search highlights
	text       string // plain text for search matching
	file       string
	oldNo      int
	newNo      int
	commentIdx int // index into model.comments; -1 when not a comment line
}

// fileEntry records where a file starts in the line list.
type fileEntry struct {
	path    string
	lineIdx int
	adds    int
	dels    int
}

type viewMode int

const (
	modeView    viewMode = iota
	modeComment          // creating a new comment
	modeEdit             // editing an existing comment
	modeFuzzy            // fuzzy file picker
	modeSearch           // incremental line search
)

type model struct {
	width, height int
	ready         bool

	lines      []uiLine
	fileStarts []fileEntry

	comments []diff.Comment

	cursor     int
	vp         viewport.Model
	ta         textarea.Model
	mode       viewMode
	editingIdx int // index of comment being edited; -1 when not editing

	fuzzyInput    textinput.Model
	fuzzyMatches  []int // indices into fileStarts
	fuzzySelected int

	searchInput   textinput.Model
	searchQuery   string // last applied query
	searchMatches []int  // line indices matching current query
	searchIdx     int    // position within searchMatches

	lineNoW int // digit width of widest line number in the diff
}

// Run launches the TUI and returns the comments left by the reviewer.
func Run(files []diff.FileDiff, tty *os.File) ([]diff.Comment, error) {
	m := newModel(files)
	p := tea.NewProgram(m, tea.WithAltScreen(), tea.WithInput(tty), tea.WithOutput(tty))
	final, err := p.Run()
	if err != nil {
		return nil, err
	}
	return final.(model).comments, nil
}

const (
	taHeight     = 4 // textarea height in lines
	fuzzyMaxRows = 8 // max visible rows in the fuzzy list
)

func newModel(files []diff.FileDiff) model {
	ta := textarea.New()
	ta.Placeholder = "Write a comment… (Ctrl+S submit · Esc cancel)"
	ta.ShowLineNumbers = false
	ta.CharLimit = 0
	ta.SetHeight(taHeight)

	fi := textinput.New()
	fi.Placeholder = "type to filter files…"
	fi.PromptStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#388bfd"))
	fi.Prompt = "  "

	si := textinput.New()
	si.PromptStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#e6edf3"))
	si.Prompt = "/"

	lines, fileStarts, lineNoW := buildLines(files)
	return model{
		lines:       lines,
		fileStarts:  fileStarts,
		lineNoW:     lineNoW,
		ta:          ta,
		editingIdx:  -1,
		fuzzyInput:  fi,
		searchInput: si,
	}
}

func (m model) Init() tea.Cmd { return nil }

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.ta.SetWidth(m.width - 6)
		m.fuzzyInput.Width = m.width - 8
		m.searchInput.Width = m.width - 4
		if !m.ready {
			m.vp = viewport.New(m.width, m.vpHeight())
			m.vp.SetContent(m.renderContent())
			m.ready = true
		} else {
			m.vp.Width = m.width
			m.vp.Height = m.vpHeight()
			m.vp.SetContent(m.renderContent())
			m.scrollToCursor()
		}
		return m, nil

	case tea.KeyMsg:
		switch m.mode {
		case modeComment, modeEdit:
			return m.updateTextarea(msg)
		case modeFuzzy:
			return m.updateFuzzy(msg)
		case modeSearch:
			return m.updateSearch(msg)
		default:
			return m.updateView(msg)
		}
	}

	if m.mode == modeComment || m.mode == modeEdit {
		var cmd tea.Cmd
		m.ta, cmd = m.ta.Update(msg)
		return m, cmd
	}
	return m, nil
}

func (m model) updateView(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "ctrl+c":
		return m, tea.Quit

	case "up", "k":
		m.cursor--
	case "down", "j":
		m.cursor++
	case "pgup", "ctrl+b", "ctrl+u":
		m.cursor -= m.vp.Height / 2
	case "pgdown", "ctrl+f", "ctrl+d":
		m.cursor += m.vp.Height / 2
	case "g", "home":
		m.cursor = 0
	case "G", "end":
		m.cursor = len(m.lines) - 1

	case "]", "}":
		for i := m.cursor + 1; i < len(m.lines); i++ {
			if m.lines[i].kind == kindFileHeader {
				m.cursor = i
				break
			}
		}
		m.clampCursor()
		m.vp.SetContent(m.renderContent())
		m.vp.SetYOffset(m.visualOffset(m.cursor))
		return m, nil
	case "[", "{":
		for i := m.cursor - 1; i >= 0; i-- {
			if m.lines[i].kind == kindFileHeader {
				m.cursor = i
				break
			}
		}
		m.clampCursor()
		m.vp.SetContent(m.renderContent())
		m.vp.SetYOffset(m.visualOffset(m.cursor))
		return m, nil

	case "f":
		m.mode = modeFuzzy
		m.vp.Height = m.vpHeight()
		m.fuzzyInput.Reset()
		m.fuzzyMatches = allFileIndices(len(m.fileStarts))
		m.fuzzySelected = 0
		return m, m.fuzzyInput.Focus()

	case "/":
		m.mode = modeSearch
		m.vp.Height = m.vpHeight()
		m.searchInput.Reset()
		return m, m.searchInput.Focus()

	case "n":
		if len(m.searchMatches) > 0 {
			m.searchIdx = (m.searchIdx + 1) % len(m.searchMatches)
			m.cursor = m.searchMatches[m.searchIdx]
			m.clampCursor()
			m.vp.SetContent(m.renderContent())
			m.scrollToCursor()
		}
		return m, nil
	case "N":
		if len(m.searchMatches) > 0 {
			m.searchIdx = (m.searchIdx - 1 + len(m.searchMatches)) % len(m.searchMatches)
			m.cursor = m.searchMatches[m.searchIdx]
			m.clampCursor()
			m.vp.SetContent(m.renderContent())
			m.scrollToCursor()
		}
		return m, nil

	case "o":
		cur := m.lines[m.cursor]
		if cur.file != "" && cur.kind != kindHunk && cur.kind != kindFileHeader {
			lineNo := cur.newNo
			if cur.kind == kindRemove {
				lineNo = cur.oldNo
			}
			if lineNo == 0 {
				lineNo = 1
			}
			editor := os.Getenv("VISUAL")
			if editor == "" {
				editor = os.Getenv("EDITOR")
			}
			if editor == "" {
				editor = "vi"
			}
			cmd := exec.Command(editor, fmt.Sprintf("+%d", lineNo), cur.file)
			return m, tea.ExecProcess(cmd, nil)
		}
		return m, nil

	case "c":
		cur := m.lines[m.cursor]
		if cur.kind == kindAdd || cur.kind == kindRemove || cur.kind == kindContext {
			m.mode = modeComment
			m.vp.Height = m.vpHeight()
			m.ta.Reset()
			m.ta.Placeholder = "Write a comment… (Ctrl+S submit · Esc cancel)"
			return m, m.ta.Focus()
		}
		return m, nil

	case "e", "enter":
		cur := m.lines[m.cursor]
		if cur.kind == kindComment {
			m.mode = modeEdit
			m.editingIdx = cur.commentIdx
			m.vp.Height = m.vpHeight()
			m.ta.Reset()
			m.ta.Placeholder = ""
			m.ta.SetValue(m.comments[cur.commentIdx].Body)
			return m, m.ta.Focus()
		}
		return m, nil

	case "x":
		cur := m.lines[m.cursor]
		if cur.kind == kindComment {
			m = m.deleteComment(cur.commentIdx)
			m.vp.SetContent(m.renderContent())
			m.scrollToCursor()
			return m, nil
		}
		return m, nil
	}

	m.clampCursor()
	m.vp.SetContent(m.renderContent())
	m.scrollToCursor()
	return m, nil
}

func (m model) updateTextarea(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.mode = modeView
		m.editingIdx = -1
		m.vp.Height = m.vpHeight()
		m.ta.Blur()
		return m, nil

	case "ctrl+s":
		body := strings.TrimSpace(m.ta.Value())
		if body != "" {
			if m.mode == modeEdit {
				m = m.applyEdit(body)
			} else {
				m = m.applyNewComment(body)
			}
		}
		m.mode = modeView
		m.editingIdx = -1
		m.vp.Height = m.vpHeight()
		m.ta.Blur()
		m.ta.Reset()
		m.vp.SetContent(m.renderContent())
		m.scrollToCursor()
		return m, nil
	}

	var cmd tea.Cmd
	m.ta, cmd = m.ta.Update(msg)
	return m, cmd
}

func (m model) updateFuzzy(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "ctrl+c":
		m.mode = modeView
		m.vp.Height = m.vpHeight()
		m.fuzzyInput.Blur()
		return m, nil

	case "enter":
		if len(m.fuzzyMatches) > 0 {
			fe := m.fileStarts[m.fuzzyMatches[m.fuzzySelected]]
			m.cursor = fe.lineIdx
			m.clampCursor()
		}
		m.mode = modeView
		m.vp.Height = m.vpHeight()
		m.fuzzyInput.Blur()
		m.vp.SetContent(m.renderContent())
		m.vp.SetYOffset(m.cursor)
		return m, nil

	case "up", "ctrl+p":
		if m.fuzzySelected > 0 {
			m.fuzzySelected--
		}
		return m, nil

	case "down", "ctrl+n":
		if m.fuzzySelected < len(m.fuzzyMatches)-1 {
			m.fuzzySelected++
		}
		return m, nil
	}

	var cmd tea.Cmd
	m.fuzzyInput, cmd = m.fuzzyInput.Update(msg)
	m.fuzzyMatches = fuzzyFilter(m.fileStarts, m.fuzzyInput.Value())
	if m.fuzzySelected >= len(m.fuzzyMatches) {
		m.fuzzySelected = max(0, len(m.fuzzyMatches)-1)
	}
	return m, cmd
}

func (m model) updateSearch(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.mode = modeView
		m.vp.Height = m.vpHeight()
		m.searchInput.Blur()
		return m, nil

	case "enter":
		query := strings.TrimSpace(m.searchInput.Value())
		m.searchQuery = query
		m.searchMatches = findMatches(m.lines, query)
		m.searchIdx = 0
		m.mode = modeView
		m.vp.Height = m.vpHeight()
		m.searchInput.Blur()
		if len(m.searchMatches) > 0 {
			m.cursor = m.searchMatches[0]
			m.clampCursor()
		}
		m.vp.SetContent(m.renderContent())
		m.scrollToCursor()
		return m, nil
	}

	var cmd tea.Cmd
	m.searchInput, cmd = m.searchInput.Update(msg)
	return m, cmd
}

func (m model) applyNewComment(body string) model {
	cur := m.lines[m.cursor]
	side, lineNo := sideAndLine(cur)
	c := diff.Comment{File: cur.file, Line: lineNo, Side: side, Body: body}
	idx := len(m.comments)
	m.comments = append(m.comments, c)

	cl := uiLine{kind: kindComment, base: renderCommentDisplay(c), text: c.Body, file: cur.file, commentIdx: idx}
	newLines := make([]uiLine, 0, len(m.lines)+1)
	newLines = append(newLines, m.lines[:m.cursor+1]...)
	newLines = append(newLines, cl)
	newLines = append(newLines, m.lines[m.cursor+1:]...)
	m.lines = newLines
	m.cursor++
	return m
}

func (m model) applyEdit(body string) model {
	m.comments[m.editingIdx].Body = body
	c := m.comments[m.editingIdx]
	for i := range m.lines {
		if m.lines[i].commentIdx == m.editingIdx {
			m.lines[i].base = renderCommentDisplay(c)
			m.lines[i].text = c.Body
			break
		}
	}
	return m
}

func (m model) deleteComment(idx int) model {
	m.comments = append(m.comments[:idx], m.comments[idx+1:]...)
	for i, ln := range m.lines {
		if ln.kind == kindComment && ln.commentIdx == idx {
			m.lines = append(m.lines[:i], m.lines[i+1:]...)
			if m.cursor >= len(m.lines) {
				m.cursor = len(m.lines) - 1
			}
			break
		}
	}
	for i := range m.lines {
		if m.lines[i].commentIdx > idx {
			m.lines[i].commentIdx--
		}
	}
	return m
}

func (m *model) clampCursor() {
	if m.cursor < 0 {
		m.cursor = 0
	}
	if m.cursor >= len(m.lines) {
		m.cursor = len(m.lines) - 1
	}
}

// gutterW returns the visible width of the line-number gutter.
// Format: "NNNNN NNNNN +/- " where N columns = m.lineNoW each.
func (m model) gutterW() int { return m.lineNoW*2 + 4 }

// wrapRows splits a pre-rendered ANSI diff line into visual rows when it
// exceeds availW visible columns. Returns nil if no wrapping is needed.
// Continuation rows get a blank gutter of the same width.
func (m model) wrapRows(base string, availW int) []string {
	gw := m.gutterW()
	rowW := availW - gw
	if rowW <= 0 {
		return nil
	}
	visContent := lipgloss.Width(base) - gw
	if visContent <= rowW {
		return nil
	}
	gutterAnsi := ansi.Cut(base, 0, gw)
	blank := strings.Repeat(" ", gw)
	rows := make([]string, 0, visContent/rowW+1)
	for off := 0; off < visContent; off += rowW {
		end := min(off+rowW, visContent)
		chunk := ansi.Cut(base, gw+off, gw+end)
		padded := chunk + strings.Repeat(" ", max(0, rowW-lipgloss.Width(chunk)))
		if off == 0 {
			rows = append(rows, gutterAnsi+padded)
		} else {
			rows = append(rows, blank+padded)
		}
	}
	return rows
}

// visualOffset returns the viewport line index for logical line at idx,
// accounting for wrapped lines above it.
func (m model) visualOffset(idx int) int {
	availW := m.width - 2
	off := 0
	for i := 0; i < idx && i < len(m.lines); i++ {
		ln := m.lines[i]
		switch ln.kind {
		case kindAdd, kindRemove, kindContext:
			rows := m.wrapRows(ln.base, availW)
			if rows != nil {
				off += len(rows)
				continue
			}
		}
		off++
	}
	return off
}

func (m *model) scrollToCursor() {
	if m.vp.Height < 1 {
		return
	}
	visStart := m.visualOffset(m.cursor)
	var visH int
	if m.cursor >= 0 && m.cursor < len(m.lines) {
		ln := m.lines[m.cursor]
		switch ln.kind {
		case kindAdd, kindRemove, kindContext:
			if rows := m.wrapRows(ln.base, m.width-2); rows != nil {
				visH = len(rows)
			}
		}
	}
	if visH == 0 {
		visH = 1
	}
	if visStart < m.vp.YOffset {
		m.vp.SetYOffset(visStart)
	} else if visStart+visH-1 > m.vp.YOffset+m.vp.Height-1 {
		m.vp.SetYOffset(visStart + visH - m.vp.Height)
	}
}

func (m model) vpHeight() int {
	var panel int
	switch m.mode {
	case modeComment, modeEdit:
		panel = 1 + (taHeight + 2) + 1
	case modeFuzzy:
		panel = 3 + 1 + fuzzyMaxRows + 1 + 1
	default:
		panel = 1 // status bar or search bar, same height
	}
	headerH := 1
	if m.renderHeader() == "" {
		headerH = 0
	}
	h := m.height - headerH - panel
	if h < 1 {
		h = 1
	}
	return h
}

func (m model) View() string {
	if !m.ready {
		return "loading…\n"
	}

	var parts []string
	if h := m.renderHeader(); h != "" {
		parts = append(parts, h)
	}
	parts = append(parts, m.vp.View())

	switch m.mode {
	case modeComment:
		cur := m.lines[m.cursor]
		side, lineNo := sideAndLine(cur)
		label := sComment.Bold(true).Render(
			fmt.Sprintf("  comment on line %d (%s)", lineNo, side),
		)
		box := sCommentBorder.Width(m.width - 4).Render(m.ta.View())
		hint := sStatus.Width(m.width).Render("  ctrl+s submit · esc cancel")
		parts = append(parts, label, box, hint)

	case modeEdit:
		c := m.comments[m.editingIdx]
		label := sComment.Bold(true).Render(
			fmt.Sprintf("  editing comment on line %d (%s)", c.Line, c.Side),
		)
		box := sCommentBorder.Width(m.width - 4).Render(m.ta.View())
		hint := sStatus.Width(m.width).Render("  ctrl+s save · esc cancel")
		parts = append(parts, label, box, hint)

	case modeFuzzy:
		parts = append(parts, m.renderFuzzyPanel())

	default:
		parts = append(parts, m.renderStatus())
	}

	return strings.Join(parts, "\n")
}

func (m model) renderFuzzyPanel() string {
	inputBox := sFuzzyPanel.Width(m.width - 4).Render(m.fuzzyInput.View())

	listLines := make([]string, fuzzyMaxRows)
	if len(m.fuzzyMatches) == 0 {
		listLines[0] = sFuzzyNormal.Render("  no matches")
	} else {
		for i := 0; i < min(len(m.fuzzyMatches), fuzzyMaxRows); i++ {
			fe := m.fileStarts[m.fuzzyMatches[i]]
			label := fmt.Sprintf("  %-50s %s %s",
				fe.path,
				sAdd.Render(fmt.Sprintf("+%d", fe.adds)),
				sDel.Render(fmt.Sprintf("-%d", fe.dels)),
			)
			if i == m.fuzzySelected {
				listLines[i] = sFuzzySelected.Width(m.width - 2).Render("▶ " + strings.TrimPrefix(label, "  "))
			} else {
				listLines[i] = sFuzzyNormal.Render(label)
			}
		}
	}
	sep := sFuzzySep.Render(strings.Repeat("─", m.width))
	hint := sStatus.Width(m.width).Render("  ↑↓/ctrl+p/n navigate · enter jump · esc close")
	return strings.Join(append([]string{inputBox, sep}, append(listLines, sep, hint)...), "\n")
}

func (m model) renderHeader() string {
	title := sHeaderTitle.Render("⬡ flak") + sHeader.Render(" review")
	if nc := len(m.comments); nc > 0 {
		title += sHeader.Render(fmt.Sprintf("  %d comment(s)", nc))
	}
	keys := sHeader.Render("  ") +
		sKey.Render("j/k") + sHeader.Render(" scroll  ") +
		sKey.Render("f") + sHeader.Render(" files  ") +
		sKey.Render("/") + sHeader.Render(" search  ") +
		sKey.Render("c") + sHeader.Render(" comment  ") +
		sKey.Render("o") + sHeader.Render(" open  ") +
		sKey.Render("q") + sHeader.Render(" quit  ")

	gap := m.width - lipgloss.Width(title) - lipgloss.Width(keys)
	if gap < 0 {
		return ""
	}
	return sHeader.Width(m.width).Render(title + strings.Repeat(" ", gap) + keys)
}

func (m model) renderStatus() string {
	// Show search position when a query is active.
	var searchInfo string
	if len(m.searchMatches) > 0 {
		searchInfo = sMatchBar.Render(fmt.Sprintf("  [%d/%d] n/N  ", m.searchIdx+1, len(m.searchMatches)))
	}

	pct := 100
	if len(m.lines) > 0 {
		pct = (m.cursor + 1) * 100 / len(m.lines)
	}
	right := fmt.Sprintf("%d%%  ", pct)
	rightW := lipgloss.Width(searchInfo) + len(right)

	// In search mode, embed the query inline in the status bar as plain text
	// so the dark background is preserved (textinput.View() has ANSI resets
	// that would blow out the sStatus background).
	if m.mode == modeSearch {
		query := m.searchInput.Value()
		left := "/" + query + "█"
		usedW := lipgloss.Width(left) + rightW
		pad := max(0, m.width-usedW)
		return sStatus.Width(m.width).Render(left + strings.Repeat(" ", pad) + searchInfo + right)
	}

	if m.cursor < 0 || m.cursor >= len(m.lines) {
		return sStatus.Width(m.width).Render("")
	}
	cur := m.lines[m.cursor]

	var left string
	switch cur.kind {
	case kindAdd:
		left = fmt.Sprintf("  %s:%d  +added", cur.file, cur.newNo)
	case kindRemove:
		left = fmt.Sprintf("  %s:%d  -removed", cur.file, cur.oldNo)
	case kindContext:
		if cur.newNo > 0 {
			left = fmt.Sprintf("  %s:%d", cur.file, cur.newNo)
		}
	case kindComment:
		left = "  " + sKey.Render("e") + sStatus.Render(" edit  ") +
			sKey.Render("x") + sStatus.Render(" delete")
	case kindFileHeader:
		left = "  " + sKey.Render("]") + sStatus.Render(" next file  ") +
			sKey.Render("[") + sStatus.Render(" prev file")
	}

	maxLeftW := m.width - rightW - 1 // reserve at least 1 space before the right side
	if lipgloss.Width(left) > maxLeftW {
		left = ansi.Truncate(left, max(0, maxLeftW), "")
	}
	usedW := lipgloss.Width(left) + rightW
	pad := max(0, m.width-usedW)
	return sStatus.Width(m.width).Render(left + strings.Repeat(" ", pad) + searchInfo + right)
}

const (
	addBg       = "#0d2818" // add line background (dark green tint, like Claude Code)
	addBgCursor = "#163d25" // add line background when cursor is on it
	delBg       = "#280d0d" // remove line background (dark red tint, like Claude Code)
	delBgCursor = "#3d1616" // remove line background when cursor is on it
)

// padLine clamps s to exactly w visible characters: truncates if wider, pads if shorter.
func padLine(s string, w int) string {
	vis := lipgloss.Width(s)
	switch {
	case vis > w:
		return ansi.Truncate(s, w, "")
	case vis < w:
		return s + strings.Repeat(" ", w-vis)
	default:
		return s
	}
}

func (m model) renderContent() string {
	var sb strings.Builder
	matchSet := make(map[int]bool, len(m.searchMatches))
	for _, idx := range m.searchMatches {
		matchSet[idx] = true
	}

	for i, ln := range m.lines {
		s := ln.base

		availW := m.width - 2

		switch ln.kind {
		case kindAdd, kindRemove:
			bg := addBg
			if ln.kind == kindRemove {
				bg = delBg
			}
			isCursor := i == m.cursor
			if isCursor {
				bg = addBgCursor
				if ln.kind == kindRemove {
					bg = delBgCursor
				}
				if matchSet[i] {
					s = injectMatchHighlight(ln, m.searchQuery)
				}
			}
			rows := m.wrapRows(s, availW)
			if rows == nil {
				rows = []string{padLine(s, availW)}
			}
			for ri, row := range rows {
				if ri > 0 {
					sb.WriteString("\x1b[0m\n")
					if isCursor {
						sb.WriteString(sCursorBar.Render("▌"))
					} else {
						sb.WriteByte(' ')
					}
				} else {
					if isCursor {
						sb.WriteString(sCursorBar.Render("▌"))
					} else if matchSet[i] {
						sb.WriteString(sMatchBar.Render("▌"))
					} else {
						sb.WriteByte(' ')
					}
				}
				sb.WriteString(withBg(row, bg))
				sb.WriteString("\x1b[0m")
			}

		default:
			isCursor := i == m.cursor
			if isCursor && matchSet[i] {
				s = injectMatchHighlight(ln, m.searchQuery)
			}
			// Only context lines wrap; headers/hunks/comments truncate.
			var rows []string
			if ln.kind == kindContext {
				rows = m.wrapRows(s, availW)
			} else {
				s = padLine(s, availW)
			}
			if isCursor {
				if rows == nil {
					rows = []string{padLine(s, availW)}
				}
				for ri, row := range rows {
					if ri > 0 {
						sb.WriteByte('\n')
					}
					sb.WriteString(sCursorBar.Render("▌"))
					sb.WriteString(sCursor.Render(row))
				}
			} else if matchSet[i] {
				if rows == nil {
					sb.WriteString(sMatchBar.Render("▌"))
					sb.WriteString(s)
				} else {
					for ri, row := range rows {
						if ri > 0 {
							sb.WriteString("\n ")
						} else {
							sb.WriteString(sMatchBar.Render("▌"))
						}
						sb.WriteString(row)
					}
				}
			} else {
				if rows == nil {
					sb.WriteByte(' ')
					sb.WriteString(s)
				} else {
					for ri, row := range rows {
						if ri > 0 {
							sb.WriteByte('\n')
						}
						sb.WriteByte(' ')
						sb.WriteString(row)
					}
				}
			}
		}
		sb.WriteByte('\n')
	}
	return sb.String()
}

func buildLines(files []diff.FileDiff) ([]uiLine, []fileEntry, int) {
	// Compute the minimum column width needed to display any line number.
	maxNo := 1
	for _, f := range files {
		for _, h := range f.Hunks {
			for _, dl := range h.Lines {
				if dl.OldNo > maxNo {
					maxNo = dl.OldNo
				}
				if dl.NewNo > maxNo {
					maxNo = dl.NewNo
				}
			}
		}
	}
	lineNoW := len(fmt.Sprintf("%d", maxNo))

	var lines []uiLine
	var fileStarts []fileEntry

	for _, f := range files {
		path := f.NewPath
		if path == "/dev/null" {
			path = f.OldPath
		}

		adds, dels := 0, 0
		for _, h := range f.Hunks {
			for _, dl := range h.Lines {
				switch dl.Type {
				case "add":
					adds++
				case "remove":
					dels++
				}
			}
		}

		headerLine := fmt.Sprintf("  %s  %s  %s",
			sFile.Render(path),
			sAdd.Render(fmt.Sprintf("+%d", adds)),
			sDel.Render(fmt.Sprintf("-%d", dels)),
		)
		sep := sHunk.Render(strings.Repeat("─", max(0, 60-lipgloss.Width(headerLine))))

		// Blank spacer before each file is its own uiLine so every entry
		// is exactly 1 visual line, keeping cursor index == visual line index.
		lines = append(lines, uiLine{kind: kindHunk, base: "", file: path, commentIdx: -1})

		fileStarts = append(fileStarts, fileEntry{
			path:    path,
			lineIdx: len(lines), // points at the header line below
			adds:    adds,
			dels:    dels,
		})

		lines = append(lines, uiLine{
			kind:       kindFileHeader,
			base:       headerLine + " " + sep,
			text:       path,
			file:       path,
			commentIdx: -1,
		})

		// Collect all content lines for this file to highlight them together.
		// Expand tabs to spaces so lipgloss.Width correctly measures visible width.
		var contents []string
		for _, h := range f.Hunks {
			for _, dl := range h.Lines {
				contents = append(contents, expandTabs(dl.Content, 4))
			}
		}
		highlighted := highlightLines(path, contents)

		idx := 0
		for _, h := range f.Hunks {
			lines = append(lines, uiLine{
				kind:       kindHunk,
				base:       sHunk.Render("  " + h.Header),
				text:       h.Header,
				file:       path,
				commentIdx: -1,
			})
			for _, dl := range h.Lines {
				expanded := dl
				expanded.Content = contents[idx]
				l := buildDiffLine(expanded, path, highlighted[idx], lineNoW)
				idx++
				l.commentIdx = -1
				lines = append(lines, l)
			}
		}
	}
	return lines, fileStarts, lineNoW
}

func buildDiffLine(dl diff.DiffLine, file, content string, w int) uiLine {
	old := sLineNo.Render(fmtNo(dl.OldNo, w))
	neu := sLineNo.Render(fmtNo(dl.NewNo, w))

	switch dl.Type {
	case "add":
		gutter := sAdd.Render(neu) + sLineNo.Render(" "+fmtNo(0, w)) + sAdd.Render(" + ")
		return uiLine{
			kind:   kindAdd,
			gutter: fmtNo(dl.NewNo, w) + " " + fmtNo(0, w) + " + ",
			base:   gutter + content,
			text:   dl.Content,
			file:   file,
			newNo:  dl.NewNo,
		}
	case "remove":
		gutter := sLineNo.Render(fmtNo(0, w)) + sDel.Render(" "+old) + sDel.Render(" - ")
		return uiLine{
			kind:   kindRemove,
			gutter: fmtNo(0, w) + " " + fmtNo(dl.OldNo, w) + " - ",
			base:   gutter + content,
			text:   dl.Content,
			file:   file,
			oldNo:  dl.OldNo,
		}
	default:
		gutter := neu + " " + old + "   "
		return uiLine{
			kind:   kindContext,
			gutter: gutter,
			base:   gutter + content,
			text:   dl.Content,
			file:   file,
			oldNo:  dl.OldNo,
			newNo:  dl.NewNo,
		}
	}
}

func fuzzyFilter(files []fileEntry, query string) []int {
	query = strings.ToLower(strings.TrimSpace(query))
	out := make([]int, 0, len(files))
	for i, fe := range files {
		if query == "" || strings.Contains(strings.ToLower(fe.path), query) {
			out = append(out, i)
		}
	}
	return out
}

func allFileIndices(n int) []int {
	out := make([]int, n)
	for i := range out {
		out[i] = i
	}
	return out
}

func findMatches(lines []uiLine, query string) []int {
	if query == "" {
		return nil
	}
	q := strings.ToLower(query)
	var out []int
	for i, ln := range lines {
		if ln.kind == kindComment || ln.kind == kindFileHeader || ln.kind == kindHunk {
			continue
		}
		if strings.Contains(strings.ToLower(ln.text), q) {
			out = append(out, i)
		}
	}
	return out
}

// withBg injects a true-color background escape after every ANSI reset in s,
// so syntax-highlighted (or otherwise styled) content renders on a persistent
// tinted background regardless of embedded \x1b[0m reset sequences.
func withBg(s, hex string) string {
	hex = strings.TrimPrefix(hex, "#")
	r, _ := strconv.ParseUint(hex[0:2], 16, 8)
	g, _ := strconv.ParseUint(hex[2:4], 16, 8)
	b, _ := strconv.ParseUint(hex[4:6], 16, 8)
	bg := fmt.Sprintf("\x1b[48;2;%d;%d;%dm", r, g, b)
	const rst = "\x1b[0m"
	out := bg + strings.ReplaceAll(s, rst, rst+bg)
	// The final rst+bg at end of string is pointless, strip the trailing bg.
	if strings.HasSuffix(out, rst+bg) {
		out = out[:len(out)-len(bg)]
	}
	return out
}

func renderCommentDisplay(c diff.Comment) string {
	loc := sComment.Faint(true).Render(fmt.Sprintf(" line %d (%s) ", c.Line, c.Side))
	body := sComment.Render("  💬 " + c.Body)
	return loc + body
}

func fmtNo(n, w int) string {
	if n == 0 {
		return strings.Repeat(" ", w)
	}
	s := fmt.Sprintf("%d", n)
	if len(s) >= w {
		return s
	}
	return strings.Repeat(" ", w-len(s)) + s
}

// injectMatchHighlight rebuilds a line with the first occurrence of query
// highlighted. The styled gutter is preserved; content is rebuilt as plain
// text so that withBg can apply the add/remove background over it and the
// yellow match highlight stands out cleanly on top.
func injectMatchHighlight(ln uiLine, query string) string {
	if query == "" {
		return ln.base
	}
	q := strings.ToLower(query)
	text := strings.ToLower(ln.text)
	idx := strings.Index(text, q)
	if idx == -1 {
		return ln.base
	}
	// Preserve the styled gutter; rebuild content as plain text + highlight.
	gutterAnsi := ansi.Truncate(ln.base, lipgloss.Width(ln.gutter), "")
	before := ln.text[:idx]
	match := ln.text[idx : idx+len(query)]
	after := ln.text[idx+len(query):]
	return gutterAnsi + before + sSearchHighlight.Render(match) + after
}

// expandTabs replaces tab characters with spaces aligned to n-space tab stops.
// This prevents lipgloss.Width from underestimating visible width (which counts
// \t as 1 cell while the terminal expands it to the next tab stop).
func expandTabs(s string, n int) string {
	if !strings.ContainsRune(s, '\t') {
		return s
	}
	var b strings.Builder
	col := 0
	for _, r := range s {
		if r == '\t' {
			spaces := n - (col % n)
			b.WriteString(strings.Repeat(" ", spaces))
			col += spaces
		} else {
			b.WriteRune(r)
			col++
		}
	}
	return b.String()
}

func sideAndLine(ln uiLine) (side string, lineNo int) {
	if ln.kind == kindRemove {
		return "old", ln.oldNo
	}
	return "new", ln.newNo
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

package app

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/spinner"
	"charm.land/bubbles/v2/table"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"lanscanner/internal/network"
	"lanscanner/internal/scanner"
	"lanscanner/internal/ui"
)

type focusTarget int

const (
	focusInput focusTarget = iota
	focusTable
)

type scanEventMsg struct {
	event  scanner.Event
	closed bool
}

type model struct {
	engine *scanner.Engine

	styles  ui.Styles
	keys    keyMap
	help    help.Model
	input   textinput.Model
	table   table.Model
	spinner spinner.Model

	subnets    []network.InterfaceSubnet
	results    []scanner.Host
	progress   scanner.Progress
	events     <-chan scanner.Event
	cancelScan context.CancelFunc

	width   int
	height  int
	focus   focusTarget
	showAll bool

	status  string
	lastErr string

	scanning bool
}

func NewModel(engine *scanner.Engine) model {
	styles := ui.DefaultStyles()
	keys := defaultKeyMap()

	input := textinput.New()
	input.Prompt = "CIDR> "
	input.Placeholder = "192.168.1.0/24"
	input.SetWidth(28)

	tableModel := ui.NewResultsTable()
	tableModel.Focus()

	spin := spinner.New(spinner.WithSpinner(spinner.Line))

	helpModel := help.New()

	m := model{
		engine:  engine,
		styles:  styles,
		keys:    keys,
		help:    helpModel,
		input:   input,
		table:   tableModel,
		spinner: spin,
		width:   110,
		height:  32,
		focus:   focusInput,
		status:  "Enter a CIDR and press Enter to scan.",
	}

	subnets, err := network.DiscoverSubnets()
	if err == nil {
		m.subnets = subnets
		if len(subnets) > 0 {
			m.input.SetValue(subnets[0].CIDR())
			m.input.SetSuggestions(subnetSuggestions(subnets))
			m.input.ShowSuggestions = true
			m.status = "Default subnet loaded from the first active interface."
		}
	} else {
		m.lastErr = err.Error()
	}

	m.table.Blur()
	return m
}

func (m model) Init() tea.Cmd {
	return m.input.Focus()
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	if m.scanning {
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		cmds = append(cmds, cmd)
	}

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.help.SetWidth(msg.Width - 4)
		m.resizeTable()
		return m, tea.Batch(cmds...)

	case tea.KeyPressMsg:
		switch {
		case key.Matches(msg, m.keys.quit):
			if m.cancelScan != nil {
				m.cancelScan()
			}
			return m, tea.Quit
		case key.Matches(msg, m.keys.help):
			m.showAll = !m.showAll
			m.help.ShowAll = m.showAll
			return m, tea.Batch(cmds...)
		case key.Matches(msg, m.keys.switchPane):
			cmds = append(cmds, m.toggleFocus())
			return m, tea.Batch(cmds...)
		case key.Matches(msg, m.keys.clear) && !m.scanning:
			m.results = nil
			m.progress = scanner.Progress{}
			m.status = "Results cleared."
			m.lastErr = ""
			m.syncRows()
			return m, tea.Batch(cmds...)
		case key.Matches(msg, m.keys.stop) && m.scanning:
			if m.cancelScan != nil {
				m.cancelScan()
			}
			m.status = "Stopping scan..."
			return m, tea.Batch(cmds...)
		case key.Matches(msg, m.keys.start) && !m.scanning:
			startCmd, err := m.startScan()
			if err != nil {
				m.lastErr = err.Error()
				return m, tea.Batch(cmds...)
			}
			cmds = append(cmds, startCmd, m.spinner.Tick)
			return m, tea.Batch(cmds...)
		}
	case scanEventMsg:
		if msg.closed {
			m.finishScan(nil)
			return m, tea.Batch(cmds...)
		}

		switch msg.event.Type {
		case scanner.EventStarted:
			m.progress = msg.event.Progress
			m.status = fmt.Sprintf("Scanning %s.", msg.event.Progress.Subnet)
			cmds = append(cmds, waitForScanEvent(m.events))
		case scanner.EventProgress:
			m.progress = msg.event.Progress
			cmds = append(cmds, waitForScanEvent(m.events))
		case scanner.EventHostFound:
			m.progress = msg.event.Progress
			m.results = append(m.results, msg.event.Host)
			m.status = fmt.Sprintf(
				"Found %d host(s) in %s.",
				m.progress.Alive,
				m.progress.Subnet,
			)
			m.syncRows()
			cmds = append(cmds, waitForScanEvent(m.events))
		case scanner.EventError:
			m.lastErr = msg.event.Err.Error()
			cmds = append(cmds, waitForScanEvent(m.events))
		case scanner.EventDone:
			m.progress = msg.event.Progress
			m.finishScan(msg.event.Err)
		}
		return m, tea.Batch(cmds...)
	}

	if m.focus == focusInput {
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		cmds = append(cmds, cmd)
	}

	if m.focus == focusTable {
		var cmd tea.Cmd
		m.table, cmd = m.table.Update(msg)
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

func (m model) View() tea.View {
	header := m.renderHeader()
	inputSection := m.renderInput()
	statusSection := m.renderStatus()
	tableSection := m.renderTable()
	detailSection := m.renderDetail()
	helpSection := m.help.View(m.keys)

	content := lipgloss.JoinVertical(
		lipgloss.Left,
		header,
		inputSection,
		statusSection,
		tableSection,
		detailSection,
		m.styles.Muted.Render(helpSection),
	)

	view := tea.NewView(content)
	view.AltScreen = true
	view.WindowTitle = "LAN Scanner"
	return view
}

func (m *model) startScan() (tea.Cmd, error) {
	prefix, err := network.ParsePrefix(m.input.Value())
	if err != nil {
		return nil, err
	}

	scanCtx, cancel := context.WithCancel(context.Background())
	events, err := m.engine.Scan(scanCtx, prefix)
	if err != nil {
		cancel()
		return nil, err
	}

	m.cancelScan = cancel
	m.events = events
	m.scanning = true
	m.progress = scanner.Progress{}
	m.results = nil
	m.lastErr = ""
	m.status = "Starting scan..."
	m.syncRows()

	return waitForScanEvent(m.events), nil
}

func waitForScanEvent(events <-chan scanner.Event) tea.Cmd {
	return func() tea.Msg {
		event, ok := <-events
		if !ok {
			return scanEventMsg{closed: true}
		}
		return scanEventMsg{event: event}
	}
}

func (m *model) finishScan(err error) {
	m.scanning = false
	m.events = nil
	if m.cancelScan != nil {
		m.cancelScan()
		m.cancelScan = nil
	}

	switch {
	case err == nil:
		m.status = fmt.Sprintf(
			"Scan finished. %d host(s) found out of %d targets.",
			m.progress.Alive,
			m.progress.Total,
		)
	case errors.Is(err, context.Canceled):
		m.status = fmt.Sprintf(
			"Scan stopped. %d/%d targets checked.",
			m.progress.Completed,
			m.progress.Total,
		)
	default:
		m.status = "Scan stopped with an error."
		m.lastErr = err.Error()
	}
}

func (m *model) toggleFocus() tea.Cmd {
	switch m.focus {
	case focusInput:
		m.focus = focusTable
		m.input.Blur()
		m.table.Focus()
		m.status = "Table focused. Use j/k or arrows to move."
		return nil
	default:
		m.focus = focusInput
		m.table.Blur()
		m.status = "Input focused. Edit the CIDR and press Enter to scan."
		return m.input.Focus()
	}
}

func (m *model) resizeTable() {
	tableWidth := m.width - 6
	tableHeight := m.height - 19
	ui.ResizeResultsTable(&m.table, tableWidth, tableHeight)
}

func (m *model) syncRows() {
	m.table.SetRows(ui.HostRows(m.results))
}

func (m model) renderHeader() string {
	title := m.styles.Title.Render("LAN Scanner")
	subtitle := m.styles.Subtitle.Render(
		"Bubble Tea powered host discovery for local IPv4 networks",
	)
	return lipgloss.JoinVertical(lipgloss.Left, title, subtitle)
}

func (m model) renderInput() string {
	label := m.styles.Section.Render("Target subnet")
	hints := "No active IPv4 subnet detected."
	if len(m.subnets) > 0 {
		hints = "Detected: " + strings.Join(subnetDescriptions(m.subnets), ", ")
	}

	boxStyle := m.styles.Box
	if m.focus == focusInput {
		boxStyle = m.styles.FocusedBox
	}

	content := lipgloss.JoinVertical(
		lipgloss.Left,
		label,
		m.input.View(),
		m.styles.Muted.Render(hints),
	)
	return boxStyle.Width(m.width - 4).Render(content)
}

func (m model) renderStatus() string {
	label := m.styles.Section.Render("Status")

	statusLine := m.status
	if m.scanning {
		statusLine = fmt.Sprintf(
			"%s scanning %s | %d/%d checked | %d alive | last %s",
			m.spinner.View(),
			blankFallback(m.progress.Subnet),
			m.progress.Completed,
			m.progress.Total,
			m.progress.Alive,
			blankFallback(m.progress.Current),
		)
	}

	statusStyle := m.styles.StatusOK
	if m.scanning {
		statusStyle = m.styles.StatusWarn
	}
	if m.lastErr != "" {
		statusStyle = m.styles.StatusErr
	}

	lines := []string{label, statusStyle.Render(statusLine)}
	if m.lastErr != "" {
		lines = append(lines, m.styles.StatusErr.Render("error: "+m.lastErr))
	}

	return m.styles.Box.Width(m.width - 4).Render(
		lipgloss.JoinVertical(lipgloss.Left, lines...),
	)
}

func (m model) renderTable() string {
	label := m.styles.Section.Render("Discovered hosts")
	tableView := m.table.View()
	if len(m.results) == 0 {
		tableView = m.styles.Muted.Render("No hosts discovered yet.")
	}

	boxStyle := m.styles.Box
	if m.focus == focusTable {
		boxStyle = m.styles.FocusedBox
	}

	return boxStyle.Width(m.width - 4).Render(
		lipgloss.JoinVertical(lipgloss.Left, label, tableView),
	)
}

func (m model) renderDetail() string {
	label := m.styles.Section.Render("Host details")

	details := []string{m.styles.Muted.Render("Select a discovered host to inspect it.")}
	if len(m.results) > 0 {
		index := m.table.Cursor()
		if index >= 0 && index < len(m.results) {
			host := m.results[index]
			details = styleDetails(m.styles, ui.DetailLines(host))
		}
	}

	return m.styles.Box.Width(m.width - 4).Render(
		lipgloss.JoinVertical(lipgloss.Left, append([]string{label}, details...)...),
	)
}

func styleDetails(styles ui.Styles, lines []string) []string {
	rendered := make([]string, 0, len(lines))
	for _, line := range lines {
		parts := strings.SplitN(line, ": ", 2)
		if len(parts) != 2 {
			rendered = append(rendered, line)
			continue
		}
		rendered = append(
			rendered,
			styles.DetailKey.Render(parts[0]+":")+" "+styles.DetailVal.Render(parts[1]),
		)
	}
	return rendered
}

func subnetSuggestions(subnets []network.InterfaceSubnet) []string {
	suggestions := make([]string, 0, len(subnets))
	for _, subnet := range subnets {
		suggestions = append(suggestions, subnet.CIDR())
	}
	return suggestions
}

func subnetDescriptions(subnets []network.InterfaceSubnet) []string {
	descriptions := make([]string, 0, len(subnets))
	for _, subnet := range subnets {
		descriptions = append(descriptions, fmt.Sprintf("%s %s", subnet.Name, subnet.CIDR()))
	}
	return descriptions
}

func blankFallback(value string) string {
	if value == "" {
		return "-"
	}
	return value
}

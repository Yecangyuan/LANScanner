package app

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

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
	history    scanner.HistoryStore
	lastDiff   scanner.SnapshotDiff

	width   int
	height  int
	focus   focusTarget
	showAll bool

	status    string
	lastErr   string
	exportDir string

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
	if cwd, err := os.Getwd(); err == nil {
		m.exportDir = cwd
	} else {
		m.exportDir = "."
	}
	m.history = scanner.NewHistoryStore(filepath.Join(m.exportDir, ".lanscanner-history"))

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
			m.lastDiff = scanner.SnapshotDiff{}
			m.status = "Results cleared."
			m.lastErr = ""
			m.syncRows()
			return m, tea.Batch(cmds...)
		case key.Matches(msg, m.keys.exportCSV) && !m.scanning:
			if err := m.exportResults("csv"); err != nil {
				m.lastErr = err.Error()
			}
			return m, tea.Batch(cmds...)
		case key.Matches(msg, m.keys.exportJSON) && !m.scanning:
			if err := m.exportResults("json"); err != nil {
				m.lastErr = err.Error()
			}
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
		m.renderChanges(),
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
	m.lastDiff = scanner.SnapshotDiff{}
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
		m.persistHistory()
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
	tableHeight := m.height - 26
	ui.ResizeResultsTable(&m.table, tableWidth, tableHeight)
}

func (m *model) syncRows() {
	m.table.SetRows(ui.HostRows(m.results))
}

func (m model) renderHeader() string {
	title := m.styles.Title.Render("LAN Scanner")
	subtitle := m.styles.Subtitle.Render(
		"Bubble Tea powered LAN scanner with common-port scanning and export",
	)
	return lipgloss.JoinVertical(lipgloss.Left, title, subtitle)
}

func (m model) renderInput() string {
	label := m.styles.Section.Render("Target subnet")
	hints := "No active IPv4 subnet detected."
	if len(m.subnets) > 0 {
		hints = "Detected: " + strings.Join(subnetDescriptions(m.subnets), ", ")
	}
	hints += " | common ports: " + strings.Join(defaultPortLabels(), ", ")

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

func (m model) renderChanges() string {
	label := m.styles.Section.Render("Device changes")

	var lines []string
	switch {
	case m.scanning:
		lines = []string{m.styles.Muted.Render("Changes will be computed when the scan completes.")}
	case !m.lastDiff.HasPrevious:
		lines = []string{m.styles.Muted.Render(m.lastDiff.Summary())}
	default:
		lines = []string{m.styles.StatusOK.Render(m.lastDiff.Summary())}
		lines = append(lines, renderHostList(m.styles, "New", m.lastDiff.Added, m.styles.StatusOK)...)
		lines = append(lines, renderHostList(m.styles, "Offline", m.lastDiff.Removed, m.styles.StatusErr)...)
	}

	return m.styles.Box.Width(m.width - 4).Render(
		lipgloss.JoinVertical(lipgloss.Left, append([]string{label}, lines...)...),
	)
}

func (m model) renderTable() string {
	label := m.styles.Section.Render("Discovered hosts and ports")
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

func (m *model) exportResults(format string) error {
	if len(m.results) == 0 {
		return fmt.Errorf("there are no results to export")
	}

	snapshot := scanner.NewSnapshot(m.progress, m.results)
	filename := filepath.Join(
		m.exportDir,
		fmt.Sprintf("lanscanner-%s.%s", time.Now().Format("20060102-150405"), format),
	)

	var err error
	switch format {
	case "csv":
		err = scanner.ExportCSV(filename, snapshot)
	case "json":
		err = scanner.ExportJSON(filename, snapshot)
	default:
		return fmt.Errorf("unsupported export format: %s", format)
	}
	if err != nil {
		return err
	}

	m.lastErr = ""
	m.status = fmt.Sprintf("Exported %d host(s) to %s.", len(m.results), filename)
	return nil
}

func defaultPortLabels() []string {
	ports := scanner.DefaultPortTargets()
	labels := make([]string, 0, len(ports))
	for _, port := range ports {
		switch port {
		case 22, 80, 443, 445, 3389, 8080:
			labels = append(labels, fmt.Sprintf("%d", port))
		}
	}
	return labels
}

func (m *model) persistHistory() {
	snapshot := scanner.NewSnapshot(m.progress, m.results)

	previous, ok, err := m.history.LoadLatest(snapshot.Subnet)
	if err != nil {
		m.lastErr = fmt.Sprintf("load history: %v", err)
		m.lastDiff = scanner.SnapshotDiff{}
	} else if ok {
		m.lastDiff = scanner.CompareSnapshots(previous, snapshot)
	} else {
		m.lastDiff = scanner.SnapshotDiff{}
	}

	if err := m.history.Save(snapshot); err != nil {
		m.lastErr = fmt.Sprintf("save history: %v", err)
	}
}

func renderHostList(styles ui.Styles, label string, hosts []scanner.Host, lineStyle lipgloss.Style) []string {
	if len(hosts) == 0 {
		return []string{styles.Muted.Render(label + ": none")}
	}

	limit := len(hosts)
	if limit > 4 {
		limit = 4
	}

	values := make([]string, 0, limit+1)
	for _, host := range hosts[:limit] {
		values = append(values, host.IP.String())
	}
	if len(hosts) > limit {
		values = append(values, fmt.Sprintf("+%d more", len(hosts)-limit))
	}

	return []string{lineStyle.Render(fmt.Sprintf("%s: %s", label, strings.Join(values, ", ")))}
}

func blankFallback(value string) string {
	if value == "" {
		return "-"
	}
	return value
}

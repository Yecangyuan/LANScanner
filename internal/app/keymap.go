package app

import "charm.land/bubbles/v2/key"

type keyMap struct {
	start      key.Binding
	stop       key.Binding
	switchPane key.Binding
	clear      key.Binding
	exportCSV  key.Binding
	exportJSON key.Binding
	help       key.Binding
	quit       key.Binding
}

func defaultKeyMap() keyMap {
	return keyMap{
		start: key.NewBinding(
			key.WithKeys("enter", "s"),
			key.WithHelp("enter/s", "start scan"),
		),
		stop: key.NewBinding(
			key.WithKeys("esc", "x"),
			key.WithHelp("esc/x", "stop scan"),
		),
		switchPane: key.NewBinding(
			key.WithKeys("tab"),
			key.WithHelp("tab", "switch focus"),
		),
		clear: key.NewBinding(
			key.WithKeys("c"),
			key.WithHelp("c", "clear results"),
		),
		exportCSV: key.NewBinding(
			key.WithKeys("e"),
			key.WithHelp("e", "export csv"),
		),
		exportJSON: key.NewBinding(
			key.WithKeys("E"),
			key.WithHelp("E", "export json"),
		),
		help: key.NewBinding(
			key.WithKeys("?"),
			key.WithHelp("?", "toggle help"),
		),
		quit: key.NewBinding(
			key.WithKeys("q", "ctrl+c"),
			key.WithHelp("q", "quit"),
		),
	}
}

func (k keyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.start, k.stop, k.exportCSV, k.quit}
}

func (k keyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.start, k.stop, k.switchPane},
		{k.clear, k.exportCSV, k.exportJSON},
		{k.help, k.quit},
	}
}

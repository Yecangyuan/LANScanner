package ui

import (
	"charm.land/bubbles/v2/table"
	"charm.land/lipgloss/v2"
)

type Styles struct {
	Title      lipgloss.Style
	Subtitle   lipgloss.Style
	Section    lipgloss.Style
	Box        lipgloss.Style
	FocusedBox lipgloss.Style
	StatusOK   lipgloss.Style
	StatusWarn lipgloss.Style
	StatusErr  lipgloss.Style
	Muted      lipgloss.Style
	DetailKey  lipgloss.Style
	DetailVal  lipgloss.Style
}

func DefaultStyles() Styles {
	border := lipgloss.RoundedBorder()

	return Styles{
		Title:      lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("63")),
		Subtitle:   lipgloss.NewStyle().Foreground(lipgloss.Color("245")),
		Section:    lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("110")),
		Box:        lipgloss.NewStyle().Border(border).BorderForeground(lipgloss.Color("238")).Padding(0, 1),
		FocusedBox: lipgloss.NewStyle().Border(border).BorderForeground(lipgloss.Color("63")).Padding(0, 1),
		StatusOK:   lipgloss.NewStyle().Foreground(lipgloss.Color("42")).Bold(true),
		StatusWarn: lipgloss.NewStyle().Foreground(lipgloss.Color("214")).Bold(true),
		StatusErr:  lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Bold(true),
		Muted:      lipgloss.NewStyle().Foreground(lipgloss.Color("245")),
		DetailKey:  lipgloss.NewStyle().Foreground(lipgloss.Color("110")).Bold(true),
		DetailVal:  lipgloss.NewStyle().Foreground(lipgloss.Color("252")),
	}
}

func TableStyles() table.Styles {
	styles := table.DefaultStyles()
	styles.Header = styles.Header.
		BorderStyle(lipgloss.NormalBorder()).
		BorderBottom(true).
		BorderForeground(lipgloss.Color("240")).
		Bold(true)
	styles.Selected = styles.Selected.
		Foreground(lipgloss.Color("230")).
		Background(lipgloss.Color("63")).
		Bold(true)
	return styles
}

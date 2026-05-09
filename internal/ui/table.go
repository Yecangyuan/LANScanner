package ui

import (
	"fmt"

	"charm.land/bubbles/v2/table"

	"lanscanner/internal/scanner"
)

func NewResultsTable() table.Model {
	model := table.New(
		table.WithColumns(defaultColumns(90)),
		table.WithHeight(12),
		table.WithFocused(true),
	)
	model.SetStyles(TableStyles())
	return model
}

func ResizeResultsTable(model *table.Model, width, height int) {
	if width < 40 {
		width = 40
	}
	if height < 6 {
		height = 6
	}

	model.SetColumns(defaultColumns(width))
	model.SetWidth(width)
	model.SetHeight(height)
}

func HostRows(hosts []scanner.Host) []table.Row {
	rows := make([]table.Row, 0, len(hosts))
	for _, host := range hosts {
		rows = append(rows, table.Row{
			host.IP.String(),
			blankFallback(host.Hostname),
			blankFallback(host.MAC),
			blankFallback(host.Source),
			host.DetectedAt.Format("15:04:05"),
		})
	}
	return rows
}

func DetailLines(host scanner.Host) []string {
	return []string{
		fmt.Sprintf("IP: %s", host.IP.String()),
		fmt.Sprintf("Hostname: %s", blankFallback(host.Hostname)),
		fmt.Sprintf("MAC: %s", blankFallback(host.MAC)),
		fmt.Sprintf("Vendor: %s", blankFallback(host.Vendor)),
		fmt.Sprintf("Source: %s", blankFallback(host.Source)),
		fmt.Sprintf("Seen: %s", host.DetectedAt.Format("2006-01-02 15:04:05")),
	}
}

func blankFallback(value string) string {
	if value == "" {
		return "-"
	}
	return value
}

func defaultColumns(width int) []table.Column {
	ipWidth := 16
	macWidth := 18
	sourceWidth := 10
	seenWidth := 10
	hostnameWidth := width - ipWidth - macWidth - sourceWidth - seenWidth - 10
	if hostnameWidth < 18 {
		hostnameWidth = 18
	}

	return []table.Column{
		{Title: "IP", Width: ipWidth},
		{Title: "Hostname", Width: hostnameWidth},
		{Title: "MAC", Width: macWidth},
		{Title: "Source", Width: sourceWidth},
		{Title: "Seen", Width: seenWidth},
	}
}

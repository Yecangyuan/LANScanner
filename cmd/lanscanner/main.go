package main

import (
	"fmt"
	"os"
	"time"

	tea "charm.land/bubbletea/v2"

	"lanscanner/internal/app"
	"lanscanner/internal/scanner"
)

func main() {
	engine := scanner.NewEngine(
		scanner.NewPingProber(1200*time.Millisecond, 250*time.Millisecond),
		scanner.WithConcurrency(64),
	)

	program := tea.NewProgram(app.NewModel(engine))
	if _, err := program.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "lan scanner failed: %v\n", err)
		os.Exit(1)
	}
}

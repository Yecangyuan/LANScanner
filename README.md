# LAN Scanner

Bubble Tea based LAN scanner for discovering live IPv4 hosts from the terminal.

## What v1 does

- Detects active IPv4 subnets from local interfaces and pre-fills the first one
- Lets you enter a CIDR manually
- Starts and stops host discovery scans from a terminal UI
- Streams scan progress into the Bubble Tea event loop
- Shows discovered hosts in a table with IP, hostname, MAC and discovery source

## Run

```bash
go run ./cmd/lanscanner
```

## Controls

- `Enter` or `s`: start scan
- `Esc` or `x`: stop scan
- `Tab`: switch focus between subnet input and results table
- `j` / `k` or arrow keys: move inside the results table
- `c`: clear results
- `?`: toggle extended help
- `q`: quit

## Layout

- `cmd/lanscanner`: application entrypoint
- `internal/network`: CIDR parsing, host enumeration and interface discovery
- `internal/scanner`: scan engine, probe abstraction and ping-based host probing
- `internal/app`: Bubble Tea model, update loop and view composition
- `internal/ui`: table setup and shared Lip Gloss styles

## Notes

The first version focuses on host discovery only. Port scanning is intentionally
out of scope so the scanning engine and TUI stay simple and easy to extend.

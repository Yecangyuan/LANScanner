# LAN Scanner

Bubble Tea based LAN scanner for discovering live IPv4 hosts and common open
TCP ports from the terminal.

## What it does

- Detects active IPv4 subnets from local interfaces and pre-fills the first one
- Lets you enter a CIDR manually
- Starts and stops host discovery scans from a terminal UI
- Scans a built-in set of common TCP ports on each live host
- Streams scan progress into the Bubble Tea event loop
- Shows discovered hosts in a table with IP, hostname, open ports and discovery source
- Exports the current results as CSV or JSON snapshots
- Saves hidden per-subnet history snapshots and shows which devices are new or offline since the last scan

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
- `e`: export CSV
- `E`: export JSON
- `?`: toggle extended help
- `q`: quit

## Layout

- `cmd/lanscanner`: application entrypoint
- `internal/network`: CIDR parsing, host enumeration and interface discovery
- `internal/scanner`: scan engine, probe abstraction, TCP port scanning and export
- `internal/app`: Bubble Tea model, update loop and view composition
- `internal/ui`: table setup and shared Lip Gloss styles

## Export output

Exports are written to the current working directory as timestamped files:

- `lanscanner-YYYYMMDD-HHMMSS.csv`
- `lanscanner-YYYYMMDD-HHMMSS.json`

## History tracking

After each successful scan the app stores a hidden history snapshot under:

- `.lanscanner-history/`

The next scan of the same subnet compares the current result to the previous
snapshot and shows:

- newly seen devices
- devices that went offline
- unchanged devices

## Common ports

The default built-in TCP scan targets are:

`20, 21, 22, 23, 25, 53, 67, 68, 80, 110, 123, 135, 137, 138, 139, 143, 161, 389, 443, 445, 465, 587, 631, 993, 995, 1433, 1521, 1723, 1883, 2049, 2375, 3000, 3306, 3389, 5000, 5432, 5900, 6379, 8000, 8080, 8443, 9000`

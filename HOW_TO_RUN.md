# 🚨 Crowd Safety System — How to Run

A real-time crowd monitoring system built with C++, Rust, Go, and Redis.
It predicts overcrowding before it happens, warns about chain-reaction
risk to neighboring zones, and suggests safe evacuation zones — even
when internet connectivity is unreliable.

## Tech Stack & Roles

| Component | Role |
|---|---|
| **C++** | Calculates risk level + predicts minutes until a zone becomes dangerous |
| **Rust** | Safely ingests live crowd-count updates, queues them offline if Redis is down |
| **Go** | Serves the API, WebSocket live feed, nearest-safe-zone lookup, analytics |
| **Redis** | Stores zone data, geospatial locations, history, and live pub/sub updates |

## Prerequisites (one-time setup)

- WSL (Ubuntu) installed on Windows
- Redis installed inside Ubuntu: `sudo apt install redis-server -y`
- Go installed inside Ubuntu: `sudo apt install golang-go -y`
- Rust installed inside Ubuntu: `curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs | sh`
- g++ installed inside Ubuntu: `sudo apt install build-essential -y`

## One-Time Build Steps

```bash
# Compile the C++ engine
cd "/mnt/d/IMPORTANT/Projects/Crowd Safety System/cpp-risk-engine"
g++ main.cpp -o risk_engine

# Build the Rust ingestor
cd "/mnt/d/IMPORTANT/Projects/Crowd Safety System/rust-sensor-ingestor"
cargo build

# Set up the Go server
cd "/mnt/d/IMPORTANT/Projects/Crowd Safety System/go-api"
go mod init crowd-safety-api
go get github.com/redis/go-redis/v9
go get github.com/gorilla/websocket
```

## How to Run (every time)

You need **3 terminal tabs** open in Ubuntu, plus your browser.

### Tab 1 — Start Redis
```bash
sudo service redis-server start
redis-cli ping   # should print PONG
```

### Seed Zone Data (only needed once, or after a Redis restart wipes it)
```bash
redis-cli
HSET zone:A name "Main Gate" current_count 450 capacity 500 lat 23.1815 lng 75.7849 risk_level "Safe"
HSET zone:B name "Food Court" current_count 200 capacity 600 lat 23.1820 lng 75.7855 risk_level "Safe"
HSET zone:C name "Prayer Hall" current_count 380 capacity 400 lat 23.1810 lng 75.7860 risk_level "Safe"
HSET zone:D name "Parking Area" current_count 100 capacity 800 lat 23.1825 lng 75.7840 risk_level "Safe"
SET zone:A:neighbors "B,C"
SET zone:B:neighbors "A,D"
SET zone:C:neighbors "A"
SET zone:D:neighbors "B"
GEOADD zones_locations 75.7849 23.1815 "zone:A"
GEOADD zones_locations 75.7855 23.1820 "zone:B"
GEOADD zones_locations 75.7860 23.1810 "zone:C"
GEOADD zones_locations 75.7840 23.1825 "zone:D"
exit
```

### Tab 2 — Start the Go Server (leave running)
```bash
cd "/mnt/d/IMPORTANT/Projects/Crowd Safety System/go-api"
go run main.go
```
Should show: `🚀 Server starting on http://localhost:8080`
**Do not close or type in this tab again.**

### Browser — Open the Dashboard
Double-click `dashboard.html` in File Explorer.
You should see 4 zone cards, all marked "Safe."

### Tab 3 — Simulate Crowd Changes (the live demo)
```bash
cd "/mnt/d/IMPORTANT/Projects/Crowd Safety System/rust-sensor-ingestor"
cargo run -- A 470
cargo run -- C 395
```
Watch the dashboard update live — risk level, ETA to danger,
cascade warnings, and nearest safe zone all appear automatically.

## Troubleshooting

| Problem | Fix |
|---|---|
| `redis-cli` not found | You're in PowerShell, not Ubuntu — open the Ubuntu app instead |
| `go.mod not found` | Run `pwd` to check your folder, `cd` into `go-api` first |
| Dashboard shows nothing | Check Tab 2 (Go server) is still running without errors |
| Status doesn't update live | Make sure you're running `cargo run` in a **different, free** tab — not the one running the Go server |
| `Application Control policy has blocked this file` | This is a Windows-only C++/exe issue — always compile and run C++ inside Ubuntu, not PowerShell |

## Project Structure
```
crowd-safety-system/
├── cpp-risk-engine/
│   └── main.cpp
├── rust-sensor-ingestor/
│   ├── src/main.rs
│   └── Cargo.toml
├── go-api/
│   └── main.go
├── dashboard.html
└── HOW_TO_RUN.md
```
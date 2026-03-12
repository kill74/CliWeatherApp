# Weather8bit — 8-Bit World Weather CLI

A retro terminal weather app written in **Go** with zero external dependencies.  
Check the weather anywhere in the world — current conditions, 7-day forecast, hourly breakdown, and weekly stats — all rendered in glorious 8-bit ASCII style.

---

## Features

- **Current conditions** — temperature, feels-like, humidity, precipitation, pressure, visibility, wind speed & direction, cloud cover, UV index
- **7-day forecast** — ASCII art weather icons, daily highs/lows, rain probability, wind, humidity, UV, sunrise & sunset
- **24-hour hourly table** — full breakdown per hour with condition, temp, humidity, precip chance, rain, wind, and visibility
- **Weekly statistics** — min, max, average, and totals for temperature, precipitation, wind, and UV
- **Pixel font numbers** — temperature displayed in chunky ▓ block digits
- **Colour-coded output** — temperatures, UV levels, and conditions all colour-coded for instant reading
- **Works everywhere** — single binary, no runtime, no installer, no API key needed

---

## Download

Pick the binary for your system:

| File | System |
|------|--------|
| `weather8bit-linux-x64` | Linux — most PCs and servers (x86_64) |
| `weather8bit-linux-arm64` | Linux ARM — Raspberry Pi, cloud VMs |
| `weather8bit-macos-arm64` | macOS — Apple Silicon (M1 / M2 / M3 / M4) |
| `weather8bit-macos-intel` | macOS — Intel chip |
| `weather8bit-windows-x64.exe` | Windows 10 / 11 (64-bit) |

---

## Quick Start

### Linux / macOS

```bash
# Make it executable (one time only)
chmod +x weather8bit-linux-x64

# Run it
./weather8bit-linux-x64 "Lisbon"
./weather8bit-linux-x64 "Tokyo" --all
./weather8bit-linux-x64 "New York" --hourly
./weather8bit-linux-x64 "48.8566,2.3522"
```

### Windows

Open **Command Prompt** or **PowerShell** and run:

```bat
weather8bit-windows-x64.exe "London"
weather8bit-windows-x64.exe "Tokyo" --all
```

### Optional: install globally

```bash
# Linux / macOS — move to PATH so you can run it from anywhere
sudo mv weather8bit-linux-x64 /usr/local/bin/weather8bit

# Then just:
weather8bit "Porto"
```

---

## Usage

```
weather8bit <location> [options]
```

### Arguments

| Argument | Description |
|----------|-------------|
| `"City Name"` | Search by city name (e.g. `"Berlin"`, `"São Paulo"`) |
| `"City, Country"` | Narrow the search (e.g. `"Springfield, US"`) |
| `"lat,lon"` | Use coordinates directly (e.g. `"38.7169,-9.1399"`) |

### Options

| Flag | Short | Description |
|------|-------|-------------|
| `--hourly` | `-H` | Show the 24-hour hourly forecast |
| `--stats` | `-s` | Show weekly statistics (min/max/avg/total) |
| `--all` | `-a` | Show everything — hourly + stats |
| `--help` | `-h` | Show usage information |

### Examples

```bash
weather8bit "Castelo Branco"
weather8bit "Tokyo" --all
weather8bit "New York, US" --hourly
weather8bit "Sydney" --stats
weather8bit "51.5074,-0.1278"          # London by coordinates
weather8bit                            # interactive mode — prompts for location
```

---

## Data Source

All weather data comes from **[Open-Meteo](https://open-meteo.com)** — a free, open-source weather API.

- Completely free
- No account needed
- No API key
- Covers every location on Earth
- Updates every hour

---

## Build from Source

You need **Go 1.22+** installed. The app has **zero external dependencies** — only the Go standard library.

```bash
# Clone or download weather8bit.go, then:
go build -o weather8bit weather8bit.go

# Run it
./weather8bit "Lisbon"
```

### Cross-compile for other platforms

```bash
# Windows from Linux/macOS
GOOS=windows GOARCH=amd64 go build -o weather8bit.exe weather8bit.go

# macOS Apple Silicon from Linux
GOOS=darwin GOARCH=arm64 go build -o weather8bit-mac weather8bit.go

# Linux ARM (Raspberry Pi)
GOOS=linux GOARCH=arm64 go build -o weather8bit-pi weather8bit.go
```

---

## Terminal Requirements

- Any terminal that supports **ANSI colour codes** (virtually all modern terminals)
- Recommended minimum width: **120 columns** (wider = better day panels)
- Font: any monospace font — the app uses block characters (▓, █, ░) that are part of standard Unicode

### Windows note

On Windows, use **Windows Terminal** or **PowerShell** for best results. The classic `cmd.exe` supports colours on Windows 10+ but may render some Unicode block characters with gaps depending on the font.

---

## Known Limitations

- Requires an internet connection to fetch weather data
- City name search depends on Open-Meteo's geocoding database — very obscure villages may not be found (use coordinates instead)
- UV index is only available during daylight hours from the API

---

## License

Free to use for personal and non-commercial purposes.  
Weather data © [Open-Meteo](https://open-meteo.com) — licensed under CC BY 4.0.

---

*Built with Go · Zero dependencies · Runs everywhere*

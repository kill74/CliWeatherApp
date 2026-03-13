# Weather8bit

A retro-styled terminal weather application written in Go with zero external dependencies. Retrieves real-time weather data for any location on Earth and renders it entirely in ASCII with ANSI colour.

---

## Contents

- [Features](#features)
- [Installation](#installation)
- [Usage](#usage)
- [Flags](#flags)
- [Live Mode](#live-mode)
- [Battle Mode](#battle-mode)
- [Building from Source](#building-from-source)
- [Terminal Requirements](#terminal-requirements)
- [Data Source](#data-source)
- [License](#license)

---

## Features

**Standard weather output**

- Current conditions: temperature, apparent temperature, humidity, precipitation, atmospheric pressure, visibility, wind speed and direction, cloud cover, UV index
- 7-day daily forecast with ASCII weather icons, temperature range, precipitation probability, wind, UV index, and sunrise/sunset times
- 24-hour hourly forecast table
- Weekly statistics: minimum, maximum, average, and cumulative totals
- Temperature rendered in pixel-block digits
- All values colour-coded by severity or range

**Exclusive modes**

| Mode | Flag | Description |
|------|------|-------------|
| Live | `--live` | Full-screen animated ASCII weather scene at ~12 fps |
| Battle | `--battle "City"` | Real-time RPG combat between two cities using actual weather data as stats |

These modes are described in detail below.

---

## Installation

No installer or runtime is required. Download the binary for your platform and run it directly.

| Binary | Platform |
|--------|----------|
| `weather8bit-linux-x64` | Linux x86\_64 |
| `weather8bit-linux-arm64` | Linux ARM64 (Raspberry Pi, cloud VMs) |
| `weather8bit-macos-arm64` | macOS Apple Silicon (M1/M2/M3/M4) |
| `weather8bit-macos-intel` | macOS Intel |
| `weather8bit-windows-x64.exe` | Windows 10/11 64-bit |

**Linux / macOS**

```bash
chmod +x weather8bit-linux-x64
./weather8bit-linux-x64 "London"
```

**Windows**

```bat
weather8bit-windows-x64.exe "London"
```

**Optional: add to PATH**

```bash
sudo mv weather8bit-linux-x64 /usr/local/bin/weather8bit
weather8bit "London"
```

---

## Usage

```
weather8bit <location> [flags]
```

Locations can be specified as a city name, a city and country, or a latitude/longitude pair.

```bash
weather8bit "Tokyo"
weather8bit "Springfield, US"
weather8bit "38.7169,-9.1399"
weather8bit                        # interactive prompt if no argument is given
```

---

## Flags

| Flag | Short | Description |
|------|-------|-------------|
| `--hourly` | `-H` | Show the 24-hour hourly forecast table |
| `--stats` | `-s` | Show weekly statistics |
| `--all` | `-a` | Show both hourly forecast and statistics |
| `--live` | `-l` | Launch the full-screen animated ASCII scene |
| `--battle "City"` | `-b "City"` | Run a weather battle against another city |
| `--help` | `-h` | Print usage information |

**Examples**

```bash
weather8bit "Lisbon" --all
weather8bit "New York" --hourly
weather8bit "Sydney" --stats
weather8bit "51.5074,-0.1278" --all
weather8bit "Tokyo" --live
weather8bit "Tokyo" --battle "London"
```

---

## Live Mode

Launched with `--live`, this mode renders a full-screen animated ASCII weather scene that updates at approximately 12 frames per second. The animation reflects actual current conditions:

| Condition | Animation |
|-----------|-----------|
| Clear sky (day) | Rotating eight-ray sun with drifting light scatter |
| Clear sky (night) | Twinkling star field with moon |
| Partly cloudy | Sun with drifting cloud shapes |
| Overcast | Heavy cloud layer scrolling across the screen |
| Rain | Falling particle stream angled by real wind speed and direction, with ground splash |
| Heavy rain | Dense particle stream, increased fall velocity |
| Snow | Snowflakes drifting on a sine-wave path, accumulating at the ground line |
| Thunderstorm | Heavy rain with periodic screen-flash lightning and a jagged animated bolt |
| Fog | Layered sine-wave fog bands scrolling at variable speeds |

A live stats panel is rendered in the lower-left corner showing current conditions. Press `Ctrl+C` to exit. The terminal is fully restored on exit.

```bash
weather8bit "Reykjavik" --live
weather8bit "Mumbai" --live
```

---

## Battle Mode

Launched with `--battle "City"`, this mode fetches live weather for both cities and converts their meteorological data into RPG combat statistics. The two cities then fight across six rounds.

**Stat derivation**

| Stat | Derived from |
|------|--------------|
| ATK | Temperature extremity (distance from 0 C) plus UV index |
| DEF | Atmospheric pressure |
| SPD | Wind speed — higher SPD attacks first each round |
| SPC | Relative humidity — determines special move probability |
| HP | Base value adjusted by atmospheric pressure |

**Combat mechanics**

- The fighter with higher SPD attacks first each round
- Damage is calculated from attacker ATK minus a fraction of defender DEF
- Humidity-based special moves deal 1.7x damage
- High SPD provides a small dodge chance
- Each weather condition has its own named attack set (e.g. thunderstorm cities use `LIGHTNING STRIKE`, `VOLTAGE SHOCK`; rainy cities use `RELENTLESS DRIZZLE`, `FLOOD RUSH`)
- A projectile animation crosses the arena on each attack
- If no knockout occurs within six rounds, the city with more remaining HP wins

```bash
weather8bit "Dubai" --battle "Oslo"
weather8bit "Miami" --battle "Reykjavik"
weather8bit "London" --battle "São Paulo"
```

---

## Building from Source

Requires Go 1.22 or later. The project has no external dependencies and uses only the Go standard library.

```bash
git clone https://github.com/your-username/weather8bit
cd weather8bit
go build -o weather8bit .
./weather8bit "Lisbon"
```

**Cross-compilation**

```bash
# Windows
GOOS=windows GOARCH=amd64 go build -ldflags="-s -w" -o weather8bit.exe .

# macOS Apple Silicon
GOOS=darwin GOARCH=arm64 go build -ldflags="-s -w" -o weather8bit-macos .

# Linux ARM64
GOOS=linux GOARCH=arm64 go build -ldflags="-s -w" -o weather8bit-arm64 .
```

**Source files**

| File | Contents |
|------|----------|
| `main.go` | Entry point, argument parsing, weather display (current, weekly, hourly, stats) |
| `live.go` | Animated ASCII scene engine and particle system |
| `battle.go` | RPG battle mode logic, stat derivation, arena renderer |

---

## Terminal Requirements

- ANSI colour support (standard in all modern terminals)
- Minimum width of 120 columns recommended; wider terminals improve the 7-day panel layout
- Live mode works best at 140 columns or wider and 40 rows or taller
- Any monospace font — the application uses standard Unicode block characters (`▓`, `█`, `░`)

On Windows, Windows Terminal or PowerShell is recommended. The legacy `cmd.exe` supports ANSI colours on Windows 10 and later but may render some block characters inconsistently depending on the configured font.

---

## Data Source

Weather data is provided by [Open-Meteo](https://open-meteo.com), a free and open-source weather API.

- No account or API key required
- Global coverage
- Hourly forecast updates
- Licensed under CC BY 4.0

---

## License

This software is free for personal and non-commercial use.  
Weather data copyright Open-Meteo, licensed under CC BY 4.0.

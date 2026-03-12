package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

// ─── ANSI ──────────────────────────────────────────────────────────────────────

const (
	Reset         = "\033[0m"
	Bold          = "\033[1m"
	Dim           = "\033[2m"
	Red           = "\033[31m"
	Green         = "\033[32m"
	Yellow        = "\033[33m"
	Blue          = "\033[34m"
	Cyan          = "\033[36m"
	White         = "\033[37m"
	BrightBlack   = "\033[90m"
	BrightRed     = "\033[91m"
	BrightGreen   = "\033[92m"
	BrightYellow  = "\033[93m"
	BrightBlue    = "\033[94m"
	BrightMagenta = "\033[95m"
	BrightCyan    = "\033[96m"
	BrightWhite   = "\033[97m"
)

func col(color, text string) string  { return color + text + Reset }
func bold(color, text string) string { return Bold + color + text + Reset }

// ─── UNICODE-AWARE WIDTH ───────────────────────────────────────────────────────
// BUG FIX: old visLen counted emoji/wide chars as 1 col — they are 2 cols wide.

func runeWidth(r rune) int {
	if r >= 0x1100 {
		if r <= 0x115F || r == 0x2329 || r == 0x232A ||
			(r >= 0x2E80 && r <= 0x303E) ||
			(r >= 0x3040 && r <= 0x33FF) ||
			(r >= 0xAC00 && r <= 0xD7A3) ||
			(r >= 0xF900 && r <= 0xFAFF) ||
			(r >= 0xFE30 && r <= 0xFE4F) ||
			(r >= 0xFF01 && r <= 0xFF60) ||
			(r >= 0xFFE0 && r <= 0xFFE6) ||
			(r >= 0x1F300 && r <= 0x1FAFF) {
			return 2
		}
	}
	return 1
}

// visLen returns the visible column width (strips ANSI escapes, counts wide chars)
func visLen(s string) int {
	inEsc := false
	n := 0
	for _, r := range s {
		if r == '\033' {
			inEsc = true
		} else if inEsc {
			if r == 'm' {
				inEsc = false
			}
		} else {
			n += runeWidth(r)
		}
	}
	return n
}

// ansiPad pads s to width visible columns.
// BUG FIX: replaces fmt.Sprintf("%-Ns", ansiStr) which padded by byte count
// including hidden ANSI escape sequences, causing totally wrong alignment.
func ansiPad(s string, width int) string {
	vl := visLen(s)
	if vl >= width {
		return s
	}
	return s + strings.Repeat(" ", width-vl)
}

// safeRuneTrunc truncates s to at most maxCols visible columns, ANSI-safe.
// BUG FIX: replaces cell[:colW-1] which panics when slicing mid-UTF-8 sequence.
func safeRuneTrunc(s string, maxCols int) string {
	inEsc := false
	cols := 0
	var out strings.Builder
	i := 0
	for i < len(s) {
		r, size := utf8.DecodeRuneInString(s[i:])
		if r == '\033' {
			inEsc = true
			out.WriteRune(r)
			i += size
			continue
		}
		if inEsc {
			out.WriteRune(r)
			if r == 'm' {
				inEsc = false
			}
			i += size
			continue
		}
		w := runeWidth(r)
		if cols+w > maxCols {
			break
		}
		cols += w
		out.WriteRune(r)
		i += size
	}
	out.WriteString(Reset)
	// Pad to full width
	if cols < maxCols {
		out.WriteString(strings.Repeat(" ", maxCols-cols))
	}
	return out.String()
}

func rep(s string, n int) string {
	if n <= 0 {
		return ""
	}
	return strings.Repeat(s, n)
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// ─── PIXEL FONT ────────────────────────────────────────────────────────────────

var pixelGlyphs = map[rune][5]string{
	'0': {"▓▓▓", "▓ ▓", "▓ ▓", "▓ ▓", "▓▓▓"},
	'1': {" ▓▓", " ▓▓", "  ▓", "  ▓", "  ▓"},
	'2': {"▓▓▓", "  ▓", "▓▓▓", "▓  ", "▓▓▓"},
	'3': {"▓▓▓", "  ▓", "▓▓▓", "  ▓", "▓▓▓"},
	'4': {"▓ ▓", "▓ ▓", "▓▓▓", "  ▓", "  ▓"},
	'5': {"▓▓▓", "▓  ", "▓▓▓", "  ▓", "▓▓▓"},
	'6': {"▓▓▓", "▓  ", "▓▓▓", "▓ ▓", "▓▓▓"},
	'7': {"▓▓▓", "  ▓", "  ▓", "  ▓", "  ▓"},
	'8': {"▓▓▓", "▓ ▓", "▓▓▓", "▓ ▓", "▓▓▓"},
	'9': {"▓▓▓", "▓ ▓", "▓▓▓", "  ▓", "▓▓▓"},
	'-': {"   ", "   ", "▓▓▓", "   ", "   "},
	'°': {"▓▓ ", "▓▓ ", "   ", "   ", "   "},
	' ': {"   ", "   ", "   ", "   ", "   "},
	'.': {"   ", "   ", "   ", "   ", " ▓ "},
	'C': {"▓▓▓", "▓  ", "▓  ", "▓  ", "▓▓▓"},
}

func pixelText(text, color string) []string {
	rows := [5]strings.Builder{}
	for _, ch := range text {
		g, ok := pixelGlyphs[ch]
		if !ok {
			g = pixelGlyphs[' ']
		}
		for i := 0; i < 5; i++ {
			rows[i].WriteString(g[i])
			rows[i].WriteString(" ")
		}
	}
	out := make([]string, 5)
	for i := 0; i < 5; i++ {
		out[i] = color + rows[i].String() + Reset
	}
	return out
}

// ─── WEATHER ICONS (all ASCII, no wide chars) ──────────────────────────────────

type IconDef struct {
	lines [5]string
	color string
}

// Icon lines must all be exactly 11 visible ASCII chars (safe for column math)
var icons = map[string]IconDef{
	"sunny":        {[5]string{"  \\ | /    ", "   \\|/     ", "---[*]---  ", "   /|\\     ", "  / | \\    "}, BrightYellow},
	"partly_cloudy":{[5]string{"  \\ | /    ", "   (*) ___ ", "  ---(___)_", "      (___)", "           "}, Yellow},
	"cloudy":       {[5]string{"           ", "  _______  ", " (________)","  '------' ", "           "}, White},
	"rainy":        {[5]string{"  _______  ", " (________)", "  '------' ", "           ", " , , , , , "}, Cyan},
	"heavy_rain":   {[5]string{"  _______  ", " (________)", "  '------' ", "           ", ",,,,,,,,,,,"}  , Blue},
	"snowy":        {[5]string{"  _______  ", " (________)", "  '------' ", "           ", " * * * * * "}, BrightWhite},
	"thunderstorm": {[5]string{"  _______  ", " (________)", "  '----^-- ", "      |    ", "      *    "}, BrightYellow},
	"foggy":        {[5]string{" --------- ", "- - - - - -", " --------- ", "- - - - - -", " --------- "}, BrightBlack},
	"night_clear":  {[5]string{"    *   .  ", "  . (  )   ", "     '..'  ", "  *     .  ", "    .   *  "}, BrightBlack},
}

// ─── WMO CODES ─────────────────────────────────────────────────────────────────

type WeatherInfo struct {
	desc    string
	iconKey string
	color   string
}

var wmoCodes = map[int]WeatherInfo{
	0:  {"Clear Sky", "sunny", BrightYellow},
	1:  {"Mainly Clear", "partly_cloudy", Yellow},
	2:  {"Partly Cloudy", "partly_cloudy", Yellow},
	3:  {"Overcast", "cloudy", White},
	45: {"Foggy", "foggy", BrightBlack},
	48: {"Icy Fog", "foggy", BrightBlack},
	51: {"Light Drizzle", "rainy", Cyan},
	53: {"Drizzle", "rainy", Cyan},
	55: {"Heavy Drizzle", "rainy", Blue},
	56: {"Freezing Drizzle", "snowy", BrightCyan},
	57: {"Hvy Frz Drizzle", "snowy", BrightCyan},
	61: {"Light Rain", "rainy", Cyan},
	63: {"Rain", "rainy", Blue},
	65: {"Heavy Rain", "heavy_rain", Blue},
	66: {"Light Frz Rain", "snowy", BrightCyan},
	67: {"Heavy Frz Rain", "snowy", BrightCyan},
	71: {"Light Snow", "snowy", BrightWhite},
	73: {"Snow", "snowy", BrightWhite},
	75: {"Heavy Snow", "snowy", White},
	77: {"Snow Grains", "snowy", White},
	80: {"Light Showers", "rainy", Cyan},
	81: {"Showers", "rainy", Blue},
	82: {"Heavy Showers", "heavy_rain", Blue},
	85: {"Snow Showers", "snowy", BrightWhite},
	86: {"Heavy Snow Showers", "snowy", White},
	95: {"Thunderstorm", "thunderstorm", BrightYellow},
	96: {"Thunderstorm+Hail", "thunderstorm", BrightYellow},
	99: {"Thunderstorm+Hail", "thunderstorm", BrightYellow},
}

func getWMO(code int) WeatherInfo {
	if w, ok := wmoCodes[code]; ok {
		return w
	}
	return WeatherInfo{"Unknown", "cloudy", White}
}

// ─── VISUAL BARS ──────────────────────────────────────────────────────────────

func fillBar(pct, total int, fillColor string) string {
	if pct < 0 {
		pct = 0
	}
	if pct > total {
		pct = total
	}
	return fillColor + rep("█", pct) + BrightBlack + rep("░", total-pct) + Reset
}

func humidityBar(h float64) string {
	c := Cyan
	if h >= 80 {
		c = BrightBlue
	} else if h >= 60 {
		c = Blue
	}
	return fillBar(int(h/10), 10, c) + " " + fmt.Sprintf("%.0f%%", h)
}

func precipBar(pct float64) string {
	c := Cyan
	if pct > 50 {
		c = Blue
	}
	return fillBar(int(pct/10), 10, c) + " " + fmt.Sprintf("%.0f%%", pct)
}

func uvBar(uv float64) string {
	c := Green
	if uv > 10 {
		c = BrightMagenta
	} else if uv > 7 {
		c = Red
	} else if uv > 5 {
		c = BrightRed
	} else if uv > 2 {
		c = Yellow
	}
	return fillBar(int(math.Min(uv, 12)), 12, c) + " " + fmt.Sprintf("%.1f", uv)
}

func tempColor(t float64) string {
	switch {
	case t <= 0:
		return BrightCyan
	case t <= 10:
		return Cyan
	case t <= 20:
		return BrightGreen
	case t <= 28:
		return Yellow
	case t <= 35:
		return BrightYellow
	default:
		return BrightRed
	}
}

func windArrow(deg float64) string {
	dirs := []string{"N", "NE", "E", "SE", "S", "SW", "W", "NW"}
	arrows := []string{"^", "^>", ">", "v>", "v", "v<", "<", "^<"}
	idx := int(math.Round(deg/45)) % 8
	if idx < 0 {
		idx += 8
	}
	return arrows[idx] + dirs[idx]
}

// ─── TERMINAL WIDTH ────────────────────────────────────────────────────────────

func termWidth() int {
	if runtime.GOOS != "windows" {
		cmd := exec.Command("stty", "size")
		cmd.Stdin = os.Stdin
		if out, err := cmd.Output(); err == nil {
			parts := strings.Fields(strings.TrimSpace(string(out)))
			if len(parts) == 2 {
				if w, err := strconv.Atoi(parts[1]); err == nil && w > 40 {
					return w
				}
			}
		}
	}
	return 120
}

func printRule(title string) {
	w := termWidth()
	total := w - 4
	inner := " " + title + " "
	vis := visLen(inner)
	left := (total - vis) / 2
	if left < 0 {
		left = 0
	}
	right := total - vis - left
	if right < 0 {
		right = 0
	}
	fmt.Println()
	fmt.Println(col(BrightCyan, "  |"+rep("-", left))+bold(BrightCyan, inner)+col(BrightCyan, rep("-", right)+"|"))
}

// ─── API STRUCTS ───────────────────────────────────────────────────────────────

type GeoResult struct {
	Name      string  `json:"name"`
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
	Country   string  `json:"country"`
	Admin1    string  `json:"admin1"`
}

type GeoResponse struct {
	Results []GeoResult `json:"results"`
}

type CurrentWeather struct {
	Temperature2m       float64 `json:"temperature_2m"`
	RelativeHumidity2m  float64 `json:"relative_humidity_2m"`
	ApparentTemperature float64 `json:"apparent_temperature"`
	IsDay               int     `json:"is_day"`
	Precipitation       float64 `json:"precipitation"`
	WeatherCode         int     `json:"weather_code"`
	CloudCover          float64 `json:"cloud_cover"`
	PressureMsl         float64 `json:"pressure_msl"`
	WindSpeed10m        float64 `json:"wind_speed_10m"`
	WindDirection10m    float64 `json:"wind_direction_10m"`
	WindGusts10m        float64 `json:"wind_gusts_10m"`
	Visibility          float64 `json:"visibility"`
	UvIndex             float64 `json:"uv_index"`
}

type DailyWeather struct {
	Time                        []string  `json:"time"`
	WeatherCode                 []int     `json:"weather_code"`
	Temperature2mMax            []float64 `json:"temperature_2m_max"`
	Temperature2mMin            []float64 `json:"temperature_2m_min"`
	Sunrise                     []string  `json:"sunrise"`
	Sunset                      []string  `json:"sunset"`
	PrecipitationSum            []float64 `json:"precipitation_sum"`
	PrecipitationProbabilityMax []float64 `json:"precipitation_probability_max"`
	WindSpeed10mMax             []float64 `json:"wind_speed_10m_max"`
	UvIndexMax                  []float64 `json:"uv_index_max"`
	RelativeHumidity2mMax       []float64 `json:"relative_humidity_2m_max"`
}

type HourlyWeather struct {
	Time                     []string  `json:"time"`
	Temperature2m            []float64 `json:"temperature_2m"`
	RelativeHumidity2m       []float64 `json:"relative_humidity_2m"`
	PrecipitationProbability []float64 `json:"precipitation_probability"`
	Precipitation            []float64 `json:"precipitation"`
	WeatherCode              []int     `json:"weather_code"`
	WindSpeed10m             []float64 `json:"wind_speed_10m"`
	Visibility               []float64 `json:"visibility"`
}

type WeatherResponse struct {
	Current          CurrentWeather    `json:"current"`
	Daily            DailyWeather      `json:"daily"`
	Hourly           HourlyWeather     `json:"hourly"`
	Timezone         string            `json:"timezone"`
	TimezoneAbbr     string            `json:"timezone_abbreviation"`
	UTCOffsetSeconds int               `json:"utc_offset_seconds"`
}

// ─── API CALLS ─────────────────────────────────────────────────────────────────

func geocode(location string) ([]GeoResult, error) {
	u := "https://geocoding-api.open-meteo.com/v1/search?name=" +
		url.QueryEscape(location) + "&count=5&language=en&format=json"
	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Get(u)
	if err != nil {
		return nil, fmt.Errorf("network error: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read error: %w", err)
	}
	// BUG FIX: errors from json.Unmarshal were previously silently ignored
	var gr GeoResponse
	if err := json.Unmarshal(body, &gr); err != nil {
		return nil, fmt.Errorf("parse error: %w", err)
	}
	return gr.Results, nil
}

func fetchWeather(lat, lon float64) (*WeatherResponse, error) {
	u := fmt.Sprintf(
		"https://api.open-meteo.com/v1/forecast"+
			"?latitude=%.4f&longitude=%.4f"+
			"&current=temperature_2m,relative_humidity_2m,apparent_temperature,is_day,"+
			"precipitation,weather_code,cloud_cover,pressure_msl,wind_speed_10m,"+
			"wind_direction_10m,wind_gusts_10m,visibility,uv_index"+
			"&hourly=temperature_2m,relative_humidity_2m,precipitation_probability,"+
			"precipitation,weather_code,wind_speed_10m,visibility"+
			"&daily=weather_code,temperature_2m_max,temperature_2m_min,sunrise,sunset,"+
			"precipitation_sum,precipitation_probability_max,wind_speed_10m_max,"+
			"uv_index_max,relative_humidity_2m_max"+
			"&timezone=auto&forecast_days=7&wind_speed_unit=kmh",
		lat, lon,
	)
	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Get(u)
	if err != nil {
		return nil, fmt.Errorf("network error: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read error: %w", err)
	}
	// BUG FIX: errors were silently ignored; now return them
	var wr WeatherResponse
	if err := json.Unmarshal(body, &wr); err != nil {
		return nil, fmt.Errorf("parse error: %w\nbody: %s", err, string(body[:minInt(len(body), 300)]))
	}
	// BUG FIX: validate the response actually has data
	if len(wr.Daily.Time) == 0 {
		snippet := string(body)
		if len(snippet) > 300 {
			snippet = snippet[:300]
		}
		return nil, fmt.Errorf("API returned no forecast data. Response: %s", snippet)
	}
	return &wr, nil
}

// ─── SAFE SLICE ACCESS ─────────────────────────────────────────────────────────

func safeF(arr []float64, i int) float64 {
	if i >= 0 && i < len(arr) {
		return arr[i]
	}
	return 0
}

func safeI(arr []int, i int) int {
	if i >= 0 && i < len(arr) {
		return arr[i]
	}
	return 0
}

func safeS(arr []string, i int) string {
	if i >= 0 && i < len(arr) {
		return arr[i]
	}
	return ""
}

// ─── SCREEN & HEADER ──────────────────────────────────────────────────────────

func clearScreen() {
	if runtime.GOOS == "windows" {
		cmd := exec.Command("cmd", "/c", "cls")
		cmd.Stdout = os.Stdout
		cmd.Run()
	} else {
		fmt.Print("\033[H\033[2J")
	}
}

func printHeader() {
	fmt.Println()
	fmt.Println(bold(BrightCyan,  "  ##   ## ###### ##  ## ######## ##  ## ###### ###### #####  "))
	fmt.Println(bold(Cyan,        "  ##   ## ##     ## ##     ##    ##  ## ##     ##  ## ##  ## "))
	fmt.Println(bold(BrightCyan,  "  ## # ## #####  ####      ##    ###### #####  ######  ##### "))
	fmt.Println(bold(Cyan,        "  ####### ##     ## ##     ##    ##  ## ##     ##  ## ##  ## "))
	fmt.Println(bold(BrightCyan,  "   ## ##  ###### ##  ##    ##    ##  ## ###### ##  ## #####  "))
	fmt.Println(bold(BrightWhite, "         [ 8-BIT WORLD WEATHER CLI ]  FREE  NO API KEY       "))
	w := termWidth()
	fmt.Println(col(BrightCyan, "  +"+rep("=", w-4)+"+"))
}

// ─── LOADING SPINNER ──────────────────────────────────────────────────────────
// BUG FIX: was "go loading(...)" with no synchronization — goroutine raced with
// subsequent output. Now returns a stop func; caller blocks until goroutine exits.

func startLoading(msg string) (stop func()) {
	frames := []string{"|", "/", "-", "\\"}
	done := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		i := 0
		for {
			select {
			case <-done:
				fmt.Print("\r" + rep(" ", len(msg)+20) + "\r")
				return
			default:
				fmt.Printf("\r  %s  %s", col(BrightCyan, frames[i%len(frames)]), bold(BrightCyan, msg))
				time.Sleep(80 * time.Millisecond)
				i++
			}
		}
	}()
	return func() {
		close(done)
		wg.Wait()
	}
}

// ─── CURRENT CONDITIONS ───────────────────────────────────────────────────────

func printCurrent(wr *WeatherResponse, locationName string) {
	cur := wr.Current
	wi := getWMO(cur.WeatherCode)
	icon := icons[wi.iconKey]

	if cur.IsDay == 0 && (wi.iconKey == "sunny" || wi.iconKey == "partly_cloudy") {
		icon = icons["night_clear"]
		wi.color = BrightBlack
	}

	tc := tempColor(cur.Temperature2m)
	pixLines := pixelText(fmt.Sprintf("%.0f°C", cur.Temperature2m), tc)

	printRule("[ CURRENT CONDITIONS ]")
	fmt.Println()

	// All info lines — no fmt.Sprintf width tricks, just ANSI-colored strings
	infoLines := []string{
		bold(BrightWhite, "LOCATION   ") + col(BrightCyan, locationName),
		bold(BrightWhite, "CONDITION  ") + col(wi.color, wi.desc),
		bold(BrightWhite, "TEMP       ") + col(tc, fmt.Sprintf("%.1f°C", cur.Temperature2m)) +
			col(BrightBlack, fmt.Sprintf("  feels %.1f°C", cur.ApparentTemperature)),
		bold(BrightWhite, "HUMIDITY   ") + humidityBar(cur.RelativeHumidity2m),
		bold(BrightWhite, "PRECIP     ") + col(Cyan, fmt.Sprintf("%.1f mm", cur.Precipitation)),
		bold(BrightWhite, "PRESSURE   ") + col(Green, fmt.Sprintf("%.1f hPa", cur.PressureMsl)),
		bold(BrightWhite, "VISIBILITY ") + col(Yellow, fmt.Sprintf("%.1f km", cur.Visibility/1000)),
		bold(BrightWhite, "WIND       ") + col(BrightGreen, fmt.Sprintf("%.0f km/h %s  gusts %.0f", cur.WindSpeed10m, windArrow(cur.WindDirection10m), cur.WindGusts10m)),
		bold(BrightWhite, "CLOUD      ") + col(White, fmt.Sprintf("%.0f%%", cur.CloudCover)),
		bold(BrightWhite, "UV INDEX   ") + uvBar(cur.UvIndex),
		bold(BrightWhite, "TIME       ") + col(BrightBlack, time.Now().Format("Mon 02 Jan 2006  15:04:05")),
	}

	const iconColW = 14
	const pixColW = 30

	maxLines := len(icon.lines)
	if len(pixLines) > maxLines {
		maxLines = len(pixLines)
	}
	if len(infoLines) > maxLines {
		maxLines = len(infoLines)
	}

	for i := 0; i < maxLines; i++ {
		// BUG FIX: use ansiPad instead of fmt.Sprintf("%-13s", iconLine) which
		// doesn't account for ANSI escapes and wide chars in the icon color
		var iconCol, pixCol, infoCol string
		if i < len(icon.lines) {
			iconCol = ansiPad(col(icon.color, icon.lines[i]), iconColW)
		} else {
			iconCol = rep(" ", iconColW)
		}
		if i < len(pixLines) {
			pixCol = ansiPad(pixLines[i], pixColW)
		} else {
			pixCol = rep(" ", pixColW)
		}
		if i < len(infoLines) {
			infoCol = infoLines[i]
		}
		fmt.Println("  " + iconCol + "  " + pixCol + "  " + infoCol)
	}
}

// ─── WEEKLY FORECAST ──────────────────────────────────────────────────────────

func printWeekly(wr *WeatherResponse) {
	daily := wr.Daily
	days := daily.Time
	if len(days) == 0 {
		fmt.Println(col(Red, "  No daily forecast data available."))
		return
	}

	printRule("[ 7-DAY FORECAST ]")
	fmt.Println()

	nDays := minInt(len(days), 7)
	w := termWidth()
	colW := (w - 4) / nDays
	if colW < 16 {
		colW = 16
	}

	type DayPanel struct {
		lines []string
		today bool
	}
	panels := make([]DayPanel, nDays)

	for i := 0; i < nDays; i++ {
		dt, err := time.Parse("2006-01-02", days[i])
		if err != nil {
			dt = time.Now().AddDate(0, 0, i)
		}

		wc := safeI(daily.WeatherCode, i)
		wi := getWMO(wc)
		icon := icons[wi.iconKey]

		tMax := safeF(daily.Temperature2mMax, i)
		tMin := safeF(daily.Temperature2mMin, i)
		precip := safeF(daily.PrecipitationSum, i)
		precipPct := safeF(daily.PrecipitationProbabilityMax, i)
		wind := safeF(daily.WindSpeed10mMax, i)
		humidity := safeF(daily.RelativeHumidity2mMax, i)
		uv := safeF(daily.UvIndexMax, i)

		sr, ss := "?????", "?????"
		if raw := safeS(daily.Sunrise, i); strings.Contains(raw, "T") {
			parts := strings.SplitN(raw, "T", 2)
			if len(parts[1]) >= 5 {
				sr = parts[1][:5]
			}
		}
		if raw := safeS(daily.Sunset, i); strings.Contains(raw, "T") {
			parts := strings.SplitN(raw, "T", 2)
			if len(parts[1]) >= 5 {
				ss = parts[1][:5]
			}
		}

		maxC := tempColor(tMax)
		minC := tempColor(tMin)
		uvC := Green
		switch {
		case uv > 10:
			uvC = BrightMagenta
		case uv > 7:
			uvC = Red
		case uv > 5:
			uvC = Yellow
		case uv > 2:
			uvC = BrightYellow
		}

		desc := wi.desc
		if len([]rune(desc)) > 13 {
			desc = string([]rune(desc)[:13])
		}

		var lines []string
		if i == 0 {
			lines = append(lines, bold(BrightYellow, "* TODAY *    "))
		} else {
			lines = append(lines, col(BrightBlack, strings.ToUpper(dt.Format("Monday    "))))
		}
		lines = append(lines, bold(BrightWhite, dt.Format("02 Jan 2006")))
		lines = append(lines, col(BrightBlack, "-------------"))
		for _, l := range icon.lines {
			// icon lines are pure ASCII 11 chars — safe to use directly
			lines = append(lines, col(wi.color, l))
		}
		lines = append(lines, col(wi.color, desc))
		lines = append(lines, "")
		lines = append(lines, col(maxC, fmt.Sprintf("^ %+.0f", tMax))+"C  "+col(minC, fmt.Sprintf("v %+.0f", tMin))+"C")
		lines = append(lines, col(Cyan, fmt.Sprintf("Rain: %.1fmm", precip))+"  "+col(Blue, fmt.Sprintf("%.0f%%", precipPct)))
		lines = append(lines, col(BrightGreen, fmt.Sprintf("Wind: %.0f km/h", wind)))
		lines = append(lines, col(Cyan, fmt.Sprintf("Hum:  %.0f%%", humidity)))
		lines = append(lines, col(uvC, fmt.Sprintf("UV:   %.0f", uv)))
		lines = append(lines, col(Yellow, "Rise: "+sr)+"  "+col(BrightYellow, "Set: "+ss))

		panels[i] = DayPanel{lines: lines, today: i == 0}
	}

	borderCol := func(i int) string {
		if panels[i].today {
			return BrightCyan
		}
		return BrightBlack
	}

	maxL := 0
	for _, p := range panels {
		if len(p.lines) > maxL {
			maxL = len(p.lines)
		}
	}

	// Top border
	line := "  " + col(borderCol(0), "+")
	for i := range panels {
		line += col(borderCol(i), rep("-", colW))
		if i < len(panels)-1 {
			line += col(BrightBlack, "+")
		}
	}
	line += col(borderCol(len(panels)-1), "+")
	fmt.Println(line)

	for row := 0; row < maxL; row++ {
		line = "  " + col(borderCol(0), "|")
		for j, p := range panels {
			cell := ""
			if row < len(p.lines) {
				cell = " " + p.lines[row]
			}
			// BUG FIX: safeRuneTrunc replaces cell[:colW-1] which panics on Unicode
			line += safeRuneTrunc(cell, colW)
			nextBorder := j + 1
			if nextBorder >= len(panels) {
				nextBorder = len(panels) - 1
			}
			line += col(borderCol(nextBorder), "|")
		}
		fmt.Println(line)
	}

	// Bottom border
	line = "  " + col(borderCol(0), "+")
	for i := range panels {
		line += col(borderCol(i), rep("-", colW))
		if i < len(panels)-1 {
			line += col(BrightBlack, "+")
		}
	}
	line += col(borderCol(len(panels)-1), "+")
	fmt.Println(line)
}

// ─── HOURLY FORECAST ──────────────────────────────────────────────────────────

func printHourly(wr *WeatherResponse) {
	hourly := wr.Hourly
	if len(hourly.Time) == 0 {
		fmt.Println(col(Red, "  No hourly data available."))
		return
	}

	printRule("[ HOURLY FORECAST - NEXT 24H ]")
	fmt.Println()

	// BUG FIX: was time.ParseInLocation with local tz, which was wrong for remote
	// locations. Now use the UTC offset from the API response to build the correct
	// fixed-offset location, and compare against now-in-that-zone.
	loc := time.FixedZone(wr.TimezoneAbbr, wr.UTCOffsetSeconds)
	nowAtLoc := time.Now().In(loc)

	type col_ struct {
		label string
		w     int
	}
	columns := []col_{
		{"TIME", 8}, {"CONDITION", 19}, {"TEMP", 10},
		{"HUMIDITY", 17}, {"PRECIP%", 14}, {"RAIN", 7},
		{"WIND", 10}, {"VIS", 7},
	}

	hdr := "  "
	sep := "  "
	for _, c := range columns {
		hdr += ansiPad(bold(BrightCyan, c.label), c.w+1)
		sep += col(BrightBlack, rep("-", c.w)) + " "
	}
	fmt.Println(hdr)
	fmt.Println(sep)

	shown := 0
	for i, ts := range hourly.Time {
		dt, err := time.ParseInLocation("2006-01-02T15:04", ts, loc)
		if err != nil {
			continue
		}
		// Skip hours more than 1h in the past
		if dt.Before(nowAtLoc.Add(-time.Hour)) {
			continue
		}
		if shown >= 24 {
			break
		}

		wc := safeI(hourly.WeatherCode, i)
		wi := getWMO(wc)
		temp := safeF(hourly.Temperature2m, i)
		hum := safeF(hourly.RelativeHumidity2m, i)
		pp := safeF(hourly.PrecipitationProbability, i)
		rain := safeF(hourly.Precipitation, i)
		wind := safeF(hourly.WindSpeed10m, i)
		vis := safeF(hourly.Visibility, i) / 1000
		tc := tempColor(temp)

		timeStr := dt.Format("15:04")
		if dt.YearDay() != nowAtLoc.YearDay() {
			timeStr = col(BrightBlack, dt.Format("Mon")+" "+dt.Format("15:04"))
		}

		desc := wi.desc
		if len([]rune(desc)) > 17 {
			desc = string([]rune(desc)[:17])
		}

		// BUG FIX: all %-Ns on ANSI-colored strings replaced with ansiPad
		row := "  " +
			ansiPad(timeStr, 9) +
			ansiPad(col(wi.color, desc), 20) +
			ansiPad(col(tc, fmt.Sprintf("%.1f°C", temp)), 11) +
			ansiPad(humidityBar(hum), 18) +
			ansiPad(precipBar(pp), 15) +
			ansiPad(col(Cyan, fmt.Sprintf("%.1f", rain)), 8) +
			ansiPad(col(BrightGreen, fmt.Sprintf("%.0f", wind)), 11) +
			col(Yellow, fmt.Sprintf("%.1f", vis))

		if shown%2 == 1 {
			fmt.Println(Dim + row + Reset)
		} else {
			fmt.Println(row)
		}
		shown++
	}
}

// ─── STATISTICS ───────────────────────────────────────────────────────────────

func printStats(wr *WeatherResponse) {
	d := wr.Daily
	if len(d.Time) == 0 {
		return
	}
	printRule("[ WEEK STATISTICS ]")
	fmt.Println()

	minMaxAvg := func(arr []float64) (mn, mx, avg float64) {
		if len(arr) == 0 {
			return 0, 0, 0
		}
		mn, mx = arr[0], arr[0]
		sum := 0.0
		for _, v := range arr {
			if v < mn {
				mn = v
			}
			if v > mx {
				mx = v
			}
			sum += v
		}
		return mn, mx, sum / float64(len(arr))
	}

	type statRow struct{ label, minS, maxS, avgS, totalS string }
	var rows []statRow

	allT := append(append([]float64{}, d.Temperature2mMax...), d.Temperature2mMin...)
	if len(allT) > 0 {
		mn, mx, avg := minMaxAvg(allT)
		rows = append(rows, statRow{"TEMPERATURE",
			col(BrightCyan, fmt.Sprintf("%.1f°C", mn)),
			col(BrightRed, fmt.Sprintf("%.1f°C", mx)),
			col(Yellow, fmt.Sprintf("%.1f°C", avg)),
			col(BrightBlack, "---"),
		})
	}
	if len(d.PrecipitationSum) > 0 {
		mn, mx, avg := minMaxAvg(d.PrecipitationSum)
		total := 0.0
		for _, v := range d.PrecipitationSum {
			total += v
		}
		rows = append(rows, statRow{"PRECIPITATION",
			col(Cyan, fmt.Sprintf("%.1fmm", mn)),
			col(Blue, fmt.Sprintf("%.1fmm", mx)),
			col(Yellow, fmt.Sprintf("%.1fmm", avg)),
			col(Green, fmt.Sprintf("%.1fmm", total)),
		})
	}
	if len(d.WindSpeed10mMax) > 0 {
		mn, mx, avg := minMaxAvg(d.WindSpeed10mMax)
		rows = append(rows, statRow{"WIND SPEED",
			col(BrightGreen, fmt.Sprintf("%.0f km/h", mn)),
			col(BrightRed, fmt.Sprintf("%.0f km/h", mx)),
			col(Yellow, fmt.Sprintf("%.0f km/h", avg)),
			col(BrightBlack, "---"),
		})
	}
	if len(d.UvIndexMax) > 0 {
		mn, mx, avg := minMaxAvg(d.UvIndexMax)
		rows = append(rows, statRow{"UV INDEX",
			col(Green, fmt.Sprintf("%.1f", mn)),
			col(BrightRed, fmt.Sprintf("%.1f", mx)),
			col(Yellow, fmt.Sprintf("%.1f", avg)),
			col(BrightBlack, "---"),
		})
	}

	const colW = 22
	fmt.Println("  " +
		ansiPad(bold(BrightCyan, "METRIC"), colW) +
		ansiPad(bold(BrightCyan, "MIN"), colW) +
		ansiPad(bold(BrightCyan, "MAX"), colW) +
		ansiPad(bold(BrightCyan, "AVERAGE"), colW) +
		bold(BrightCyan, "TOTAL"))
	fmt.Println("  " + col(BrightBlack, rep("-", colW*5)))
	for _, r := range rows {
		fmt.Println("  " +
			ansiPad(bold(BrightWhite, r.label), colW) +
			ansiPad(r.minS, colW) +
			ansiPad(r.maxS, colW) +
			ansiPad(r.avgS, colW) +
			r.totalS)
	}
}

// ─── LOCATION SELECT ──────────────────────────────────────────────────────────

func selectLocation(results []GeoResult) *GeoResult {
	if len(results) == 0 {
		return nil
	}
	if len(results) == 1 {
		return &results[0]
	}
	printRule("[ MULTIPLE LOCATIONS FOUND ]")
	fmt.Println()
	n := minInt(len(results), 5)
	for i := 0; i < n; i++ {
		r := results[i]
		fmt.Printf("  %s  %-20s  %-15s  %-15s  %s\n",
			bold(BrightYellow, fmt.Sprintf("[%d]", i+1)),
			bold(BrightWhite, r.Name),
			col(Yellow, r.Admin1),
			col(Green, r.Country),
			col(BrightBlack, fmt.Sprintf("%.2f, %.2f", r.Latitude, r.Longitude)),
		)
	}
	fmt.Println()
	fmt.Print(bold(BrightCyan, "  > SELECT (1-5): "))
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		idx, err := strconv.Atoi(strings.TrimSpace(scanner.Text()))
		if err == nil && idx >= 1 && idx <= n {
			return &results[idx-1]
		}
		fmt.Print(col(Red, "  Invalid. ")+bold(BrightCyan, "> SELECT (1-5): "))
	}
	return &results[0]
}

// ─── MAIN ─────────────────────────────────────────────────────────────────────

func main() {
	showHourly := false
	showStats := false
	locationArg := ""

	for _, a := range os.Args[1:] {
		switch a {
		case "--hourly", "-H":
			showHourly = true
		case "--stats", "-s":
			showStats = true
		case "--all", "-a":
			showHourly = true
			showStats = true
		case "--help", "-h":
			clearScreen()
			printHeader()
			fmt.Println()
			fmt.Println(bold(BrightWhite, "  USAGE:"))
			fmt.Println(col(BrightCyan, "    weather8bit <location> [options]"))
			fmt.Println()
			fmt.Println(bold(BrightWhite, "  OPTIONS:"))
			fmt.Println("    " + col(Yellow, "--hourly") + "   " + col(BrightBlack, "24-hour breakdown"))
			fmt.Println("    " + col(Yellow, "--stats") + "    " + col(BrightBlack, "Weekly statistics"))
			fmt.Println("    " + col(Yellow, "--all") + "      " + col(BrightBlack, "Show everything"))
			fmt.Println()
			fmt.Println(bold(BrightWhite, "  EXAMPLES:"))
			fmt.Println("    " + col(Green, `weather8bit "Tokyo"`))
			fmt.Println("    " + col(Green, `weather8bit "New York" --all`))
			fmt.Println("    " + col(Green, `weather8bit "48.8566,2.3522"`))
			fmt.Println()
			return
		default:
			if !strings.HasPrefix(a, "-") {
				locationArg = a
			}
		}
	}

	clearScreen()
	printHeader()

	if locationArg == "" {
		fmt.Println()
		fmt.Print(bold(BrightCyan, "  > ENTER LOCATION (city or lat,lon): "))
		scanner := bufio.NewScanner(os.Stdin)
		if scanner.Scan() {
			locationArg = strings.TrimSpace(scanner.Text())
		}
		if locationArg == "" {
			fmt.Println(col(Red, "  No location provided."))
			os.Exit(1)
		}
	}

	var lat, lon float64
	var locationName string
	isLatLon := false

	// Try lat,lon
	parts := strings.SplitN(locationArg, ",", 2)
	if len(parts) == 2 {
		la, err1 := strconv.ParseFloat(strings.TrimSpace(parts[0]), 64)
		lo, err2 := strconv.ParseFloat(strings.TrimSpace(parts[1]), 64)
		if err1 == nil && err2 == nil && la >= -90 && la <= 90 && lo >= -180 && lo <= 180 {
			lat, lon = la, lo
			locationName = fmt.Sprintf("%.4f, %.4f", lat, lon)
			isLatLon = true
		}
	}

	if !isLatLon {
		stopLoad := startLoading("SEARCHING: " + strings.ToUpper(locationArg) + " ...")
		results, err := geocode(locationArg)
		stopLoad() // BUG FIX: spinner fully stopped before any output below

		if err != nil {
			fmt.Println(col(Red, "\n  ERROR: "+err.Error()))
			os.Exit(1)
		}
		if len(results) == 0 {
			fmt.Println(col(Red, "\n  Location not found: "+locationArg))
			fmt.Println(col(BrightBlack, "  Tip: try a different spelling, or use lat,lon (e.g. 51.5,-0.12)"))
			os.Exit(1)
		}
		chosen := selectLocation(results)
		if chosen == nil {
			os.Exit(1)
		}
		lat = chosen.Latitude
		lon = chosen.Longitude
		ps := []string{chosen.Name}
		if chosen.Admin1 != "" {
			ps = append(ps, chosen.Admin1)
		}
		if chosen.Country != "" {
			ps = append(ps, chosen.Country)
		}
		locationName = strings.Join(ps, ", ")
	}

	stopLoad := startLoading("DOWNLOADING WEATHER FOR " + strings.ToUpper(locationName) + " ...")
	wr, err := fetchWeather(lat, lon)
	stopLoad() // BUG FIX: spinner stopped before rendering output

	if err != nil {
		fmt.Println(col(Red, "\n  ERROR: "+err.Error()))
		os.Exit(1)
	}

	fmt.Println()
	printCurrent(wr, locationName)
	printWeekly(wr)

	if showHourly {
		printHourly(wr)
	}
	if showStats {
		printStats(wr)
	}

	fmt.Println()
	fmt.Println(col(BrightBlack, "  [ data: open-meteo.com | free | no api key | flags: --hourly --stats --all ]"))
	fmt.Println()
}

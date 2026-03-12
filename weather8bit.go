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
	"time"
)

// ─── ANSI COLORS ───────────────────────────────────────────────────────────────

const (
	Reset       = "\033[0m"
	Bold        = "\033[1m"
	Dim         = "\033[2m"
	Black       = "\033[30m"
	Red         = "\033[31m"
	Green       = "\033[32m"
	Yellow      = "\033[33m"
	Blue        = "\033[34m"
	Magenta     = "\033[35m"
	Cyan        = "\033[36m"
	White       = "\033[37m"
	BrightBlack = "\033[90m"
	BrightRed   = "\033[91m"
	BrightGreen = "\033[92m"
	BrightYellow = "\033[93m"
	BrightBlue  = "\033[94m"
	BrightMagenta = "\033[95m"
	BrightCyan  = "\033[96m"
	BrightWhite = "\033[97m"
	BgBlack     = "\033[40m"
	BgGrey      = "\033[100m"
)

func c(color, text string) string { return color + text + Reset }
func cb(color, text string) string { return Bold + color + text + Reset }

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

func pixelText(text string, color string) string {
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
	var out strings.Builder
	for i := 0; i < 5; i++ {
		out.WriteString(color + rows[i].String() + Reset + "\n")
	}
	return out.String()
}

// ─── WEATHER ICONS ─────────────────────────────────────────────────────────────

type IconDef struct {
	lines [5]string
	color string
}

var icons = map[string]IconDef{
	"sunny":         {[5]string{"  \\  |  /  ", "   \\ | /   ", "----[☀]----", "   / | \\   ", "  /  |  \\  "}, BrightYellow},
	"partly_cloudy": {[5]string{"  \\  |  /  ", "    \\ | /  ", "  -[☀]▄▄▄  ", "   ▄██████▄", "  ▀▀▀▀▀▀▀▀ "}, Yellow},
	"cloudy":        {[5]string{"           ", "   ▄▄▄▄▄   ", " ▄███████▄ ", "▀▀▀▀▀▀▀▀▀▀▀", "           "}, White},
	"rainy":         {[5]string{"   ▄▄▄▄▄   ", " ▄███████▄ ", "███████████", "▀▀▀▀▀▀▀▀▀▀▀", " ▌ ▌ ▌ ▌ ▌ "}, Cyan},
	"heavy_rain":    {[5]string{"   ▄▄▄▄▄   ", " ▄███████▄ ", "███████████", "▀▀▀▀▀▀▀▀▀▀▀", "▌▌▌▌▌▌▌▌▌▌▌"}, Blue},
	"snowy":         {[5]string{"   ▄▄▄▄▄   ", " ▄███████▄ ", "███████████", "▀▀▀▀▀▀▀▀▀▀▀", " * * * * * "}, BrightWhite},
	"thunderstorm":  {[5]string{"   ▄▄▄▄▄   ", "▄██████████", "███████████", "▀▀▀▄▀▀▀▀▀▀▀", "   ▌▄      "}, BrightYellow},
	"foggy":         {[5]string{" ─ ─ ─ ─ ─ ", "─ ─ ─ ─ ─ ─", " ─ ─ ─ ─ ─ ", "─ ─ ─ ─ ─ ─", " ─ ─ ─ ─ ─ "}, BrightBlack},
	"night_clear":   {[5]string{"    *   .  ", "  .   ☽    ", "     .   * ", "  *   .    ", "    .   *  "}, BrightBlack},
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
	57: {"Heavy Frz Drizzle", "snowy", BrightCyan},
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

// ─── TERMINAL WIDTH ────────────────────────────────────────────────────────────

func termWidth() int {
	if runtime.GOOS == "windows" {
		return 120
	}
	cmd := exec.Command("stty", "size")
	cmd.Stdin = os.Stdin
	out, err := cmd.Output()
	if err != nil {
		return 120
	}
	parts := strings.Fields(strings.TrimSpace(string(out)))
	if len(parts) == 2 {
		if w, err := strconv.Atoi(parts[1]); err == nil {
			return w
		}
	}
	return 120
}

func repeat(s string, n int) string {
	if n <= 0 {
		return ""
	}
	return strings.Repeat(s, n)
}

func padRight(s string, width int) string {
	// visible length (strip ANSI)
	vis := visLen(s)
	if vis >= width {
		return s
	}
	return s + strings.Repeat(" ", width-vis)
}

func visLen(s string) int {
	inEsc := false
	n := 0
	for _, r := range s {
		if r == '\033' {
			inEsc = true
		} else if inEsc && r == 'm' {
			inEsc = false
		} else if !inEsc {
			n++
		}
	}
	return n
}

// ─── BARS ──────────────────────────────────────────────────────────────────────

func bar(filled, total int, fillChar, emptyChar, color string) string {
	if filled > total {
		filled = total
	}
	if filled < 0 {
		filled = 0
	}
	return color + strings.Repeat(fillChar, filled) + BrightBlack + strings.Repeat(emptyChar, total-filled) + Reset
}

func humidityBar(h float64) string {
	filled := int(h / 10)
	col := Cyan
	if h >= 80 {
		col = BrightBlue
	} else if h >= 60 {
		col = Blue
	}
	return bar(filled, 10, "█", "░", col) + " " + fmt.Sprintf("%.0f%%", h)
}

func precipBar(pct float64) string {
	filled := int(pct / 10)
	col := Cyan
	if pct > 50 {
		col = Blue
	}
	return bar(filled, 10, "█", "░", col) + " " + fmt.Sprintf("%.0f%%", pct)
}

func uvBar(uv float64) string {
	filled := int(math.Min(uv, 12))
	col := Green
	if uv > 10 {
		col = BrightMagenta
	} else if uv > 7 {
		col = Red
	} else if uv > 5 {
		col = BrightRed
	} else if uv > 2 {
		col = Yellow
	}
	return bar(filled, 12, "█", "░", col) + " " + fmt.Sprintf("%.1f", uv)
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
	dirs := []string{"↑N", "↗NE", "→E", "↘SE", "↓S", "↙SW", "←W", "↖NW"}
	idx := int(math.Round(deg/45)) % 8
	if idx < 0 {
		idx += 8
	}
	return dirs[idx]
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
	Temperature2m        float64 `json:"temperature_2m"`
	RelativeHumidity2m   float64 `json:"relative_humidity_2m"`
	ApparentTemperature  float64 `json:"apparent_temperature"`
	IsDay                int     `json:"is_day"`
	Precipitation        float64 `json:"precipitation"`
	WeatherCode          int     `json:"weather_code"`
	CloudCover           float64 `json:"cloud_cover"`
	PressureMsl          float64 `json:"pressure_msl"`
	WindSpeed10m         float64 `json:"wind_speed_10m"`
	WindDirection10m     float64 `json:"wind_direction_10m"`
	WindGusts10m         float64 `json:"wind_gusts_10m"`
	Visibility           float64 `json:"visibility"`
	UvIndex              float64 `json:"uv_index"`
}

type DailyWeather struct {
	Time                       []string  `json:"time"`
	WeatherCode                []int     `json:"weather_code"`
	Temperature2mMax           []float64 `json:"temperature_2m_max"`
	Temperature2mMin           []float64 `json:"temperature_2m_min"`
	ApparentTemperatureMax     []float64 `json:"apparent_temperature_max"`
	ApparentTemperatureMin     []float64 `json:"apparent_temperature_min"`
	Sunrise                    []string  `json:"sunrise"`
	Sunset                     []string  `json:"sunset"`
	PrecipitationSum           []float64 `json:"precipitation_sum"`
	PrecipitationProbabilityMax []float64 `json:"precipitation_probability_max"`
	WindSpeed10mMax            []float64 `json:"wind_speed_10m_max"`
	WindGusts10mMax            []float64 `json:"wind_gusts_10m_max"`
	WindDirection10mDominant   []float64 `json:"wind_direction_10m_dominant"`
	UvIndexMax                 []float64 `json:"uv_index_max"`
	RelativeHumidity2mMax      []float64 `json:"relative_humidity_2m_max"`
}

type HourlyWeather struct {
	Time                        []string  `json:"time"`
	Temperature2m               []float64 `json:"temperature_2m"`
	RelativeHumidity2m          []float64 `json:"relative_humidity_2m"`
	PrecipitationProbability    []float64 `json:"precipitation_probability"`
	Precipitation               []float64 `json:"precipitation"`
	WeatherCode                 []int     `json:"weather_code"`
	WindSpeed10m                []float64 `json:"wind_speed_10m"`
	Visibility                  []float64 `json:"visibility"`
}

type WeatherResponse struct {
	Current  CurrentWeather `json:"current"`
	Daily    DailyWeather   `json:"daily"`
	Hourly   HourlyWeather  `json:"hourly"`
	Timezone string         `json:"timezone"`
}

// ─── API CALLS ─────────────────────────────────────────────────────────────────

func geocode(location string) ([]GeoResult, error) {
	u := "https://geocoding-api.open-meteo.com/v1/search?name=" +
		url.QueryEscape(location) + "&count=5&language=en&format=json"
	resp, err := http.Get(u)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	var gr GeoResponse
	json.Unmarshal(body, &gr)
	return gr.Results, nil
}

func fetchWeather(lat, lon float64) (*WeatherResponse, error) {
	params := fmt.Sprintf(
		"https://api.open-meteo.com/v1/forecast?latitude=%.4f&longitude=%.4f"+
			"&current=temperature_2m,relative_humidity_2m,apparent_temperature,is_day,precipitation,weather_code,cloud_cover,pressure_msl,wind_speed_10m,wind_direction_10m,wind_gusts_10m,visibility,uv_index"+
			"&hourly=temperature_2m,relative_humidity_2m,precipitation_probability,precipitation,weather_code,wind_speed_10m,visibility"+
			"&daily=weather_code,temperature_2m_max,temperature_2m_min,apparent_temperature_max,apparent_temperature_min,sunrise,sunset,precipitation_sum,precipitation_probability_max,wind_speed_10m_max,wind_gusts_10m_max,wind_direction_10m_dominant,uv_index_max,relative_humidity_2m_max"+
			"&timezone=auto&forecast_days=7&wind_speed_unit=kmh",
		lat, lon,
	)
	resp, err := http.Get(params)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	var wr WeatherResponse
	json.Unmarshal(body, &wr)
	return &wr, nil
}

// ─── RENDERING ─────────────────────────────────────────────────────────────────

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
	fmt.Println(cb(BrightCyan, "  ██╗    ██╗███████╗ █████╗ ████████╗██╗  ██╗███████╗██████╗  █████╗  "))
	fmt.Println(cb(Cyan,       "  ██║    ██║██╔════╝██╔══██╗╚══██╔══╝██║  ██║██╔════╝██╔══██╗██╔══██╗ "))
	fmt.Println(cb(BrightCyan, "  ██║ █╗ ██║█████╗  ███████║   ██║   ███████║█████╗  ██████╔╝╚█████╔╝ "))
	fmt.Println(cb(Cyan,       "  ██║███╗██║██╔══╝  ██╔══██║   ██║   ██╔══██║██╔══╝  ██╔══██╗██╔══██╗ "))
	fmt.Println(cb(BrightCyan, "  ╚███╔███╔╝███████╗██║  ██║   ██║   ██║  ██║███████╗██║  ██║╚█████╔╝ "))
	fmt.Println(cb(Cyan,       "   ╚══╝╚══╝ ╚══════╝╚═╝  ╚═╝   ╚═╝   ╚═╝  ╚═╝╚══════╝╚═╝  ╚═╝ ╚════╝  "))
	fmt.Println(cb(BrightWhite, "              ▓ 8-BIT WORLD WEATHER CLI ▓  FREE · NO API KEY ▓         "))
	w := termWidth()
	fmt.Println(c(BrightCyan, "  ╔"+repeat("═", w-4)+"╗"))
	fmt.Println(c(BrightCyan, "  ╚"+repeat("═", w-4)+"╝"))
}

func printRule(title string) {
	w := termWidth()
	total := w - 4
	inner := " " + title + " "
	vis := visLen(inner)
	left := (total - vis) / 2
	right := total - vis - left
	fmt.Println()
	fmt.Println(c(BrightCyan, "  ╟"+repeat("─", left)) + cb(BrightCyan, inner) + c(BrightCyan, repeat("─", right)+"╢"))
}

func safeFloat(arr []float64, i int) float64 {
	if i < len(arr) {
		return arr[i]
	}
	return 0
}

func safeInt(arr []int, i int) int {
	if i < len(arr) {
		return arr[i]
	}
	return 0
}

func safeString(arr []string, i int) string {
	if i < len(arr) {
		return arr[i]
	}
	return ""
}

func printCurrent(wr *WeatherResponse, locationName string) {
	cur := wr.Current
	wi := getWMO(cur.WeatherCode)
	icon := icons[wi.iconKey]

	if cur.IsDay == 0 && wi.iconKey == "sunny" {
		icon = icons["night_clear"]
	}

	tc := tempColor(cur.Temperature2m)
	tempStr := fmt.Sprintf("%.0f°C", cur.Temperature2m)
	pixTemp := pixelText(tempStr, tc)

	fmt.Println()
	printRule("▓ CURRENT CONDITIONS ▓")
	fmt.Println()

	// Print icon + pixel temp side by side with info
	iconLines := icon.lines[:]
	pixLines := strings.Split(strings.TrimRight(pixTemp, "\n"), "\n")

	// Build info lines
	infoLines := []string{
		cb(BrightWhite, "  LOCATION   ") + c(BrightCyan, locationName),
		cb(BrightWhite, "  CONDITION  ") + c(wi.color, wi.desc),
		cb(BrightWhite, "  FEELS LIKE ") + c(tc, fmt.Sprintf("%.1f°C", cur.ApparentTemperature)),
		cb(BrightWhite, "  HUMIDITY   ") + humidityBar(cur.RelativeHumidity2m),
		cb(BrightWhite, "  PRECIP     ") + c(Cyan, fmt.Sprintf("%.1f mm", cur.Precipitation)),
		cb(BrightWhite, "  PRESSURE   ") + c(Green, fmt.Sprintf("%.1f hPa", cur.PressureMsl)),
		cb(BrightWhite, "  VISIBILITY ") + c(Yellow, fmt.Sprintf("%.1f km", cur.Visibility/1000)),
		cb(BrightWhite, "  WIND       ") + c(BrightGreen, fmt.Sprintf("%.0f km/h %s  gusts %.0f", cur.WindSpeed10m, windArrow(cur.WindDirection10m), cur.WindGusts10m)),
		cb(BrightWhite, "  CLOUD      ") + c(White, fmt.Sprintf("%.0f%%", cur.CloudCover)),
		cb(BrightWhite, "  UV INDEX   ") + uvBar(cur.UvIndex),
		cb(BrightWhite, "  TIME       ") + c(BrightBlack, time.Now().Format("Mon 02 Jan 2006  15:04:05")),
	}

	maxLines := len(iconLines)
	if len(pixLines) > maxLines {
		maxLines = len(pixLines)
	}
	if len(infoLines) > maxLines {
		maxLines = len(infoLines)
	}

	for i := 0; i < maxLines; i++ {
		line := "  "
		if i < len(iconLines) {
			line += c(icon.color, fmt.Sprintf("%-13s", iconLines[i]))
		} else {
			line += strings.Repeat(" ", 13)
		}
		line += "  "
		if i < len(pixLines) {
			line += fmt.Sprintf("%-30s", pixLines[i])
		} else {
			line += strings.Repeat(" ", 30)
		}
		line += "  "
		if i < len(infoLines) {
			line += infoLines[i]
		}
		fmt.Println(line)
	}
}

func printWeekly(wr *WeatherResponse) {
	printRule("▓ 7-DAY FORECAST ▓")
	fmt.Println()

	daily := wr.Daily
	days := daily.Time
	if len(days) == 0 {
		return
	}

	w := termWidth()
	colW := (w - 4) / len(days)
	if colW < 16 {
		colW = 16
	}

	// Build panel content for each day (collect lines per day)
	type DayPanel struct {
		lines []string
		color string
	}
	panels := make([]DayPanel, len(days))

	for i, dateStr := range days {
		dt, _ := time.Parse("2006-01-02", dateStr)
		dayName := strings.ToUpper(dt.Format("Mon"))
		dayNum := dt.Format("02")
		month := strings.ToUpper(dt.Format("Jan"))

		wc := safeInt(daily.WeatherCode, i)
		wi := getWMO(wc)
		icon := icons[wi.iconKey]

		tMax := safeFloat(daily.Temperature2mMax, i)
		tMin := safeFloat(daily.Temperature2mMin, i)
		precip := safeFloat(daily.PrecipitationSum, i)
		precipPct := safeFloat(daily.PrecipitationProbabilityMax, i)
		wind := safeFloat(daily.WindSpeed10mMax, i)
		humidity := safeFloat(daily.RelativeHumidity2mMax, i)
		uv := safeFloat(daily.UvIndexMax, i)

		sunriseRaw := safeString(daily.Sunrise, i)
		sunsetRaw := safeString(daily.Sunset, i)
		sr, ss := "?", "?"
		if idx := strings.Index(sunriseRaw, "T"); idx >= 0 && len(sunriseRaw) > idx+5 {
			sr = sunriseRaw[idx+1 : idx+6]
		}
		if idx := strings.Index(sunsetRaw, "T"); idx >= 0 && len(sunsetRaw) > idx+5 {
			ss = sunsetRaw[idx+1 : idx+6]
		}

		maxC := tempColor(tMax)
		minC := tempColor(tMin)
		uvC := Green
		if uv > 10 {
			uvC = BrightMagenta
		} else if uv > 7 {
			uvC = Red
		} else if uv > 5 {
			uvC = Yellow
		} else if uv > 2 {
			uvC = BrightYellow
		}

		marker := ""
		if i == 0 {
			marker = cb(BrightYellow, "★TODAY")
		} else {
			marker = cb(BrightBlack, "      ")
		}

		var lines []string
		lines = append(lines, marker)
		lines = append(lines, cb(BrightWhite, dayName+" "+dayNum+" "+month))
		lines = append(lines, c(BrightBlack, "─────────────"))
		for _, l := range icon.lines {
			lines = append(lines, c(wi.color, l))
		}
		lines = append(lines, c(wi.color, wi.desc))
		lines = append(lines, "")
		lines = append(lines, c(maxC, fmt.Sprintf("▲ %+.0f°C", tMax))+"  "+c(minC, fmt.Sprintf("▼ %+.0f°C", tMin)))
		lines = append(lines, c(Cyan, fmt.Sprintf("💧 %.1fmm", precip))+"  "+c(Blue, fmt.Sprintf("%.0f%%", precipPct)))
		lines = append(lines, c(BrightGreen, fmt.Sprintf("🌬 %.0f km/h", wind)))
		lines = append(lines, c(Cyan, fmt.Sprintf("💦 Hum %.0f%%", humidity)))
		lines = append(lines, c(uvC, fmt.Sprintf("☀  UV %.0f", uv)))
		lines = append(lines, c(Yellow, "🌅 "+sr)+"  "+c(Yellow, "🌇 "+ss))

		panels[i] = DayPanel{lines: lines, color: wi.color}
	}

	// Find max lines
	maxL := 0
	for _, p := range panels {
		if len(p.lines) > maxL {
			maxL = len(p.lines)
		}
	}

	// Print side by side
	borderChar := "│"
	// top border
	topLine := "  ┌"
	for i := range panels {
		topLine += repeat("─", colW)
		if i < len(panels)-1 {
			topLine += "┬"
		}
	}
	topLine += "┐"
	fmt.Println(c(BrightCyan, topLine))

	for row := 0; row < maxL; row++ {
		line := "  " + c(BrightCyan, borderChar)
		for _, p := range panels {
			cell := ""
			if row < len(p.lines) {
				cell = " " + p.lines[row]
			}
			padded := padRight(cell, colW)
			// trim if too long
			if visLen(padded) > colW {
				padded = cell[:colW-1] + " "
			}
			line += padded + c(BrightCyan, borderChar)
		}
		fmt.Println(line)
	}

	// bottom border
	botLine := "  └"
	for i := range panels {
		botLine += repeat("─", colW)
		if i < len(panels)-1 {
			botLine += "┴"
		}
	}
	botLine += "┘"
	fmt.Println(c(BrightCyan, botLine))
}

func printHourly(wr *WeatherResponse) {
	printRule("▓ HOURLY FORECAST (NEXT 24H) ▓")
	fmt.Println()

	hourly := wr.Hourly
	now := time.Now()

	// Header
	fmt.Println("  " + cb(BrightCyan,
		fmt.Sprintf("%-7s  %-18s  %-9s  %-14s  %-12s  %-7s  %-8s  %-7s",
			"TIME", "CONDITION", "TEMP", "HUMIDITY", "PRECIP %", "RAIN", "WIND", "VIS")))
	fmt.Println("  " + c(BrightBlack, repeat("─", 96)))

	shown := 0
	for i, ts := range hourly.Time {
		dt, err := time.ParseInLocation("2006-01-02T15:04", ts, now.Location())
		if err != nil {
			continue
		}
		if dt.Before(now.Add(-time.Hour)) {
			continue
		}
		if shown >= 24 {
			break
		}

		wc := safeInt(hourly.WeatherCode, i)
		wi := getWMO(wc)
		temp := safeFloat(hourly.Temperature2m, i)
		hum := safeFloat(hourly.RelativeHumidity2m, i)
		pp := safeFloat(hourly.PrecipitationProbability, i)
		rain := safeFloat(hourly.Precipitation, i)
		wind := safeFloat(hourly.WindSpeed10m, i)
		vis := safeFloat(hourly.Visibility, i) / 1000
		tc := tempColor(temp)

		timeStr := dt.Format("15:04")
		if dt.Day() != now.Day() {
			timeStr = c(BrightBlack, dt.Format("Mon")+dt.Format(" 15:04"))
		}

		bgToggle := ""
		if shown%2 == 1 {
			bgToggle = BgBlack
		}

		desc := wi.desc
		if len(desc) > 16 {
			desc = desc[:16]
		}

		row := fmt.Sprintf("  "+bgToggle+"%-7s  %s%-18s%s  %s%-9s%s  %-14s  %-12s  %s%-7s%s  %s%-8s%s  %s%-7s%s"+Reset,
			timeStr,
			wi.color, desc, Reset,
			tc, fmt.Sprintf("%.1f°C", temp), Reset,
			humidityBar(hum),
			precipBar(pp),
			Cyan, fmt.Sprintf("%.1fmm", rain), Reset,
			BrightGreen, fmt.Sprintf("%.0fkm/h", wind), Reset,
			Yellow, fmt.Sprintf("%.1fkm", vis), Reset,
		)
		fmt.Println(row)
		shown++
	}
}

func printStats(wr *WeatherResponse) {
	printRule("▓ WEEKLY STATISTICS ▓")
	fmt.Println()

	d := wr.Daily
	if len(d.Time) == 0 {
		return
	}

	minMaxAvg := func(arr []float64) (float64, float64, float64) {
		if len(arr) == 0 {
			return 0, 0, 0
		}
		mn, mx, sum := arr[0], arr[0], 0.0
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

	tMaxSlice := d.Temperature2mMax
	tMinSlice := d.Temperature2mMin
	allTemps := append(append([]float64{}, tMaxSlice...), tMinSlice...)

	type StatRow struct {
		label, minS, maxS, avgS, totalS string
	}
	rows := []StatRow{}

	if len(allTemps) > 0 {
		mn, mx, avg := minMaxAvg(allTemps)
		rows = append(rows, StatRow{
			label: "TEMPERATURE",
			minS:  c(BrightCyan, fmt.Sprintf("%.1f°C", mn)),
			maxS:  c(BrightRed, fmt.Sprintf("%.1f°C", mx)),
			avgS:  c(Yellow, fmt.Sprintf("%.1f°C", avg)),
			totalS: c(BrightBlack, "—"),
		})
	}
	if len(d.PrecipitationSum) > 0 {
		mn, mx, avg := minMaxAvg(d.PrecipitationSum)
		total := 0.0
		for _, v := range d.PrecipitationSum {
			total += v
		}
		rows = append(rows, StatRow{
			label:  "PRECIPITATION",
			minS:   c(Cyan, fmt.Sprintf("%.1fmm", mn)),
			maxS:   c(Blue, fmt.Sprintf("%.1fmm", mx)),
			avgS:   c(Yellow, fmt.Sprintf("%.1fmm", avg)),
			totalS: c(Green, fmt.Sprintf("%.1fmm", total)),
		})
	}
	if len(d.WindSpeed10mMax) > 0 {
		mn, mx, avg := minMaxAvg(d.WindSpeed10mMax)
		rows = append(rows, StatRow{
			label: "WIND SPEED",
			minS:  c(BrightGreen, fmt.Sprintf("%.0fkm/h", mn)),
			maxS:  c(BrightRed, fmt.Sprintf("%.0fkm/h", mx)),
			avgS:  c(Yellow, fmt.Sprintf("%.0fkm/h", avg)),
			totalS: c(BrightBlack, "—"),
		})
	}
	if len(d.UvIndexMax) > 0 {
		mn, mx, avg := minMaxAvg(d.UvIndexMax)
		rows = append(rows, StatRow{
			label: "UV INDEX",
			minS:  c(Green, fmt.Sprintf("%.1f", mn)),
			maxS:  c(BrightRed, fmt.Sprintf("%.1f", mx)),
			avgS:  c(Yellow, fmt.Sprintf("%.1f", avg)),
			totalS: c(BrightBlack, "—"),
		})
	}

	hdr := fmt.Sprintf("  %-20s  %-20s  %-20s  %-20s  %-20s",
		cb(BrightCyan, "METRIC"),
		cb(BrightCyan, "MIN"),
		cb(BrightCyan, "MAX"),
		cb(BrightCyan, "AVERAGE"),
		cb(BrightCyan, "TOTAL"),
	)
	fmt.Println(hdr)
	fmt.Println("  " + c(BrightBlack, repeat("─", 100)))
	for _, r := range rows {
		fmt.Printf("  %-29s  %-29s  %-29s  %-29s  %-29s\n",
			cb(BrightWhite, r.label), r.minS, r.maxS, r.avgS, r.totalS)
	}
}

// ─── LOCATION SELECT ───────────────────────────────────────────────────────────

func selectLocation(results []GeoResult) *GeoResult {
	if len(results) == 0 {
		return nil
	}
	if len(results) == 1 {
		return &results[0]
	}

	fmt.Println()
	printRule("▓ MULTIPLE LOCATIONS FOUND ▓")
	fmt.Println()
	for i, r := range results {
		if i >= 5 {
			break
		}
		fmt.Printf("  %s  %-20s  %-15s  %-15s  %s\n",
			cb(BrightYellow, fmt.Sprintf("[%d]", i+1)),
			cb(BrightWhite, r.Name),
			c(Yellow, r.Admin1),
			c(Green, r.Country),
			c(BrightBlack, fmt.Sprintf("%.2f, %.2f", r.Latitude, r.Longitude)),
		)
	}
	fmt.Println()
	fmt.Print(cb(BrightCyan, "  ▶ SELECT (1-5): "))
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		text := strings.TrimSpace(scanner.Text())
		idx, err := strconv.Atoi(text)
		if err == nil && idx >= 1 && idx <= min(len(results), 5) {
			return &results[idx-1]
		}
		fmt.Print(c(Red, "  INVALID — ") + cb(BrightCyan, "▶ SELECT (1-5): "))
	}
	return &results[0]
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func loading(msg string) {
	frames := []string{"⣾", "⣽", "⣻", "⢿", "⡿", "⣟", "⣯", "⣷"}
	for i := 0; i < 12; i++ {
		fmt.Printf("\r  %s  %s", c(BrightCyan, frames[i%len(frames)]), cb(BrightCyan, msg))
		time.Sleep(80 * time.Millisecond)
	}
	fmt.Print("\r" + strings.Repeat(" ", 60) + "\r")
}

// ─── MAIN ──────────────────────────────────────────────────────────────────────

func printUsage() {
	fmt.Println()
	fmt.Println(cb(BrightWhite, "  USAGE:"))
	fmt.Println(c(BrightCyan,   "    weather8bit <location> [options]"))
	fmt.Println()
	fmt.Println(cb(BrightWhite, "  OPTIONS:"))
	fmt.Println("    " + c(Yellow, "--hourly") + "  " + c(BrightBlack, "Show 24-hour forecast"))
	fmt.Println("    " + c(Yellow, "--stats") + "   " + c(BrightBlack, "Show weekly statistics"))
	fmt.Println("    " + c(Yellow, "--all") + "     " + c(BrightBlack, "Show everything"))
	fmt.Println()
	fmt.Println(cb(BrightWhite, "  EXAMPLES:"))
	fmt.Println("    " + c(Green, `weather8bit "Tokyo"`))
	fmt.Println("    " + c(Green, `weather8bit "New York" --all`))
	fmt.Println("    " + c(Green, `weather8bit "48.8566,2.3522"`))
	fmt.Println()
}

func main() {
	args := os.Args[1:]
	showHourly := false
	showStats := false
	locationArg := ""

	for _, a := range args {
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
			printUsage()
			return
		default:
			locationArg = a
		}
	}

	clearScreen()
	printHeader()

	if locationArg == "" {
		fmt.Println()
		fmt.Print(cb(BrightCyan, "  ▶ ENTER LOCATION (city or lat,lon): "))
		scanner := bufio.NewScanner(os.Stdin)
		if scanner.Scan() {
			locationArg = strings.TrimSpace(scanner.Text())
		}
		if locationArg == "" {
			fmt.Println(c(Red, "  No location provided."))
			os.Exit(1)
		}
	}

	var lat, lon float64
	var locationName string
	isLatLon := false

	// Try parsing as lat,lon
	parts := strings.SplitN(locationArg, ",", 2)
	if len(parts) == 2 {
		la, err1 := strconv.ParseFloat(strings.TrimSpace(parts[0]), 64)
		lo, err2 := strconv.ParseFloat(strings.TrimSpace(parts[1]), 64)
		if err1 == nil && err2 == nil && la >= -90 && la <= 90 && lo >= -180 && lo <= 180 {
			lat, lon = la, lo
			locationName = fmt.Sprintf("%.4f°, %.4f°", lat, lon)
			isLatLon = true
		}
	}

	if !isLatLon {
		go loading("SEARCHING: " + strings.ToUpper(locationArg) + " ...")
		results, err := geocode(locationArg)
		if err != nil || len(results) == 0 {
			fmt.Println(c(Red, "\n  ERROR: Location not found — "+locationArg))
			os.Exit(1)
		}
		chosen := selectLocation(results)
		if chosen == nil {
			os.Exit(1)
		}
		lat = chosen.Latitude
		lon = chosen.Longitude
		parts := []string{chosen.Name}
		if chosen.Admin1 != "" {
			parts = append(parts, chosen.Admin1)
		}
		if chosen.Country != "" {
			parts = append(parts, chosen.Country)
		}
		locationName = strings.Join(parts, ", ")
	}

	loading("DOWNLOADING WEATHER FOR " + strings.ToUpper(locationName) + " ...")

	wr, err := fetchWeather(lat, lon)
	if err != nil || wr == nil {
		fmt.Println(c(Red, "\n  ERROR: Could not fetch weather data"))
		os.Exit(1)
	}

	printCurrent(wr, locationName)
	printWeekly(wr)

	if showHourly {
		printHourly(wr)
	}
	if showStats {
		printStats(wr)
	}

	// Footer
	fmt.Println()
	w := termWidth()
	footer := fmt.Sprintf("  DATA: open-meteo.com  |  FREE & NO API KEY  |  FLAGS: --hourly --stats --all")
	pad := (w - visLen(footer)) / 2
	fmt.Println(strings.Repeat(" ", pad) + c(BrightBlack, footer))
	fmt.Println()
}

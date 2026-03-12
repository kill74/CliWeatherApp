package main

import (
	"fmt"
	"math"
	"math/rand"
	"os"
	"os/exec"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// ─── TERMINAL HEIGHT ──────────────────────────────────────────────────────────

func termHeight() int {
	cmd := exec.Command("stty", "size")
	cmd.Stdin = os.Stdin
	if out, err := cmd.Output(); err == nil {
		parts := strings.Fields(strings.TrimSpace(string(out)))
		if len(parts) == 2 {
			if h, err := strconv.Atoi(parts[0]); err == nil && h > 5 {
				return h
			}
		}
	}
	return 40
}

// ─── GRID (double-buffer cell renderer) ───────────────────────────────────────

type Cell struct {
	ch    rune
	color string
}

type Grid struct {
	cells [][]Cell
	w, h  int
}

func newGrid(w, h int) *Grid {
	g := &Grid{w: w, h: h}
	g.cells = make([][]Cell, h)
	for y := range g.cells {
		g.cells[y] = make([]Cell, w)
	}
	return g
}

func (g *Grid) clear() {
	for y := range g.cells {
		for x := range g.cells[y] {
			g.cells[y][x] = Cell{' ', Reset}
		}
	}
}

func (g *Grid) set(x, y int, ch rune, color string) {
	if x >= 0 && x < g.w && y >= 0 && y < g.h {
		g.cells[y][x] = Cell{ch, color}
	}
}

// txt writes plain ASCII text into the grid (no ANSI in the string itself)
func (g *Grid) txt(x, y int, s string, color string) {
	for i, ch := range s {
		g.set(x+i, y, ch, color)
	}
}

func (g *Grid) box(x, y, w, h int, bc string) {
	if w < 2 || h < 2 {
		return
	}
	g.set(x, y, '+', bc)
	g.set(x+w-1, y, '+', bc)
	g.set(x, y+h-1, '+', bc)
	g.set(x+w-1, y+h-1, '+', bc)
	for i := 1; i < w-1; i++ {
		g.set(x+i, y, '-', bc)
		g.set(x+i, y+h-1, '-', bc)
	}
	for j := 1; j < h-1; j++ {
		g.set(x, y+j, '|', bc)
		g.set(x+w-1, y+j, '|', bc)
		for i := 1; i < w-1; i++ {
			g.set(x+i, y+j, ' ', Reset)
		}
	}
}

// render writes the entire grid as one string (single write = minimum flicker)
func (g *Grid) render(buf *strings.Builder) {
	buf.WriteString("\033[H") // cursor to home, no clear = less flicker
	for y := 0; y < g.h; y++ {
		lastColor := ""
		for x := 0; x < g.w; x++ {
			c := g.cells[y][x]
			if c.color != lastColor {
				if lastColor != "" {
					buf.WriteString(Reset)
				}
				if c.color != Reset && c.color != "" {
					buf.WriteString(c.color)
				}
				lastColor = c.color
			}
			buf.WriteRune(c.ch)
		}
		buf.WriteString(Reset)
		if y < g.h-1 {
			buf.WriteByte('\n')
		}
	}
}

// ─── PARTICLE ─────────────────────────────────────────────────────────────────

type Particle struct {
	x, y   float64
	vx, vy float64
	ch     rune
	color  string
	life   int
}

func (p *Particle) step() {
	p.x += p.vx
	p.y += p.vy
	p.life--
}

// ─── CLOUD OBJECT ─────────────────────────────────────────────────────────────

type cloud struct {
	x, y  float64
	speed float64
	rows  []string
}

var cloudShapes = [][]string{
	{" .---. ", "(     )", " '---' "},
	{"  .----.  ", " (      ) ", "  '----'  "},
	{" .--.  ", "(    ) ", " '--' "},
	{"  .-~-.  ", " (     ) ", "  '-~-'  "},
}

func makeCloud(w int, rng *rand.Rand, startX float64) cloud {
	shape := cloudShapes[rng.Intn(len(cloudShapes))]
	return cloud{
		x:     startX,
		y:     float64(1 + rng.Intn(7)),
		speed: 0.04 + rng.Float64()*0.07,
		rows:  shape,
	}
}

// ─── LIVE SCENE ───────────────────────────────────────────────────────────────

type LiveScene struct {
	g         *Grid
	particles []*Particle
	clouds    []cloud
	frame     int
	lightning int
	lightX    int
	sunAngle  float64
	wr        *WeatherResponse
	location  string
	rng       *rand.Rand
}

func maxI(a, b int) int {
	if a > b {
		return a
	}
	return b
}
func minI(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// ─── SPAWN ────────────────────────────────────────────────────────────────────

func (s *LiveScene) spawnRain(n, wcode int) {
	drift := s.wr.Current.WindSpeed10m * 0.007
	if s.wr.Current.WindDirection10m > 180 {
		drift = -drift
	}
	heavy := wcode == 65 || wcode == 82 || wcode >= 95
	c := Cyan
	vy := 0.55 + s.rng.Float64()*0.35
	if heavy {
		c = Blue
		vy += 0.35
	}
	for i := 0; i < n; i++ {
		s.particles = append(s.particles, &Particle{
			x: float64(s.rng.Intn(s.g.w)), y: -float64(s.rng.Intn(s.g.h)),
			vx: drift, vy: vy,
			ch: '|', color: c, life: s.g.h * 3,
		})
	}
}

func (s *LiveScene) spawnSnow(n int) {
	glyphs := []rune{'*', '.', '+', '*', '.'}
	for i := 0; i < n; i++ {
		s.particles = append(s.particles, &Particle{
			x: float64(s.rng.Intn(s.g.w)), y: -float64(s.rng.Intn(s.g.h)),
			vx: (s.rng.Float64() - 0.5) * 0.1,
			vy: 0.06 + s.rng.Float64()*0.1,
			ch: glyphs[s.rng.Intn(len(glyphs))], color: BrightWhite, life: s.g.h * 10,
		})
	}
}

// ─── DRAW CALLS ───────────────────────────────────────────────────────────────

func (s *LiveScene) drawSun() {
	cx := s.g.w * 3 / 4
	cy := s.g.h / 6

	// 8 rotating rays
	for i := 0; i < 8; i++ {
		ang := s.sunAngle + float64(i)*math.Pi/4
		for r := 4; r <= 10; r++ {
			rx := cx + int(math.Round(float64(r)*math.Cos(ang)*1.7))
			ry := cy + int(math.Round(float64(r)*math.Sin(ang)*0.75))
			ch := '.'
			if r >= 7 {
				ch = '*'
			}
			if r == 10 {
				ch = '+'
			}
			s.g.set(rx, ry, ch, BrightYellow)
		}
	}
	// Sun body
	body := []string{
		"  .---.  ",
		" (     ) ",
		"( *   * )",
		" (     ) ",
		"  '---'  ",
	}
	for i, row := range body {
		for j, ch := range row {
			s.g.set(cx-4+j, cy-2+i, ch, BrightYellow)
		}
	}
	// Lens flare sparkle (random)
	if s.frame%6 == 0 {
		s.g.set(s.rng.Intn(s.g.w), s.rng.Intn(s.g.h/4), '+', Yellow)
	}
}

func (s *LiveScene) drawMoon() {
	cx := s.g.w*3/4
	cy := 3
	moon := []string{" .-. ", "(   )", " '-' "}
	for i, row := range moon {
		for j, ch := range row {
			s.g.set(cx+j, cy+i, ch, BrightWhite)
		}
	}
	// Persistent stars
	stars := [][2]int{{5,2},{14,4},{29,1},{44,3},{60,5},{74,2},{90,4},{10,7},{28,6},{52,8},{71,6},{88,7},{3,9},{19,10},{35,8}}
	for _, sp := range stars {
		if sp[0] < s.g.w && sp[1] < s.g.h {
			br := BrightBlack
			if (s.frame/10+sp[0]+sp[1])%4 == 0 {
				br = BrightWhite
			}
			s.g.set(sp[0], sp[1], '.', br)
		}
	}
}

func (s *LiveScene) drawClouds(heavy bool) {
	cc := White
	if heavy {
		cc = BrightBlack
	}
	for ci := range s.clouds {
		c := &s.clouds[ci]
		c.x += c.speed
		if c.x > float64(s.g.w+14) {
			c.x = -float64(len(c.rows[0])) - float64(s.rng.Intn(30))
			c.y = float64(1 + s.rng.Intn(8))
		}
		for ri, row := range c.rows {
			for xi, ch := range row {
				if ch != ' ' {
					s.g.set(int(c.x)+xi, int(c.y)+ri, ch, cc)
				}
			}
		}
	}
}

func (s *LiveScene) drawLightning() {
	if s.lightning <= 0 {
		return
	}
	if s.lightning > 5 {
		// Screen flash
		topH := s.g.h / 3
		for y := 0; y < topH; y++ {
			for x := 0; x < s.g.w; x++ {
				s.g.set(x, y, '#', BrightWhite)
			}
		}
		s.lightning--
		return
	}
	// Jagged bolt
	dx := 0
	x := s.lightX
	for y := 2; y < s.g.h-4; y++ {
		dx += s.rng.Intn(3) - 1
		dx = maxI(minI(dx, 3), -3)
		x = maxI(minI(x+dx*2, s.g.w-2), 1)
		ch := '|'
		if dx > 0 {
			ch = '/'
		} else if dx < 0 {
			ch = '\\'
		}
		s.g.set(x, y, ch, BrightYellow)
		if s.lightning <= 2 {
			s.g.set(x-1, y, '*', Yellow)
			s.g.set(x+1, y, '*', Yellow)
		}
	}
	s.lightning--
}

func (s *LiveScene) drawFog() {
	for y := 0; y < s.g.h-4; y++ {
		for x := 0; x < s.g.w; x++ {
			wave := math.Sin(float64(x)*0.11+float64(s.frame)*0.035+float64(y)*0.22)
			if wave > 0.15 {
				ch := '-'
				c := BrightBlack
				if wave > 0.55 {
					ch = '~'
					c = White
				}
				if wave > 0.8 {
					ch = '='
					c = BrightWhite
				}
				s.g.set(x, y, rune(ch), c)
			}
		}
	}
}

func (s *LiveScene) drawGround(wcode int) {
	isSnow := (wcode >= 71 && wcode <= 77) || wcode == 85 || wcode == 86
	gy := s.g.h - 3
	for x := 0; x < s.g.w; x++ {
		if isSnow {
			s.g.set(x, gy, '~', BrightWhite)
			s.g.set(x, gy+1, '~', White)
		} else {
			s.g.set(x, gy, '_', BrightBlack)
		}
	}
}

func (s *LiveScene) drawStats() {
	cur := s.wr.Current
	wi := getWMO(cur.WeatherCode)
	tc := tempColor(cur.Temperature2m)
	isDay := cur.IsDay == 1

	const bw = 30
	const bh = 13
	bx := 0
	by := s.g.h - bh - 1

	s.g.box(bx, by, bw, bh, BrightCyan)

	loc := s.location
	if len([]rune(loc)) > bw-4 {
		loc = string([]rune(loc)[:bw-4])
	}
	dayNight := "DAY"
	if !isDay {
		dayNight = "NIGHT"
	}

	s.g.txt(bx+2, by+1,  loc, BrightWhite)
	s.g.txt(bx+2, by+2,  wi.desc+"  ("+dayNight+")", wi.color)
	s.g.txt(bx+2, by+3,  fmt.Sprintf("Temp:     %.1f C", cur.Temperature2m), tc)
	s.g.txt(bx+2, by+4,  fmt.Sprintf("Feels:    %.1f C", cur.ApparentTemperature), tc)
	s.g.txt(bx+2, by+5,  fmt.Sprintf("Humidity: %.0f%%", cur.RelativeHumidity2m), Cyan)
	s.g.txt(bx+2, by+6,  fmt.Sprintf("Wind:     %.0f km/h %s", cur.WindSpeed10m, windArrow(cur.WindDirection10m)), BrightGreen)
	s.g.txt(bx+2, by+7,  fmt.Sprintf("Precip:   %.1f mm", cur.Precipitation), Blue)
	s.g.txt(bx+2, by+8,  fmt.Sprintf("Pressure: %.0f hPa", cur.PressureMsl), Green)
	s.g.txt(bx+2, by+9,  fmt.Sprintf("UV Index: %.0f", cur.UvIndex), Yellow)
	s.g.txt(bx+2, by+10, fmt.Sprintf("Vis:      %.1f km", cur.Visibility/1000), BrightBlack)
	s.g.txt(bx+2, by+11, "^C to exit  [ --live mode ]", BrightBlack)
}

// ─── UPDATE + RENDER ──────────────────────────────────────────────────────────

func (s *LiveScene) update() {
	s.frame++
	s.sunAngle += 0.014

	wcode := s.wr.Current.WeatherCode
	isRain := (wcode >= 51 && wcode <= 67) || (wcode >= 80 && wcode <= 82) || wcode >= 95
	isSnow := (wcode >= 71 && wcode <= 77) || wcode == 85 || wcode == 86
	isStorm := wcode >= 95

	if isRain && s.frame%2 == 0 {
		n := 3
		if wcode == 65 || wcode == 82 || isStorm {
			n = 8
		}
		s.spawnRain(n, wcode)
	}
	if isSnow && s.frame%5 == 0 {
		s.spawnSnow(2)
	}
	if isStorm && s.lightning <= 0 && s.rng.Intn(120) == 0 {
		s.lightning = 9
		s.lightX = s.g.w/4 + s.rng.Intn(s.g.w/2)
	}

	// Update particles; apply snow drift
	alive := s.particles[:0]
	for _, p := range s.particles {
		if p.ch != '|' { // snow drift
			p.vx = 0.08 * math.Sin(float64(s.frame)*0.04+p.x*0.08)
		}
		p.step()
		if p.life > 0 && p.y < float64(s.g.h-3) {
			alive = append(alive, p)
		}
	}
	s.particles = alive
}

func (s *LiveScene) renderFrame() string {
	wcode := s.wr.Current.WeatherCode
	isDay := s.wr.Current.IsDay == 1

	s.g.clear()

	// Background scatter
	switch {
	case wcode == 0 && isDay:
		for y := 0; y < s.g.h*2/3; y++ {
			for x := 0; x < s.g.w; x++ {
				if (x*7+y*13)%40 == 0 {
					s.g.set(x, y, '.', BrightBlack)
				}
			}
		}
	case wcode == 0 && !isDay:
		for y := 0; y < s.g.h*2/3; y++ {
			for x := 0; x < s.g.w; x++ {
				if (x*11+y*7)%71 == 0 {
					s.g.set(x, y, '.', BrightBlack)
				}
			}
		}
	case wcode >= 45 && wcode <= 48:
		s.drawFog()
	}

	// Sun or moon
	if wcode <= 2 {
		if isDay {
			s.drawSun()
		} else {
			s.drawMoon()
		}
	}

	// Clouds
	switch {
	case wcode >= 1 && wcode <= 3:
		s.drawClouds(wcode == 3)
	case wcode >= 51 && wcode <= 82:
		s.drawClouds(true)
	case wcode >= 95:
		s.drawClouds(true)
	}

	// Particles
	for _, p := range s.particles {
		s.g.set(int(p.x), int(p.y), p.ch, p.color)
		if int(p.y) == s.g.h-4 { // splash on ground
			s.g.set(int(p.x)-1, int(p.y), '~', p.color)
			s.g.set(int(p.x)+1, int(p.y), '~', p.color)
		}
	}

	// Lightning on top
	if wcode >= 95 {
		s.drawLightning()
	}

	s.drawGround(wcode)
	s.drawStats()

	// Title strip
	now := time.Now().Format("15:04:05")
	title := fmt.Sprintf("[ WEATHER8BIT -- LIVE ] %s  %s", s.location, now)
	tx := (s.g.w - len(title)) / 2
	if tx < 0 {
		tx = 0
	}
	s.g.txt(tx, 0, title, BrightCyan)

	var buf strings.Builder
	s.g.render(&buf)
	return buf.String()
}

// ─── ENTRY POINT ──────────────────────────────────────────────────────────────

func RunLiveMode(wr *WeatherResponse, locationName string) {
	w := termWidth()
	h := termHeight()
	if h < 20 {
		h = 24
	}
	h-- // one spare row to avoid scroll

	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	wcode := wr.Current.WeatherCode

	scene := &LiveScene{
		g:        newGrid(w, h),
		wr:       wr,
		location: locationName,
		rng:      rng,
	}

	// Pre-populate clouds
	nClouds := 4
	if wcode == 3 {
		nClouds = 7
	}
	for i := 0; i < nClouds; i++ {
		x := float64(rng.Intn(w))
		scene.clouds = append(scene.clouds, makeCloud(w, rng, x))
	}

	// Pre-fill particles so the scene isn't empty on frame 1
	isRain := (wcode >= 51 && wcode <= 67) || (wcode >= 80 && wcode <= 82) || wcode >= 95
	isSnow := (wcode >= 71 && wcode <= 77) || wcode == 85 || wcode == 86
	if isRain {
		scene.spawnRain(200, wcode)
	}
	if isSnow {
		scene.spawnSnow(120)
	}

	fmt.Print("\033[?25l") // hide cursor
	fmt.Print("\033[2J\033[H")

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	ticker := time.NewTicker(80 * time.Millisecond) // ~12 fps
	defer ticker.Stop()

	defer func() {
		signal.Stop(sigCh)
		fmt.Print("\033[?25h")
		fmt.Print("\033[2J\033[H")
		fmt.Println(col(BrightCyan, "\n  >> LIVE MODE ENDED"))
		fmt.Println()
	}()

	for {
		select {
		case <-sigCh:
			return
		case <-ticker.C:
			scene.update()
			frame := scene.renderFrame()
			os.Stdout.WriteString(frame) // single write = less flicker
		}
	}
}

package main

import (
	"fmt"
	"math"
	"math/rand"
	"strings"
	"time"
)

// ─── BATTLE STATS (derived from real weather data) ────────────────────────────

type Fighter struct {
	location string
	wi       WeatherInfo
	hp       int
	maxHP    int
	atk      int // from temperature extremity + UV
	def      int // from atmospheric pressure
	spd      int // from wind speed
	spc      int // special move chance (%) from humidity
}

func buildFighter(wr *WeatherResponse, name string) Fighter {
	cur := wr.Current
	wi := getWMO(cur.WeatherCode)

	// ATK: how extreme the temperature is + UV brutality
	atk := int(math.Abs(cur.Temperature2m)*1.3 + cur.UvIndex*2.8)
	atk = maxI(minI(atk, 60), 8)

	// DEF: higher pressure = more defensive
	def := int((cur.PressureMsl - 960) / 4.5)
	def = maxI(minI(def, 28), 3)

	// SPD: wind speed
	spd := int(cur.WindSpeed10m / 1.4)
	spd = maxI(minI(spd, 50), 3)

	// SPC: humid conditions = unpredictable special attacks
	spc := int(cur.RelativeHumidity2m * 0.35)
	spc = maxI(minI(spc, 45), 5)

	// HP: pressure-based with floor/ceil
	hp := 85 + int((cur.PressureMsl-1000)/4)
	hp = maxI(minI(hp, 120), 55)

	return Fighter{
		location: name,
		wi:       wi,
		hp: hp, maxHP: hp,
		atk: atk, def: def, spd: spd, spc: spc,
	}
}

// ─── FIGHTER SPRITES ──────────────────────────────────────────────────────────

// Each sprite: 7 lines, ~11 chars wide
var fighterSprites = map[string][]string{
	"sunny": {
		`  (***) `,
		` (^_^)  `,
		`  |=|   `,
		` / | \  `,
		`/ _|_ \ `,
		` [FIRE] `,
		`  ~~~   `,
	},
	"partly_cloudy": {
		`  (^~^) `,
		` (o.o)  `,
		`  |=|   `,
		` / | \  `,
		`/ _|_ \ `,
		` [GUST] `,
		`  ~~~   `,
	},
	"cloudy": {
		`  (___) `,
		` (._.)  `,
		`  |=|   `,
		` / | \  `,
		`/ _|_ \ `,
		` [GREY] `,
		`        `,
	},
	"rainy": {
		`  (~~~) `,
		` (~_~)  `,
		`  |=|   `,
		` / | \  `,
		`/ _|_ \ `,
		` [RAIN] `,
		`  ,,,   `,
	},
	"heavy_rain": {
		`  (===) `,
		` (=_=)  `,
		`  |=|   `,
		` / | \  `,
		`/ _|_ \ `,
		` [FLOOD]`,
		` ,,,,,  `,
	},
	"snowy": {
		`  (* *) `,
		` (*.*) `,
		`  |*|   `,
		` / | \  `,
		`/ _|_ \ `,
		` [ICE]  `,
		`  ***   `,
	},
	"thunderstorm": {
		`  /\/\  `,
		` (o_O)  `,
		`  |Z|   `,
		` / | \  `,
		`/ _|_ \ `,
		` [ZAP!] `,
		`  ///   `,
	},
	"foggy": {
		`  ---   `,
		` (-.-)  `,
		`  |=|   `,
		` / | \  `,
		`/ _|_ \ `,
		` [FOG]  `,
		`        `,
	},
	"night_clear": {
		`  ( * ) `,
		` (^_^)  `,
		`  |*|   `,
		` / | \  `,
		`/ _|_ \ `,
		` [MOON] `,
		`  ...   `,
	},
}

var attacksByIcon = map[string][]string{
	"sunny":         {"SCORCHING HEAT", "SOLAR FLARE", "UV OVERDRIVE", "HEAT WAVE CRASH", "SUNBURN STRIKE"},
	"partly_cloudy": {"MIXED STRIKE", "SUN BREAK SLASH", "VARIABLE ASSAULT", "WEATHER COMBO", "HALF-BAKED HIT"},
	"cloudy":        {"GREY ASSAULT", "OVERCAST SMASH", "DULL BUT STURDY", "CLOUD PUNCH", "OPPRESSIVE GLOOM"},
	"rainy":         {"RELENTLESS DRIZZLE", "SOGGY STRIKE", "WET MISERY COMBO", "FLOOD RUSH", "DAMP DESPAIR"},
	"heavy_rain":    {"TORRENTIAL FLOOD", "MONSOON ASSAULT", "DROWNING BLOW", "STORM DRAIN", "PUDDLE OF DOOM"},
	"snowy":         {"BLIZZARD STRIKE", "FROSTBITE SLASH", "ICE SHARD BURST", "FREEZE WAVE", "CHILLY OBLIVION"},
	"thunderstorm":  {"LIGHTNING STRIKE", "THUNDER CRASH", "VOLTAGE SHOCK", "STORM SURGE", "ELECTRIC FURY"},
	"foggy":         {"FOG OF CONFUSION", "SHROUD ATTACK", "MIST SLASH", "VISIBILITY DRAIN", "MURKY DEMISE"},
	"night_clear":   {"MOONBEAM STRIKE", "STARLIGHT BLAST", "MIDNIGHT RUSH", "DARK SKY SLASH", "COSMIC SMITE"},
}

var hitQuips = []string{
	"A direct hit!",
	"The crowd gasps!",
	"Ouch. That stings.",
	"Brutal efficiency.",
	"Weathering the storm!",
	"That's gonna leave a mark!",
	"Physics in action.",
	"Meteorologically savage!",
	"The barometer weeps.",
	"Nature is merciless.",
}

var lowHPQuips = []string{
	"is hanging on by a thread!",
	"looks absolutely desperate!",
	"is barely standing!",
	"won't last much longer!",
	"is running on empty!",
}

var missQuips = []string{
	"but the wind changes at the last second!",
	"but a lucky gust deflects it!",
	"but it evaporates before landing!",
	"but the fog swallows it whole!",
	"but the humidity absorbs the impact!",
}

func getAttack(iconKey string, rng *rand.Rand) string {
	if names, ok := attacksByIcon[iconKey]; ok && len(names) > 0 {
		return names[rng.Intn(len(names))]
	}
	return "WEATHER ATTACK"
}

func getSprite(iconKey string) []string {
	if s, ok := fighterSprites[iconKey]; ok {
		return s
	}
	return fighterSprites["cloudy"]
}

// ─── HP BAR ───────────────────────────────────────────────────────────────────

func battleHPBar(cur, max, width int) string {
	if max == 0 {
		max = 1
	}
	filled := cur * width / max
	filled = maxI(minI(filled, width), 0)
	c := BrightGreen
	pct := float64(cur) / float64(max)
	if pct < 0.25 {
		c = BrightRed
	} else if pct < 0.55 {
		c = Yellow
	}
	return c + strings.Repeat("█", filled) + BrightBlack + strings.Repeat("░", width-filled) + Reset +
		fmt.Sprintf(" %d/%d", cur, max)
}

// ─── ARENA RENDERER ───────────────────────────────────────────────────────────

func printArena(a, b Fighter, aHP, bHP, w int, log []string) {
	if w < 80 {
		w = 80
	}
	inner := w - 4
	half := inner / 2

	border := func(ch string) { fmt.Println("  +" + strings.Repeat(ch, inner) + "+") }
	padLine := func(content string) {
		pad := inner - visLen(content)
		if pad < 0 {
			pad = 0
		}
		fmt.Printf("  |%s%s|\n", content, strings.Repeat(" ", pad))
	}

	border("=")
	title := "  *** WEATHER  BATTLE ***  "
	padLine(strings.Repeat(" ", (inner-len(title))/2) + bold(BrightYellow, title))
	border("-")

	// Names
	aName := a.location
	bName := b.location
	if len([]rune(aName)) > half-2 { aName = string([]rune(aName)[:half-2]) }
	if len([]rune(bName)) > half-2 { bName = string([]rune(bName)[:half-2]) }
	nameLine := " " + bold(a.wi.color, aName) + strings.Repeat(" ", half-1-len([]rune(aName))) +
		bold(b.wi.color, bName)
	padLine(nameLine)

	// HP bars
	barW := half/2 - 4
	if barW < 10 { barW = 10 }
	aBar := battleHPBar(aHP, a.maxHP, barW)
	bBar := battleHPBar(bHP, b.maxHP, barW)
	hpLabel := " HP: " + aBar
	hpPad := inner - visLen(hpLabel) - visLen(bBar) - 3
	if hpPad < 1 { hpPad = 1 }
	padLine(hpLabel + strings.Repeat(" ", hpPad) + bBar + " ")

	padLine("")

	// Fighters side by side
	aS := getSprite(a.wi.iconKey)
	bS := getSprite(b.wi.iconKey)
	rows := maxI(len(aS), len(bS))
	spriteW := 10
	gap := inner - spriteW*2 - 4
	if gap < 4 { gap = 4 }

	for i := 0; i < rows; i++ {
		aLine, bLine := "", ""
		if i < len(aS) { aLine = aS[i] }
		if i < len(bS) { bLine = bS[i] }
		vs := "  "
		if i == rows/2 { vs = "VS" }
		line := " " + col(a.wi.color, fmt.Sprintf("%-*s", spriteW, aLine)) +
			strings.Repeat(" ", gap/2-1) + col(BrightBlack, vs) +
			strings.Repeat(" ", gap-gap/2-1) +
			col(b.wi.color, fmt.Sprintf("%-*s", spriteW, bLine))
		padLine(line)
	}

	padLine("")

	// Stats
	aStats := fmt.Sprintf(" ATK:%-3d DEF:%-3d SPD:%-3d SPC:%d%%", a.atk, a.def, a.spd, a.spc)
	bStats := fmt.Sprintf("ATK:%-3d DEF:%-3d SPD:%-3d SPC:%d%%", b.atk, b.def, b.spd, b.spc)
	statsPad := inner - len(aStats) - len(bStats) - 2
	if statsPad < 1 { statsPad = 1 }
	padLine(col(BrightBlack, aStats) + strings.Repeat(" ", statsPad) + col(BrightBlack, bStats))

	border("-")

	// Battle log (last 4 lines)
	const logLines = 4
	start := 0
	if len(log) > logLines { start = len(log) - logLines }
	for _, entry := range log[start:] {
		padLine(" " + entry)
	}
	for i := len(log) - start; i < logLines; i++ {
		padLine("")
	}
	border("=")
}

// ─── PROJECTILE ANIMATION ─────────────────────────────────────────────────────

func animateProjectile(fromLeft bool, w int, c string) {
	arrow := ">>>>>>>>>>"
	if !fromLeft {
		arrow = "<<<<<<<<<<"
	}
	al := len(arrow)
	steps := (w - 12) / 4
	for i := 0; i <= steps; i++ {
		var pos int
		if fromLeft {
			pos = 6 + i*4
		} else {
			pos = w - 6 - al - i*4
		}
		pos = maxI(minI(pos, w-al-4), 4)
		line := strings.Repeat(" ", pos) + c + arrow + Reset
		fmt.Printf("\r%s", line)
		time.Sleep(25 * time.Millisecond)
	}
	fmt.Printf("\r%s\r\n", strings.Repeat(" ", w))
}

// ─── MAIN BATTLE FUNCTION ─────────────────────────────────────────────────────

func RunBattleMode(city1, city2 string) {
	w := termWidth()
	if w < 80 { w = 80 }

	clearScreen()
	printHeader()
	fmt.Println()

	// ── Fetch city 1 ──
	stopA := startLoading("SEARCHING: " + strings.ToUpper(city1) + " ...")
	resA, errA := geocode(city1)
	stopA()
	if errA != nil || len(resA) == 0 {
		fmt.Println(col(Red, "  ERROR: location not found: "+city1))
		return
	}
	chosenA := resA[0]
	partsA := []string{chosenA.Name}
	if chosenA.Admin1 != "" { partsA = append(partsA, chosenA.Admin1) }
	if chosenA.Country != "" { partsA = append(partsA, chosenA.Country) }
	nameA := strings.Join(partsA, ", ")

	stopA2 := startLoading("FETCHING WEATHER: " + strings.ToUpper(nameA) + " ...")
	wrA, errA2 := fetchWeather(chosenA.Latitude, chosenA.Longitude)
	stopA2()
	if errA2 != nil {
		fmt.Println(col(Red, "  ERROR: "+errA2.Error()))
		return
	}

	// ── Fetch city 2 ──
	stopB := startLoading("SEARCHING: " + strings.ToUpper(city2) + " ...")
	resB, errB := geocode(city2)
	stopB()
	if errB != nil || len(resB) == 0 {
		fmt.Println(col(Red, "  ERROR: location not found: "+city2))
		return
	}
	chosenB := resB[0]
	partsB := []string{chosenB.Name}
	if chosenB.Admin1 != "" { partsB = append(partsB, chosenB.Admin1) }
	if chosenB.Country != "" { partsB = append(partsB, chosenB.Country) }
	nameB := strings.Join(partsB, ", ")

	stopB2 := startLoading("FETCHING WEATHER: " + strings.ToUpper(nameB) + " ...")
	wrB, errB2 := fetchWeather(chosenB.Latitude, chosenB.Longitude)
	stopB2()
	if errB2 != nil {
		fmt.Println(col(Red, "  ERROR: "+errB2.Error()))
		return
	}

	// ── Build fighters ──
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	fa := buildFighter(wrA, nameA)
	fb := buildFighter(wrB, nameB)
	aHP := fa.maxHP
	bHP := fb.maxHP
	var log []string

	// ── Intro screen ──
	clearScreen()
	printHeader()
	fmt.Println()
	fmt.Println(bold(BrightYellow, "  THE WEATHER GODS HAVE DECREED: IT IS TIME TO BATTLE."))
	fmt.Println()
	fmt.Printf("  %s\n", col(fa.wi.color, nameA+" ["+fa.wi.desc+"]"))
	fmt.Printf("  %-*s\n", 6, "VS")
	fmt.Printf("  %s\n", col(fb.wi.color, nameB+" ["+fb.wi.desc+"]"))
	fmt.Println()
	time.Sleep(1800 * time.Millisecond)

	clearScreen()
	printHeader()
	printArena(fa, fb, aHP, bHP, w, log)
	fmt.Println()
	fmt.Println(bold(BrightWhite, "  FIGHT!"))
	time.Sleep(1200 * time.Millisecond)

	// ── Battle rounds ──
	const maxRounds = 6
	winner := ""

	for round := 1; round <= maxRounds && aHP > 0 && bHP > 0; round++ {
		clearScreen()
		printHeader()
		printArena(fa, fb, aHP, bHP, w, log)
		fmt.Printf("\n  %s\n", bold(BrightWhite, fmt.Sprintf("-- ROUND %d --", round)))
		time.Sleep(500 * time.Millisecond)

		// Higher SPD attacks first
		aFirst := fa.spd >= fb.spd
		if fa.spd == fb.spd {
			aFirst = rng.Intn(2) == 0
		}

		for turn := 0; turn < 2; turn++ {
			attackerA := (turn == 0 && aFirst) || (turn == 1 && !aFirst)
			var att, def *Fighter
			var defHP *int
			if attackerA {
				att, def, defHP = &fa, &fb, &bHP
			} else {
				att, def, defHP = &fb, &fa, &aHP
			}

			attackName := getAttack(att.wi.iconKey, rng)

			// Damage calculation
			base := float64(att.atk) * (0.8 + rng.Float64()*0.4)
			base -= float64(def.def) * 0.45
			if base < 4 { base = 4 }
			dmg := int(base)

			// Special move?
			isSpecial := rng.Intn(100) < att.spc
			specTag := ""
			if isSpecial {
				dmg = int(float64(dmg) * 1.7)
				specTag = bold(BrightMagenta, " [SPECIAL!]")
			}

			// Dodge?
			dodge := rng.Intn(100) < def.spd/6
			if dodge {
				dmg = 0
			}

			// Build log entry
			attName := att.location
			if len([]rune(attName)) > 14 { attName = string([]rune(attName)[:14]) }
			var entry string
			if dodge {
				entry = col(BrightBlack, ">>") + " " + col(att.wi.color, attName) +
					" uses " + bold(BrightWhite, attackName) + specTag + " ... " +
					col(BrightBlack, missQuips[rng.Intn(len(missQuips))])
			} else {
				entry = col(BrightBlack, ">>") + " " + col(att.wi.color, attName) +
					" uses " + bold(BrightWhite, attackName) + specTag +
					" -> " + bold(BrightRed, fmt.Sprintf("%d DMG", dmg))
			}
			log = append(log, entry)

			// Print action
			fmt.Printf("\n  %s uses %s%s!\n",
				bold(att.wi.color, attName), bold(BrightWhite, attackName), specTag)

			// Projectile
			animateProjectile(attackerA, w, att.wi.color)

			// Result
			*defHP -= dmg
			if *defHP < 0 { *defHP = 0 }

			if dodge {
				fmt.Printf("  %s\n", col(BrightBlack, "...dodged!"))
			} else {
				quip := hitQuips[rng.Intn(len(hitQuips))]
				fmt.Printf("  %s takes %s damage! %s\n",
					bold(def.wi.color, def.location),
					bold(BrightRed, fmt.Sprintf("%d", dmg)),
					col(BrightBlack, quip))
			}

			// Low HP taunt
			if *defHP > 0 && float64(*defHP)/float64(def.maxHP) < 0.28 {
				fmt.Printf("  %s %s\n",
					bold(def.wi.color, def.location),
					col(BrightRed, lowHPQuips[rng.Intn(len(lowHPQuips))]))
			}

			if *defHP <= 0 {
				if attackerA { winner = fa.location } else { winner = fb.location }
				break
			}
			time.Sleep(900 * time.Millisecond)

			// Redraw after each attack
			clearScreen()
			printHeader()
			printArena(fa, fb, aHP, bHP, w, log)
			fmt.Printf("\n  Round %d — turn %d/2\n", round, turn+1)
			time.Sleep(400 * time.Millisecond)
		}
		if winner != "" {
			break
		}
	}

	// ── Determine winner if rounds ran out ──
	if winner == "" {
		switch {
		case aHP > bHP: winner = fa.location
		case bHP > aHP: winner = fb.location
		default:        winner = "DRAW"
		}
	}

	// ── Final screen ──
	clearScreen()
	printHeader()
	printArena(fa, fb, maxI(aHP, 0), maxI(bHP, 0), w, log)
	fmt.Println()

	if winner == "DRAW" {
		fmt.Println(bold(BrightWhite, "  IT'S A DRAW! Both cities fought valiantly!"))
		fmt.Println(col(BrightBlack, "  The atmosphere remains undecided."))
	} else {
		wc := fa.wi.color
		if winner == fb.location { wc = fb.wi.color }
		fmt.Println(bold(wc, "  *** WINNER: "+strings.ToUpper(winner)+" ***"))
		fmt.Println(col(BrightBlack, "  The weather gods have spoken. Climate is power."))
		fmt.Println()
		fmt.Println(col(BrightYellow, "      .-.     "))
		fmt.Println(col(BrightYellow, "     (^_^)    "))
		fmt.Println(col(BrightYellow, "    \\[T]/     "))
		fmt.Println(col(BrightYellow, "     / \\      "))
		fmt.Println(col(BrightYellow, "  VICTORY!    "))
	}
	fmt.Println()
}

// Package prototype is a THROWAWAY UI sandbox for the phase-wait throbber.
//
// Question: what should the per-phase wait indicator look like?
// Run: ship throbber
//
// Full-screen vertical rocket ascending into night sky — not production. See NOTES.md.
package prototype

import (
	"context"
	"fmt"
	"io"
	"math"
	"math/rand"
	"os"
	"strings"
	"syscall"
	"time"
	"unsafe"
)

// Options control a demo (or future phase wait).
type Options struct {
	Phase string
	Color bool
}

type cell struct {
	ch rune
	fg string // ANSI SGR params, e.g. "38;2;r;g;b" or ""
}

// Run draws the rocket tableau until ctx is done, then restores the terminal.
func Run(ctx context.Context, w io.Writer, opts Options) {
	if opts.Phase == "" {
		opts.Phase = "Implement"
	}
	if !isTTY(w) {
		fmt.Fprintf(w, "ship: waiting on %s…\n", opts.Phase)
		<-ctx.Done()
		fmt.Fprintf(w, "ship: %s done\n", opts.Phase)
		return
	}

	cols, rows := termSize(w)
	if cols < 40 {
		cols = 40
	}
	if rows < 16 {
		rows = 16
	}

	enterAlt(w)
	hideCursor(w)
	defer func() {
		showCursor(w)
		leaveAlt(w)
	}()

	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	stars := seedStars(cols, rows, rng)
	start := time.Now()
	tick := time.NewTicker(70 * time.Millisecond)
	defer tick.Stop()

	frame := 0
	for {
		select {
		case <-ctx.Done():
			clearScreen(w)
			fmt.Fprintf(w, "%s\n", finishLine(opts, time.Since(start)))
			return
		case <-tick.C:
			buf := renderFrame(cols, rows, frame, time.Since(start), opts, stars, rng)
			paint(w, buf, cols, rows)
			frame++
		}
	}
}

func finishLine(opts Options, d time.Duration) string {
	check := "✓"
	if opts.Color {
		check = "\033[38;2;120;200;140m✓\033[0m"
	}
	return fmt.Sprintf("%s  %s  ·  %s", check, opts.Phase, formatDur(d))
}

func formatDur(d time.Duration) string {
	d = d.Truncate(time.Second)
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	return fmt.Sprintf("%dm %02ds", int(d.Minutes()), int(d.Seconds())%60)
}

func isTTY(w io.Writer) bool {
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	fi, err := f.Stat()
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice != 0
}

func termSize(w io.Writer) (cols, rows int) {
	cols, rows = 80, 24
	f, ok := w.(*os.File)
	if !ok {
		return
	}
	var ws struct {
		Row, Col, X, Y uint16
	}
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, f.Fd(), uintptr(syscall.TIOCGWINSZ), uintptr(unsafe.Pointer(&ws)))
	if errno == 0 && ws.Col > 0 && ws.Row > 0 {
		return int(ws.Col), int(ws.Row)
	}
	return
}

func enterAlt(w io.Writer)    { fmt.Fprint(w, "\033[?1049h\033[2J\033[H") }
func leaveAlt(w io.Writer)    { fmt.Fprint(w, "\033[?1049l") }
func hideCursor(w io.Writer)  { fmt.Fprint(w, "\033[?25l") }
func showCursor(w io.Writer)  { fmt.Fprint(w, "\033[?25h") }
func clearScreen(w io.Writer) { fmt.Fprint(w, "\033[2J\033[H") }

type star struct {
	x, y   int
	phase  float64
	bright int // 0..2
}

func seedStars(cols, rows int, rng *rand.Rand) []star {
	// Sparse quiet sky — leave HUD alone.
	skyH := rows - 4
	if skyH < 8 {
		skyH = rows
	}
	n := cols * skyH / 32
	if n < 18 {
		n = 18
	}
	if n > 52 {
		n = 52
	}
	out := make([]star, n)
	for i := range out {
		out[i] = star{
			x:      rng.Intn(cols),
			y:      rng.Intn(skyH),
			phase:  rng.Float64() * 2 * math.Pi,
			bright: rng.Intn(4), // most stay dim
		}
	}
	return out
}

// Compact vertical rocket (nose up). Markers: * tip, o viewport, = engines.
var rocketArt = []string{
	`   *`,
	`  /|\`,
	` | o |`,
	` |   |`,
	`/|= =|\`,
}

const rocketArtWidth = 8

func renderFrame(cols, rows int, frame int, elapsed time.Duration, opts Options, stars []star, rng *rand.Rand) [][]cell {
	buf := make([][]cell, rows)
	for y := 0; y < rows; y++ {
		buf[y] = make([]cell, cols)
		for x := 0; x < cols; x++ {
			buf[y][x] = cell{ch: ' '}
		}
	}

	hudH := 2
	skyBot := rows - hudH
	color := opts.Color
	t := float64(frame) * 0.05
	_ = rng

	// Flat deep space — no busy gradient fill noise
	bg := cRGB(color, 8, 10, 22)
	for y := 0; y < skyBot; y++ {
		for x := 0; x < cols; x++ {
			buf[y][x] = cell{ch: ' ', fg: bg}
		}
	}

	// Quiet stars + very slow downward drift
	starScroll := frame / 6
	for _, s := range stars {
		sy := (s.y + starScroll) % max(1, skyBot)
		if sy < 0 || sy >= skyBot || s.x >= cols {
			continue
		}
		tw := 0.5 + 0.5*math.Sin(t*0.7+s.phase)
		ch := rune('.')
		fg := cRGB(color, 55, 65, 95)
		if s.bright == 0 && tw > 0.7 {
			ch = '·'
			fg = cRGB(color, 110, 125, 165)
		}
		buf[sy][s.x] = cell{ch: ch, fg: fg}
	}

	// One compact rocket, almost still (tiny bob only); twin exhaust under fins.
	rocketH := len(rocketArt)
	plumeRoom := 4
	baseY := (skyBot-rocketH-plumeRoom)/2 + int(math.Round(math.Sin(t*0.5)))
	if baseY < 1 {
		baseY = 1
	}
	if baseY+rocketH+plumeRoom > skyBot {
		baseY = max(0, skyBot-rocketH-plumeRoom)
	}
	shipX := cols/2 - rocketArtWidth/2
	drawRocket(buf, shipX, baseY, color, t)
	// Plumes under each `=` in the base row (`/|= =|\`).
	drawPlume(buf, shipX+2, baseY+rocketH, cols, skyBot, color, t, frame)
	drawPlume(buf, shipX+4, baseY+rocketH, cols, skyBot, color, t, frame)

	drawHUD(buf, cols, rows, hudH, opts, elapsed)
	return buf
}

func drawRocket(buf [][]cell, ox, oy int, color bool, t float64) {
	edge := cRGB(color, 185, 195, 220)
	glass := cRGB(color, 90, 200, 230)
	lamp := cRGB(color, 255, 200, 110)
	fin := cRGB(color, 210, 130, 70)
	engine := cRGB(color, 255, 150, 70)
	rows := len(buf)
	cols := 0
	if rows > 0 {
		cols = len(buf[0])
	}
	_ = t
	for i, line := range rocketArt {
		y := oy + i
		if y < 0 || y >= rows {
			continue
		}
		for j, ch := range line {
			x := ox + j
			if x < 0 || x >= cols || ch == ' ' {
				continue
			}
			fg := edge
			switch ch {
			case '*':
				fg = lamp
				ch = '▲'
			case 'o':
				fg = glass
				ch = '●'
			case '=':
				fg = engine
				ch = '▼'
			case '/', '\\':
				fg = fin
			}
			buf[y][x] = cell{ch: ch, fg: fg}
		}
	}
}

func drawPlume(buf [][]cell, plumeX, engineY, cols, skyBot int, color bool, t float64, frame int) {
	if engineY < 0 || engineY >= skyBot {
		return
	}
	cx := plumeX
	// Short single-column flame — gentle breathe, no crackle.
	length := 2
	if math.Sin(t*3) > 0.3 {
		length = 3
	}
	glyphs := []rune{'▓', '▒', '░'}
	for i := 0; i < length; i++ {
		y := engineY + i
		if y >= skyBot {
			break
		}
		x := cx
		if x < 0 || x >= cols {
			continue
		}
		ch := glyphs[min(i, len(glyphs)-1)]
		// Soft heat falloff
		heat := 1.0 - float64(i)/float64(length)
		r := min(255, int(160+heat*80))
		g := min(255, int(50+heat*90))
		b := 30
		_ = frame
		buf[y][x] = cell{ch: ch, fg: cRGB(color, r, g, b)}
	}
}

func drawHUD(buf [][]cell, cols, rows, hudH int, opts Options, elapsed time.Duration) {
	top := rows - hudH
	color := opts.Color
	barFG := cRGB(color, 230, 175, 90)
	dimFG := cRGB(color, 70, 75, 95)

	for y := top; y < rows; y++ {
		for x := 0; x < cols; x++ {
			ch := ' '
			if y == top {
				ch = '─'
			}
			buf[y][x] = cell{ch: ch, fg: dimFG}
		}
	}

	label := fmt.Sprintf("%s  ·  %s", opts.Phase, formatDur(elapsed))
	runes := []rune(label)
	start := (cols - len(runes)) / 2
	y := top
	if hudH > 1 {
		y = top + 1
	}
	if y >= rows {
		y = rows - 1
	}
	for i, ch := range runes {
		x := start + i
		if x < 0 || x >= cols {
			continue
		}
		buf[y][x] = cell{ch: ch, fg: barFG}
	}
}

func paint(w io.Writer, buf [][]cell, cols, rows int) {
	var b strings.Builder
	b.Grow(rows * cols * 12)
	b.WriteString("\033[H")
	var lastFG string
	for y := 0; y < rows; y++ {
		for x := 0; x < cols; x++ {
			c := buf[y][x]
			fg := c.fg
			if fg == "" {
				fg = "0"
			}
			if fg != lastFG {
				b.WriteString("\033[0;")
				b.WriteString(fg)
				b.WriteByte('m')
				lastFG = fg
			}
			b.WriteRune(c.ch)
		}
		if y < rows-1 {
			b.WriteByte('\n')
		}
	}
	b.WriteString("\033[0m")
	fmt.Fprint(w, b.String())
}

func rgb(r, g, b int) string {
	return fmt.Sprintf("38;2;%d;%d;%d", r, g, b)
}

func cRGB(color bool, r, g, b int) string {
	if !color {
		return ""
	}
	return rgb(r, g, b)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

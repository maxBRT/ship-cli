package throbber

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"time"
)

// brailleFrames are the classic CLI spinner glyphs.
var brailleFrames = []rune{'⠋', '⠙', '⠹', '⠸', '⠼', '⠴', '⠦', '⠧', '⠇', '⠏'}

// Line is the production Phase-wait UI: one status line with a braille
// throbber on a TTY, a single waiting line otherwise.
type Line struct {
	Out   io.Writer
	Color bool
}

var _ Port = Line{}

// During shows wait UI for status until work returns, then finishes
// succeed/fail from work's error. Always restores the terminal before returning.
func (l Line) During(ctx context.Context, status Status, work func(context.Context) error) error {
	if status.Phase == "" {
		status.Phase = "Implement"
	}
	out := l.Out
	if out == nil {
		out = io.Discard
	}
	start := time.Now()

	if !isTTY(out) {
		fmt.Fprintf(out, "ship: %s\n", formatStatus(status, '…'))
		err := work(ctx)
		fmt.Fprintln(out, finishLine(status, l.Color, err == nil, time.Since(start)))
		return err
	}

	uiCtx, stopUI := context.WithCancel(ctx)
	done := make(chan struct{})
	go func() {
		defer close(done)
		runLine(uiCtx, out, status, l.Color)
	}()

	err := work(ctx)
	stopUI()
	<-done
	clearLine(out)
	fmt.Fprintln(out, finishLine(status, l.Color, err == nil, time.Since(start)))
	return err
}

func formatStatus(s Status, spin rune) string {
	label := s.Label()
	if label == "" {
		return string(spin)
	}
	return fmt.Sprintf("%c  %s", spin, label)
}

func finishLine(status Status, color, ok bool, d time.Duration) string {
	label := formatStatus(status, ' ')
	label = strings.TrimSpace(label)
	if !ok {
		mark := "✗"
		if color {
			mark = "\033[38;2;220;90;90m✗\033[0m"
		}
		return fmt.Sprintf("%s  %s failed  ·  %s", mark, label, formatDur(d))
	}
	check := "✓"
	if color {
		check = "\033[38;2;120;200;140m✓\033[0m"
	}
	return fmt.Sprintf("%s  %s  ·  %s", check, label, formatDur(d))
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

func hideCursor(w io.Writer) { fmt.Fprint(w, "\033[?25l") }
func showCursor(w io.Writer) { fmt.Fprint(w, "\033[?25h") }
func clearLine(w io.Writer)  { fmt.Fprint(w, "\r\033[K") }

// runLine rewrites one status line with a braille throbber until ctx is done.
func runLine(ctx context.Context, w io.Writer, status Status, color bool) {
	hideCursor(w)
	defer showCursor(w)

	tick := time.NewTicker(80 * time.Millisecond)
	defer tick.Stop()

	frame := 0
	paint := func() {
		spin := brailleFrames[frame%len(brailleFrames)]
		line := formatStatus(status, spin)
		if color {
			line = "\033[38;2;230;175;90m" + line + "\033[0m"
		}
		fmt.Fprintf(w, "\r\033[K%s", line)
	}
	paint()
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
			frame++
			paint()
		}
	}
}

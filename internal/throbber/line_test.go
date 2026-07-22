package throbber

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"
)

// ansiTerm is a tiny screen that understands the sequences runLine emits:
// \r, \n, \033[K (clear to EOL), \033[1A (cursor up), \033[?25l/h (ignored).
type ansiTerm struct {
	lines [][]byte
	row   int
	col   int
}

func (t *ansiTerm) Write(p []byte) (int, error) {
	for i := 0; i < len(p); i++ {
		switch p[i] {
		case '\r':
			t.col = 0
		case '\n':
			t.row++
			t.col = 0
			t.ensureRow()
		case '\033':
			if i+1 < len(p) && p[i+1] == '[' {
				i += 2
				n := 0
				for i < len(p) && !((p[i] >= 'A' && p[i] <= 'Z') || (p[i] >= 'a' && p[i] <= 'z')) {
					if p[i] >= '0' && p[i] <= '9' {
						n = n*10 + int(p[i]-'0')
					}
					i++
				}
				if i >= len(p) {
					break
				}
				switch p[i] {
				case 'K': // clear to end of line
					t.ensureRow()
					if t.col < len(t.lines[t.row]) {
						t.lines[t.row] = t.lines[t.row][:t.col]
					}
				case 'A': // cursor up n (default 1)
					if n == 0 {
						n = 1
					}
					t.row -= n
					if t.row < 0 {
						t.row = 0
					}
					t.col = 0
				case 'l', 'h': // cursor hide/show — ignore
				}
			}
		default:
			t.ensureRow()
			for len(t.lines[t.row]) <= t.col {
				t.lines[t.row] = append(t.lines[t.row], ' ')
			}
			t.lines[t.row][t.col] = p[i]
			t.col++
		}
	}
	return len(p), nil
}

func (t *ansiTerm) ensureRow() {
	for len(t.lines) <= t.row {
		t.lines = append(t.lines, nil)
	}
}

func (t *ansiTerm) String() string {
	var b strings.Builder
	for i, line := range t.lines {
		if i > 0 {
			b.WriteByte('\n')
		}
		b.Write(bytes.TrimRight(line, " "))
	}
	return b.String()
}

func captureTTYScreen(t *testing.T, status Status, ticks int) string {
	t.Helper()
	term := &ansiTerm{}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		runLine(ctx, term, status, false)
	}()
	time.Sleep(time.Duration(ticks)*80*time.Millisecond + 40*time.Millisecond)
	cancel()
	<-done
	return term.String()
}

func TestRunLine_withRemaining_rewritesInPlace(t *testing.T) {
	screen := captureTTYScreen(t, Status{
		Phase:     "Implement",
		Iteration: 1,
		Ticket:    "#8 Add throbber",
		Remaining: "queue remaining · #7 · #9",
	}, 5)

	statusCopies := 0
	for _, line := range strings.Split(screen, "\n") {
		if strings.Contains(line, "Iteration 1") {
			statusCopies++
		}
	}
	if statusCopies != 1 {
		t.Fatalf("visible Iteration lines = %d, want 1 (in-place redraw). screen=%q", statusCopies, screen)
	}
	if !strings.Contains(screen, "queue remaining · #7 · #9") {
		t.Fatalf("missing remaining hint: %q", screen)
	}
}

func TestRunLine_withoutRemaining_rewritesInPlace(t *testing.T) {
	screen := captureTTYScreen(t, Status{
		Phase:     "Review",
		Iteration: 2,
		Ticket:    "#3 fix",
	}, 5)

	if strings.Count(screen, "\n") != 0 {
		t.Fatalf("single-line throbber grew newlines: %q", screen)
	}
	if !strings.Contains(screen, "Iteration 2") {
		t.Fatalf("missing status: %q", screen)
	}
}

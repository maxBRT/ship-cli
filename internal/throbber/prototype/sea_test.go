package prototype

import (
	"bytes"
	"context"
	"math/rand"
	"testing"
	"time"
)

func TestRenderFrame_noPanicOnPlume(t *testing.T) {
	// Repro: frame 0 with tip flare (dx=-1) made Go's % return a negative
	// flicker index and panic with "index out of range [-1]".
	rng := rand.New(rand.NewSource(1))
	stars := seedStars(80, 24, rng)
	for frame := 0; frame < 120; frame++ {
		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("renderFrame panicked at frame %d: %v", frame, r)
				}
			}()
			_ = renderFrame(80, 24, frame, time.Duration(frame)*50*time.Millisecond,
				Options{Phase: "Implement", Color: true}, stars, rng)
		}()
	}
}

func TestDrawPlume_negativeModuloSafe(t *testing.T) {
	buf := make([][]cell, 20)
	for y := range buf {
		buf[y] = make([]cell, 40)
	}
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("drawPlume panicked: %v", r)
		}
	}()
	// Conditions that previously yielded fi == -1 from Go's signed %.
	drawPlume(buf, 13, 10, 40, 18, true, 0, 0)
}

func TestRun_nonTTY(t *testing.T) {
	var buf bytes.Buffer
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	Run(ctx, &buf, Options{Phase: "Implement", Color: false})
	if buf.Len() == 0 {
		t.Fatal("expected non-TTY waiting lines")
	}
}

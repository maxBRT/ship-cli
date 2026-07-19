package throbber

import (
	"math/rand"
	"testing"
	"time"
)

func TestRenderFrame_noPanicOnPlume(t *testing.T) {
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
				"Implement", true, stars)
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
	drawPlume(buf, 13, 10, 40, 18, true, 0)
}

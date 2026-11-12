package curve

import (
	"math"
	"math/rand"
	"testing"
)

func TestNewFalloff_RejectsInvalid(t *testing.T) {
	cases := []struct {
		name   string
		knots  []Knot
		zeroAt float64
		floor  float64
	}{
		{"floor zero", nil, 10, 0},
		{"floor positive", nil, 10, 5},
		{"zeroAt zero without knots", nil, 0, -100},
		{"zeroAt negative", nil, -1, -100},
		{"knot at distance zero", []Knot{{0, 75}}, 10, -100},
		{"knot at negative distance", []Knot{{-1, 75}}, 10, -100},
		{"knot score equal to peak (flat)", []Knot{{5, 100}}, 10, -100},
		{"knot score above peak (rising)", []Knot{{5, 110}}, 10, -100},
		{"knot score zero", []Knot{{5, 0}}, 10, -100},
		{"knot score negative", []Knot{{5, -5}}, 10, -100},
		{"knot distances not increasing", []Knot{{5, 80}, {5, 60}}, 10, -100},
		{"knot distances decreasing", []Knot{{5, 80}, {4, 60}}, 10, -100},
		{"knot scores equal (flat)", []Knot{{5, 80}, {8, 80}}, 10, -100},
		{"knot scores rising", []Knot{{5, 60}, {8, 80}}, 10, -100},
		{"zeroAt at last knot", []Knot{{5, 75}}, 5, -100},
		{"zeroAt before last knot", []Knot{{5, 75}}, 3, -100},
		{"NaN zeroAt", nil, math.NaN(), -100},
		{"infinite floor", nil, 10, math.Inf(-1)},
		{"NaN knot", []Knot{{math.NaN(), 75}}, 10, -100},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := NewFalloff(tc.knots, tc.zeroAt, tc.floor); err == nil {
				t.Fatalf("expected an error, got none")
			}
		})
	}
}

func TestFalloff_ZeroValuePanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("expected Score on a zero-value Falloff to panic")
		}
	}()
	var f Falloff
	f.Score(1)
}

func TestMustFalloff_PanicsOnInvalid(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("expected MustFalloff to panic on invalid input")
		}
	}()
	MustFalloff(nil, 10, 0)
}

func TestFalloff_AnchorPoints(t *testing.T) {
	f, err := NewFalloff([]Knot{{5, 75}}, 25, -200)
	if err != nil {
		t.Fatal(err)
	}
	checks := []struct{ d, want float64 }{
		{-3, 100},
		{0, 100},
		{2.5, 87.5},
		{5, 75},
		{25, 0},
	}
	for _, c := range checks {
		if got := f.Score(c.d); math.Abs(got-c.want) > 1e-9 {
			t.Errorf("Score(%v) = %v, want %v", c.d, got, c.want)
		}
	}
	if f.Floor() != -200 {
		t.Errorf("Floor() = %v, want -200", f.Floor())
	}
}

func TestFalloff_DoesNotMutateWithCallerSlice(t *testing.T) {
	knots := []Knot{{5, 75}}
	f, err := NewFalloff(knots, 25, -200)
	if err != nil {
		t.Fatal(err)
	}
	knots[0].Score = 100 // would make the curve flat if it were shared
	if got := f.Score(5); math.Abs(got-75) > 1e-9 {
		t.Fatalf("Score(5) = %v after caller mutated its slice, want 75", got)
	}
}

// TestFalloff_Properties checks, over many generated valid curves, the
// guarantees the rest of the program relies on: continuity, strict monotonic
// decrease, and a floor that is approached but never reached.
func TestFalloff_Properties(t *testing.T) {
	rng := rand.New(rand.NewSource(1))

	for n := range 500 {
		nKnots := rng.Intn(4)
		knots := make([]Knot, 0, nKnots)
		d, s := 0.0, Peak
		for range nKnots {
			d += 0.1 + rng.Float64()*30
			s -= (s - 1) * (0.05 + rng.Float64()*0.6)
			knots = append(knots, Knot{D: d, Score: s})
		}
		zeroAt := d + 0.1 + rng.Float64()*50
		floor := -(1 + rng.Float64()*400)

		f, err := NewFalloff(knots, zeroAt, floor)
		if err != nil {
			t.Fatalf("curve %d: generated curve rejected: %v", n, err)
		}

		if got := f.Score(0); got != Peak {
			t.Fatalf("curve %d: Score(0) = %v, want %v", n, got, Peak)
		}
		for _, k := range knots {
			if got := f.Score(k.D); math.Abs(got-k.Score) > 1e-9 {
				t.Fatalf("curve %d: Score(%v) = %v, want knot score %v", n, k.D, got, k.Score)
			}
		}
		if got := f.Score(zeroAt); math.Abs(got) > 1e-9 {
			t.Fatalf("curve %d: Score(zeroAt=%v) = %v, want 0", n, zeroAt, got)
		}

		// Steepest slope anywhere on the curve bounds how far one step can move.
		maxSlope := 0.0
		prevD, prevS := 0.0, Peak
		for _, k := range knots {
			maxSlope = math.Max(maxSlope, (prevS-k.Score)/(k.D-prevD))
			prevD, prevS = k.D, k.Score
		}
		maxSlope = math.Max(maxSlope, (prevS-floor)/f.h)

		// Dense sweep from 0 to 40x the zero crossing.
		const steps = 20000
		span := zeroAt * 40
		step := span / steps
		prev := f.Score(0)
		for i := 1; i <= steps; i++ {
			x := float64(i) * step
			cur := f.Score(x)
			if !(cur < prev) {
				t.Fatalf("curve %d: not strictly decreasing at d=%v: %v then %v", n, x, prev, cur)
			}
			if !(cur > floor) {
				t.Fatalf("curve %d: Score(%v) = %v reached floor %v", n, x, cur, floor)
			}
			if drop := prev - cur; drop > maxSlope*step*(1+1e-9)+1e-9 {
				t.Fatalf("curve %d: jump of %v at d=%v exceeds slope bound %v", n, drop, x, maxSlope*step)
			}
			prev = cur
		}

		// Far out, still distinguishable and still above the floor.
		far, farther := f.Score(zeroAt*1e4), f.Score(zeroAt*1e5)
		if !(farther < far && farther > floor) {
			t.Fatalf("curve %d: tail not strictly decreasing above floor: %v, %v (floor %v)", n, far, farther, floor)
		}
	}
}

func TestSaturating(t *testing.T) {
	if got := Saturating(10, 2, 0); got != 0 {
		t.Errorf("Saturating at 0 = %v, want 0", got)
	}
	if got := Saturating(10, 2, -1); got != 0 {
		t.Errorf("Saturating at negative x = %v, want 0", got)
	}
	if got := Saturating(10, 2, 2); math.Abs(got-5) > 1e-9 {
		t.Errorf("Saturating at half = %v, want 5", got)
	}
	prev := 0.0
	for i := 1; i <= 10000; i++ {
		x := float64(i) * 0.01
		cur := Saturating(10, 2, x)
		if !(cur > prev) || !(cur < 10) {
			t.Fatalf("Saturating(10, 2, %v) = %v: must rise strictly and stay below max (prev %v)", x, cur, prev)
		}
		prev = cur
	}
}

func TestStemPenalty(t *testing.T) {
	checks := []struct {
		stem, min, max float64
		wantPen, wantU float64
	}{
		{80, 60, 100, 0, 0},
		{60, 60, 100, 15, 1},
		{100, 60, 100, 15, 1},
		{90, 60, 100, 3.75, 0.5},
		{110, 60, 100, 15 * (1 + 2*0.25 + 0.5), 1.5},
		{80, 0, 100, 0, 0},   // no usable range
		{80, 100, 100, 0, 0}, // empty range
	}
	for _, c := range checks {
		pen, u := StemPenalty(c.stem, c.min, c.max)
		if math.Abs(pen-c.wantPen) > 1e-9 || math.Abs(u-c.wantU) > 1e-9 {
			t.Errorf("StemPenalty(%v, %v, %v) = (%v, %v), want (%v, %v)", c.stem, c.min, c.max, pen, u, c.wantPen, c.wantU)
		}
	}

	// Continuous and strictly increasing away from the midpoint, across u = 1.
	prev, _ := StemPenalty(80, 60, 100)
	for i := 1; i <= 6000; i++ {
		stem := 80 + float64(i)*0.01
		cur, _ := StemPenalty(stem, 60, 100)
		if !(cur > prev) {
			t.Fatalf("StemPenalty not strictly increasing at stem=%v: %v then %v", stem, prev, cur)
		}
		if cur-prev > 0.1 {
			t.Fatalf("StemPenalty jumps by %v at stem=%v", cur-prev, stem)
		}
		prev = cur
	}
}

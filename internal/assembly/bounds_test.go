package assembly_test

import (
	"encoding/json"
	"errors"
	"math"
	"strings"
	"testing"

	"github.com/matta/bikegeo/internal/assembly"
	"github.com/matta/bikegeo/internal/pose"
)

func assertPanics(t *testing.T, name string, fn func()) {
	t.Helper()
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("%s: expected panic, got none", name)
		}
	}()
	fn()
}

func TestLoadState(t *testing.T) {
	if !assembly.Static.Valid() || !assembly.Dynamic.Valid() {
		t.Fatal("Static and Dynamic must be valid")
	}
	if assembly.LoadState("").Valid() || assembly.LoadState("unknown").Valid() {
		t.Fatal("empty or unknown LoadState must be invalid")
	}
	if got := assembly.Static.WorldSystem(); got != assembly.StaticWorld {
		t.Errorf("Static.WorldSystem() = %v, want %v", got, assembly.StaticWorld)
	}
	if got := assembly.Dynamic.WorldSystem(); got != assembly.DynamicWorld {
		t.Errorf("Dynamic.WorldSystem() = %v, want %v", got, assembly.DynamicWorld)
	}
	assertPanics(t, "empty LoadState WorldSystem", func() {
		_ = assembly.LoadState("").WorldSystem()
	})

	if got, ok := assembly.LoadStateForWorld(assembly.StaticWorld); !ok || got != assembly.Static {
		t.Errorf("LoadStateForWorld(StaticWorld) = (%q, %v), want (%q, true)", got, ok, assembly.Static)
	}
	if got, ok := assembly.LoadStateForWorld(assembly.DynamicWorld); !ok || got != assembly.Dynamic {
		t.Errorf("LoadStateForWorld(DynamicWorld) = (%q, %v), want (%q, true)", got, ok, assembly.Dynamic)
	}
	customSys := pose.NewCoordinateSystem("test:custom_world")
	for _, sys := range []pose.CoordinateSystem{assembly.UnpitchedChart, assembly.FrameLocal, customSys, {}} {
		if got, ok := assembly.LoadStateForWorld(sys); ok || got != "" {
			t.Errorf("LoadStateForWorld(%v) = (%q, %v), want (\"\", false)", sys, got, ok)
		}
	}
}

func TestBoundedRangeValidationAndOperations(t *testing.T) {
	r, err := assembly.NewBoundedRange(100, 290, "seatpost min insertion", "frame max insertion")
	if err != nil {
		t.Fatalf("NewBoundedRange: %v", err)
	}
	if !r.Valid() {
		t.Fatal("constructed range must be valid")
	}
	if r.Min() != 100 || r.Max() != 290 {
		t.Errorf("range = [%v, %v], want [100, 290]", r.Min(), r.Max())
	}
	if r.MinSource() != "seatpost min insertion" || r.MaxSource() != "frame max insertion" {
		t.Errorf("sources = (%q, %q)", r.MinSource(), r.MaxSource())
	}
	if r.Midpoint() != 195 {
		t.Errorf("Midpoint = %v, want 195", r.Midpoint())
	}
	if r.Clamp(50) != 100 || r.Clamp(180) != 180 || r.Clamp(310) != 290 {
		t.Errorf("Clamp unexpected: %v, %v, %v", r.Clamp(50), r.Clamp(180), r.Clamp(310))
	}
	if !math.IsNaN(r.Clamp(math.NaN())) {
		t.Error("Clamp(NaN) should remain NaN")
	}

	// Equal endpoints and one-sided unbounded ranges are valid.
	pinned, err := assembly.NewBoundedRange(250, 250, "a", "b")
	if err != nil || pinned.Midpoint() != 250 || pinned.Clamp(100) != 250 {
		t.Fatalf("pinned = %+v, %v", pinned, err)
	}
	unbounded, err := assembly.NewBoundedRange(0, math.Inf(1), "non-negative", "unbounded")
	if err != nil {
		t.Fatalf("unbounded: %v", err)
	}
	if !math.IsInf(unbounded.Midpoint(), 1) {
		t.Errorf("unbounded Midpoint = %v, want +Inf", unbounded.Midpoint())
	}
	if unbounded.Clamp(-5) != 0 || unbounded.Clamp(1e9) != 1e9 {
		t.Errorf("unbounded Clamp unexpected: %v, %v", unbounded.Clamp(-5), unbounded.Clamp(1e9))
	}
	if _, err := unbounded.Bind(assembly.InputSeatpostSetback, 500); err != nil {
		t.Errorf("unbounded Bind(500): %v", err)
	}
	var inputErr *assembly.InputError
	if _, err := unbounded.Bind(assembly.InputSeatpostSetback, math.Inf(1)); !errors.As(err, &inputErr) || inputErr.Field != assembly.InputSeatpostSetback {
		t.Errorf("unbounded Bind(+Inf) = %v, want InputSeatpostSetback", err)
	}

	doublyUnbounded, err := assembly.NewBoundedRange(math.Inf(-1), math.Inf(1), "none", "none")
	if err != nil || doublyUnbounded.Midpoint() != 0 {
		t.Fatalf("doubly unbounded = %+v, %v", doublyUnbounded, err)
	}

	// Overflow-safe Midpoint across opposite-sign and same-sign finite extremes.
	for _, tc := range []struct {
		name     string
		min, max float64
		want     float64
	}{
		{"symmetric MaxFloat64", -math.MaxFloat64, math.MaxFloat64, 0},
		{"asymmetric negative extreme", -math.MaxFloat64, 1, (-math.MaxFloat64 + 1) / 2},
		{"asymmetric positive extreme", -1, math.MaxFloat64, (-1 + math.MaxFloat64) / 2},
		{"positive extreme singleton", math.MaxFloat64, math.MaxFloat64, math.MaxFloat64},
		{"negative extreme singleton", -math.MaxFloat64, -math.MaxFloat64, -math.MaxFloat64},
		{"zero to MaxFloat64", 0, math.MaxFloat64, math.MaxFloat64 / 2},
		{"-MaxFloat64 to zero", -math.MaxFloat64, 0, -math.MaxFloat64 / 2},
	} {
		rng, err := assembly.NewBoundedRange(tc.min, tc.max, "lo", "hi")
		if err != nil {
			t.Fatalf("%s: NewBoundedRange: %v", tc.name, err)
		}
		got := rng.Midpoint()
		if math.IsInf(got, 0) || math.IsNaN(got) || got != tc.want {
			t.Errorf("%s: Midpoint() = %v, want %v", tc.name, got, tc.want)
		}
	}

	for _, tc := range []struct {
		name     string
		min, max float64
	}{
		{"NaN min", math.NaN(), 10},
		{"NaN max", 0, math.NaN()},
		{"min > max", 20, 10},
		{"+Inf min", math.Inf(1), math.Inf(1)},
		{"-Inf max", math.Inf(-1), math.Inf(-1)},
	} {
		if _, err := assembly.NewBoundedRange(tc.min, tc.max, "lo", "hi"); err == nil {
			t.Errorf("%s: want error", tc.name)
		}
	}

	var zero assembly.BoundedRange
	if zero.Valid() {
		t.Error("zero BoundedRange must not be valid")
	}
	assertPanics(t, "zero Min", func() { _ = zero.Min() })
	assertPanics(t, "zero Max", func() { _ = zero.Max() })
	assertPanics(t, "zero Midpoint", func() { _ = zero.Midpoint() })
	assertPanics(t, "zero Clamp", func() { _ = zero.Clamp(0) })
	assertPanics(t, "zero Bind", func() { _, _ = zero.Bind(assembly.InputSeatpostInsertion, 0) })
}

func TestIntersectRangesPreservesTighterSource(t *testing.T) {
	post, err := assembly.NewBoundedRange(100, 365, "seatpost min insertion", "seatpost max insertion")
	if err != nil {
		t.Fatalf("post range: %v", err)
	}
	frame, err := assembly.NewBoundedRange(0, 290, "frame min insertion", "frame max insertion")
	if err != nil {
		t.Fatalf("frame range: %v", err)
	}
	got, ok := assembly.IntersectRanges(post, frame)
	if !ok {
		t.Fatal("expected non-empty intersection")
	}
	if got.Min() != 100 || got.Max() != 290 {
		t.Errorf("intersection = [%v, %v], want [100, 290]", got.Min(), got.Max())
	}
	if got.MinSource() != "seatpost min insertion" || got.MaxSource() != "frame max insertion" {
		t.Errorf("sources = (%q, %q), want (seatpost min insertion, frame max insertion)", got.MinSource(), got.MaxSource())
	}

	// Single-point overlap.
	a, _ := assembly.NewBoundedRange(100, 200, "a min", "a max")
	b, _ := assembly.NewBoundedRange(200, 300, "b min", "b max")
	single, ok := assembly.IntersectRanges(a, b)
	if !ok || single.Min() != 200 || single.Max() != 200 || single.MinSource() != "b min" || single.MaxSource() != "a max" {
		t.Errorf("single-point intersection = %+v, ok=%v", single, ok)
	}

	// Disjoint ranges.
	c, _ := assembly.NewBoundedRange(0, 90, "frame min", "frame max")
	if _, ok := assembly.IntersectRanges(post, c); ok {
		t.Error("disjoint ranges should return ok=false")
	}

	// Zero range.
	var zero assembly.BoundedRange
	if _, ok := assembly.IntersectRanges(post, zero); ok {
		t.Error("intersecting with zero range should return ok=false")
	}
}

func TestBoundStatusMarginsAndEndpoints(t *testing.T) {
	r, err := assembly.NewBoundedRange(100, 250, "seatpost min insertion", "seatpost max insertion")
	if err != nil {
		t.Fatalf("NewBoundedRange: %v", err)
	}

	status, err := r.Bind(assembly.InputSeatpostInsertion, 125)
	if err != nil {
		t.Fatalf("Bind(125): %v", err)
	}
	if status.Field != assembly.InputSeatpostInsertion || status.Value != 125 ||
		status.Min != 100 || status.Max != 250 ||
		status.MinSource != "seatpost min insertion" || status.MaxSource != "seatpost max insertion" {
		t.Errorf("unexpected status: %+v", status)
	}
	assertNear(t, "MarginMm", status.MarginMm(), 25, 1e-12)
	if !status.WithinMargin(30) || !status.WithinMargin(25) || status.WithinMargin(24.999) {
		t.Error("WithinMargin mismatch at margin 25 mm")
	}
	if status.AtMin(1e-6) || status.AtMax(1e-6) {
		t.Error("interior value should not be AtMin or AtMax")
	}

	atMin, err := r.Bind(assembly.InputSeatpostInsertion, 100)
	if err != nil {
		t.Fatalf("Bind(100): %v", err)
	}
	if atMin.MarginMm() != 0 || !atMin.AtMin(0) || atMin.AtMax(1e-6) {
		t.Errorf("atMin status unexpected: %+v", atMin)
	}

	atMax, err := r.Bind(assembly.InputSeatpostInsertion, 250)
	if err != nil {
		t.Fatalf("Bind(250): %v", err)
	}
	if atMax.MarginMm() != 0 || atMax.AtMin(1e-6) || !atMax.AtMax(0) {
		t.Errorf("atMax status unexpected: %+v", atMax)
	}

	for _, bad := range []float64{99.999, 250.001, math.NaN(), math.Inf(1)} {
		var inputErr *assembly.InputError
		if _, err := r.Bind(assembly.InputSeatpostInsertion, bad); !errors.As(err, &inputErr) || inputErr.Field != assembly.InputSeatpostInsertion {
			t.Errorf("Bind(%v) = %v, want InputSeatpostInsertion", bad, err)
		}
	}
}

func TestBoundStatusJSON(t *testing.T) {
	for _, tc := range []struct {
		name           string
		min, max, val  float64
		minSrc, maxSrc string
		wantContains   []string
	}{
		{
			name:         "finite bounds",
			min:          100,
			max:          250,
			val:          125,
			minSrc:       "seatpost min insertion",
			maxSrc:       "seatpost max insertion",
			wantContains: []string{`"Min":100`, `"Max":250`},
		},
		{
			name:         "one-sided unbounded above",
			min:          0,
			max:          math.Inf(1),
			val:          15,
			minSrc:       "non-negative",
			maxSrc:       "unbounded",
			wantContains: []string{`"Min":0`, `"Max":null`},
		},
		{
			name:         "one-sided unbounded below",
			min:          math.Inf(-1),
			max:          100,
			val:          15,
			minSrc:       "unbounded",
			maxSrc:       "upper limit",
			wantContains: []string{`"Min":null`, `"Max":100`},
		},
		{
			name:         "doubly unbounded",
			min:          math.Inf(-1),
			max:          math.Inf(1),
			val:          15,
			minSrc:       "none",
			maxSrc:       "none",
			wantContains: []string{`"Min":null`, `"Max":null`},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r, err := assembly.NewBoundedRange(tc.min, tc.max, tc.minSrc, tc.maxSrc)
			if err != nil {
				t.Fatalf("NewBoundedRange: %v", err)
			}
			want, err := r.Bind(assembly.InputSeatpostSetback, tc.val)
			if err != nil {
				t.Fatalf("Bind: %v", err)
			}
			raw, err := json.Marshal(want)
			if err != nil {
				t.Fatalf("json.Marshal: %v", err)
			}
			for _, sub := range tc.wantContains {
				if !strings.Contains(string(raw), sub) {
					t.Errorf("JSON %s missing %s", raw, sub)
				}
			}
			var got assembly.BoundStatus
			if err := json.Unmarshal(raw, &got); err != nil {
				t.Fatalf("json.Unmarshal: %v", err)
			}
			if got != want {
				t.Errorf("round-trip = %+v, want %+v", got, want)
			}
		})
	}

	// JSON null is a no-op per encoding/json Unmarshaler conventions.
	existing := assembly.BoundStatus{Field: assembly.InputSeatpostSetback, Value: 15, Min: 0, Max: 50}
	if err := json.Unmarshal([]byte(" null "), &existing); err != nil {
		t.Fatalf("Unmarshal null: %v", err)
	}
	if existing.Min != 0 || existing.Max != 50 {
		t.Errorf("Unmarshal null mutated existing status: %+v", existing)
	}

	// Corrupted non-finite fields still fail MarshalJSON rather than silently becoming null.
	for _, tc := range []struct {
		name   string
		status assembly.BoundStatus
	}{
		{"+Inf Min", assembly.BoundStatus{Value: 10, Min: math.Inf(1), Max: 20}},
		{"-Inf Max", assembly.BoundStatus{Value: 10, Min: 0, Max: math.Inf(-1)}},
		{"NaN Min", assembly.BoundStatus{Value: 10, Min: math.NaN(), Max: 20}},
		{"NaN Max", assembly.BoundStatus{Value: 10, Min: 0, Max: math.NaN()}},
		{"+Inf Value", assembly.BoundStatus{Value: math.Inf(1), Min: 0, Max: 20}},
	} {
		if _, err := json.Marshal(tc.status); err == nil {
			t.Errorf("%s: json.Marshal succeeded, want error", tc.name)
		}
	}
}

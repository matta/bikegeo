package bounds_test

import (
	"encoding/json"
	"errors"
	"math"
	"strings"
	"testing"

	"github.com/matta/bikegeo/internal/bounds"
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

func assertNear(t *testing.T, label string, got, want, tol float64) {
	t.Helper()
	if math.IsNaN(got) || math.Abs(got-want) > tol {
		t.Errorf("%s = %.9f, want %.9f (diff %.3e > tol %.3e)", label, got, want, got-want, tol)
	}
}

func TestLoadState(t *testing.T) {
	if !bounds.Static.Valid() || !bounds.Dynamic.Valid() {
		t.Fatal("Static and Dynamic must be valid")
	}
	if bounds.LoadState("").Valid() || bounds.LoadState("unknown").Valid() {
		t.Fatal("empty or unknown LoadState must be invalid")
	}
	if got := bounds.Static.WorldSystem(); got != bounds.StaticWorld {
		t.Errorf("Static.WorldSystem() = %v, want %v", got, bounds.StaticWorld)
	}
	if got := bounds.Dynamic.WorldSystem(); got != bounds.DynamicWorld {
		t.Errorf("Dynamic.WorldSystem() = %v, want %v", got, bounds.DynamicWorld)
	}
	assertPanics(t, "empty LoadState WorldSystem", func() {
		_ = bounds.LoadState("").WorldSystem()
	})

	if got, ok := bounds.LoadStateForWorld(bounds.StaticWorld); !ok || got != bounds.Static {
		t.Errorf("LoadStateForWorld(StaticWorld) = (%q, %v), want (%q, true)", got, ok, bounds.Static)
	}
	if got, ok := bounds.LoadStateForWorld(bounds.DynamicWorld); !ok || got != bounds.Dynamic {
		t.Errorf("LoadStateForWorld(DynamicWorld) = (%q, %v), want (%q, true)", got, ok, bounds.Dynamic)
	}
	partSys := pose.NewCoordinateSystem("test:part_local")
	customSys := pose.NewCoordinateSystem("test:custom_world")
	for _, sys := range []pose.CoordinateSystem{bounds.UnpitchedChart, partSys, customSys, {}} {
		if got, ok := bounds.LoadStateForWorld(sys); ok || got != "" {
			t.Errorf("LoadStateForWorld(%v) = (%q, %v), want (\"\", false)", sys, got, ok)
		}
	}
}

func TestBoundedRangeValidationAndOperations(t *testing.T) {
	r, err := bounds.NewBoundedRange(100, 290, "seatpost min insertion", "frame max insertion")
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
	pinned, err := bounds.NewBoundedRange(250, 250, "a", "b")
	if err != nil || pinned.Midpoint() != 250 || pinned.Clamp(100) != 250 {
		t.Fatalf("pinned = %+v, %v", pinned, err)
	}
	unbounded, err := bounds.NewBoundedRange(0, math.Inf(1), "non-negative", "unbounded")
	if err != nil {
		t.Fatalf("unbounded: %v", err)
	}
	if !math.IsInf(unbounded.Midpoint(), 1) {
		t.Errorf("unbounded Midpoint = %v, want +Inf", unbounded.Midpoint())
	}
	if unbounded.Clamp(-5) != 0 || unbounded.Clamp(1e9) != 1e9 {
		t.Errorf("unbounded Clamp unexpected: %v, %v", unbounded.Clamp(-5), unbounded.Clamp(1e9))
	}
	if _, err := unbounded.Bind(bounds.InputSeatpostSetback, 500); err != nil {
		t.Errorf("unbounded Bind(500): %v", err)
	}
	var inputErr *bounds.InputError
	if _, err := unbounded.Bind(bounds.InputSeatpostSetback, math.Inf(1)); !errors.As(err, &inputErr) || inputErr.Field != bounds.InputSeatpostSetback {
		t.Errorf("unbounded Bind(+Inf) = %v, want InputSeatpostSetback", err)
	}

	doublyUnbounded, err := bounds.NewBoundedRange(math.Inf(-1), math.Inf(1), "none", "none")
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
		rng, err := bounds.NewBoundedRange(tc.min, tc.max, "lo", "hi")
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
		if _, err := bounds.NewBoundedRange(tc.min, tc.max, "lo", "hi"); err == nil {
			t.Errorf("%s: want error", tc.name)
		}
	}

	var zero bounds.BoundedRange
	if zero.Valid() {
		t.Error("zero BoundedRange must not be valid")
	}
	assertPanics(t, "zero Min", func() { _ = zero.Min() })
	assertPanics(t, "zero Max", func() { _ = zero.Max() })
	assertPanics(t, "zero Midpoint", func() { _ = zero.Midpoint() })
	assertPanics(t, "zero Clamp", func() { _ = zero.Clamp(0) })
	assertPanics(t, "zero Bind", func() { _, _ = zero.Bind(bounds.InputSeatpostInsertion, 0) })
}

func TestIntersectRangesPreservesTighterSource(t *testing.T) {
	post, err := bounds.NewBoundedRange(100, 365, "seatpost min insertion", "seatpost max insertion")
	if err != nil {
		t.Fatalf("post range: %v", err)
	}
	frame, err := bounds.NewBoundedRange(0, 290, "frame min insertion", "frame max insertion")
	if err != nil {
		t.Fatalf("frame range: %v", err)
	}
	got, ok := bounds.IntersectRanges(post, frame)
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
	a, _ := bounds.NewBoundedRange(100, 200, "a min", "a max")
	b, _ := bounds.NewBoundedRange(200, 300, "b min", "b max")
	single, ok := bounds.IntersectRanges(a, b)
	if !ok || single.Min() != 200 || single.Max() != 200 || single.MinSource() != "b min" || single.MaxSource() != "a max" {
		t.Errorf("single-point intersection = %+v, ok=%v", single, ok)
	}

	// Disjoint ranges.
	c, _ := bounds.NewBoundedRange(0, 90, "frame min", "frame max")
	if _, ok := bounds.IntersectRanges(post, c); ok {
		t.Error("disjoint ranges should return ok=false")
	}

	// Zero range.
	var zero bounds.BoundedRange
	if _, ok := bounds.IntersectRanges(post, zero); ok {
		t.Error("intersecting with zero range should return ok=false")
	}
}

func TestBoundStatusMarginsAndEndpoints(t *testing.T) {
	r, err := bounds.NewBoundedRange(100, 250, "seatpost min insertion", "seatpost max insertion")
	if err != nil {
		t.Fatalf("NewBoundedRange: %v", err)
	}

	status, err := r.Bind(bounds.InputSeatpostInsertion, 125)
	if err != nil {
		t.Fatalf("Bind(125): %v", err)
	}
	if status.Field != bounds.InputSeatpostInsertion || status.Value != 125 ||
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

	atMin, err := r.Bind(bounds.InputSeatpostInsertion, 100)
	if err != nil {
		t.Fatalf("Bind(100): %v", err)
	}
	if atMin.MarginMm() != 0 || !atMin.AtMin(0) || atMin.AtMax(1e-6) {
		t.Errorf("atMin status unexpected: %+v", atMin)
	}

	atMax, err := r.Bind(bounds.InputSeatpostInsertion, 250)
	if err != nil {
		t.Fatalf("Bind(250): %v", err)
	}
	if atMax.MarginMm() != 0 || atMax.AtMin(1e-6) || !atMax.AtMax(0) {
		t.Errorf("atMax status unexpected: %+v", atMax)
	}

	for _, bad := range []float64{99.999, 250.001, math.NaN(), math.Inf(1)} {
		var inputErr *bounds.InputError
		if _, err := r.Bind(bounds.InputSeatpostInsertion, bad); !errors.As(err, &inputErr) || inputErr.Field != bounds.InputSeatpostInsertion {
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
			r, err := bounds.NewBoundedRange(tc.min, tc.max, tc.minSrc, tc.maxSrc)
			if err != nil {
				t.Fatalf("NewBoundedRange: %v", err)
			}
			want, err := r.Bind(bounds.InputSeatpostSetback, tc.val)
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
			var got bounds.BoundStatus
			if err := json.Unmarshal(raw, &got); err != nil {
				t.Fatalf("json.Unmarshal: %v", err)
			}
			if got != want {
				t.Errorf("round-trip = %+v, want %+v", got, want)
			}
		})
	}

	// JSON null is a no-op per encoding/json Unmarshaler conventions.
	existing := bounds.BoundStatus{Field: bounds.InputSeatpostSetback, Value: 15, Min: 0, Max: 50}
	if err := json.Unmarshal([]byte(" null "), &existing); err != nil {
		t.Fatalf("Unmarshal null: %v", err)
	}
	if existing.Min != 0 || existing.Max != 50 {
		t.Errorf("Unmarshal null mutated existing status: %+v", existing)
	}

	// Corrupted non-finite fields still fail MarshalJSON rather than silently becoming null.
	for _, tc := range []struct {
		name   string
		status bounds.BoundStatus
	}{
		{"+Inf Min", bounds.BoundStatus{Value: 10, Min: math.Inf(1), Max: 20}},
		{"-Inf Max", bounds.BoundStatus{Value: 10, Min: 0, Max: math.Inf(-1)}},
		{"NaN Min", bounds.BoundStatus{Value: 10, Min: math.NaN(), Max: 20}},
		{"NaN Max", bounds.BoundStatus{Value: 10, Min: 0, Max: math.NaN()}},
		{"+Inf Value", bounds.BoundStatus{Value: math.Inf(1), Min: 0, Max: 20}},
	} {
		if _, err := json.Marshal(tc.status); err == nil {
			t.Errorf("%s: json.Marshal succeeded, want error", tc.name)
		}
	}
}

func TestSlideJointBindAndTransformAt(t *testing.T) {
	parentSys := pose.NewCoordinateSystem("test:slide_parent")
	childSys := pose.NewCoordinateSystem("test:slide_child")

	t.Run("negative X direction", func(t *testing.T) {
		allowed, err := bounds.NewBoundedRange(100, 290, "post min", "frame max")
		if err != nil {
			t.Fatalf("NewBoundedRange: %v", err)
		}
		originAtZero := pose.NewPoint(parentSys, 400, 0)
		step := pose.NewVector(parentSys, -1, 0)
		joint := bounds.NewSlideJoint(bounds.InputSeatpostInsertion, allowed, originAtZero, step, 0, childSys)

		if joint.Field() != bounds.InputSeatpostInsertion {
			t.Errorf("Field() = %q, want %q", joint.Field(), bounds.InputSeatpostInsertion)
		}
		if joint.Allowed() != allowed {
			t.Errorf("Allowed() = %+v, want %+v", joint.Allowed(), allowed)
		}
		if joint.StepPerMm() != step {
			t.Errorf("StepPerMm() = %+v, want %+v", joint.StepPerMm(), step)
		}
		if joint.ParentSystem() != parentSys || joint.ChildSystem() != childSys {
			t.Errorf("systems = (%v, %v), want (%v, %v)", joint.ParentSystem(), joint.ChildSystem(), parentSys, childSys)
		}

		// TransformAt works for probe values both inside and outside Allowed.
		probe := joint.TransformAt(50)
		if probe.Src() != childSys || probe.Dst() != parentSys {
			t.Errorf("probe systems = (%v -> %v), want (%v -> %v)", probe.Src(), probe.Dst(), childSys, parentSys)
		}
		assertNear(t, "probe.X", probe.OriginInDst().X(), 350, 1e-12)
		assertNear(t, "probe.Y", probe.OriginInDst().Y(), 0, 1e-12)
		assertNear(t, "probe.Heading", float64(probe.HeadingInDst().Deg()), 0, 1e-12)

		// Bind succeeds in range.
		tr, status, err := joint.Bind(150)
		if err != nil {
			t.Fatalf("Bind(150): %v", err)
		}
		assertNear(t, "bound.X", tr.OriginInDst().X(), 250, 1e-12)
		assertNear(t, "bound.Y", tr.OriginInDst().Y(), 0, 1e-12)
		if status.Field != bounds.InputSeatpostInsertion || status.Value != 150 || status.Min != 100 || status.Max != 290 {
			t.Errorf("unexpected status: %+v", status)
		}

		// Bind rejects out-of-range and non-finite values with attributed *InputError.
		for _, bad := range []float64{99.9, 290.1, math.NaN(), math.Inf(1)} {
			var inputErr *bounds.InputError
			if _, _, err := joint.Bind(bad); !errors.As(err, &inputErr) || inputErr.Field != bounds.InputSeatpostInsertion {
				t.Errorf("Bind(%v) = %v, want InputSeatpostInsertion", bad, err)
			}
		}
	})

	t.Run("positive X direction", func(t *testing.T) {
		allowed, err := bounds.NewBoundedRange(0, 40, "non-negative", "max spacers")
		if err != nil {
			t.Fatalf("NewBoundedRange: %v", err)
		}
		originAtZero := pose.NewPoint(parentSys, 12, 0)
		step := pose.NewVector(parentSys, 1, 0)
		joint := bounds.NewSlideJoint(bounds.InputStemSpacers, allowed, originAtZero, step, 0, childSys)

		tr, status, err := joint.Bind(25)
		if err != nil {
			t.Fatalf("Bind(25): %v", err)
		}
		assertNear(t, "bound.X", tr.OriginInDst().X(), 37, 1e-12)
		assertNear(t, "bound.Y", tr.OriginInDst().Y(), 0, 1e-12)
		assertNear(t, "status.MarginMm", status.MarginMm(), 15, 1e-12)

		var inputErr *bounds.InputError
		if _, _, err := joint.Bind(-0.1); !errors.As(err, &inputErr) || inputErr.Field != bounds.InputStemSpacers {
			t.Errorf("Bind(-0.1) = %v, want InputStemSpacers", err)
		}
	})

	t.Run("rotated direction and child turn", func(t *testing.T) {
		allowed, err := bounds.NewBoundedRange(35, 120, "min stem", "max stem")
		if err != nil {
			t.Fatalf("NewBoundedRange: %v", err)
		}
		turn := pose.Deg(-84) // e.g., -90° + 6° stem angle in steerer coordinates
		originAtZero := pose.NewPoint(parentSys, 50, 0)
		step := pose.Along(pose.NewHeading(parentSys, turn), 1)
		joint := bounds.NewSlideJoint(bounds.InputStemLength, allowed, originAtZero, step, turn, childSys)

		tr, status, err := joint.Bind(80)
		if err != nil {
			t.Fatalf("Bind(80): %v", err)
		}
		rad := float64(turn) * math.Pi / 180
		wantX := 50 + 80*math.Cos(rad)
		wantY := 80 * math.Sin(rad)
		assertNear(t, "origin.X", tr.OriginInDst().X(), wantX, 1e-12)
		assertNear(t, "origin.Y", tr.OriginInDst().Y(), wantY, 1e-12)
		assertNear(t, "heading", float64(tr.HeadingInDst().Deg()), float64(turn), 1e-12)
		if status.Value != 80 {
			t.Errorf("status.Value = %v, want 80", status.Value)
		}

		// Verify a child-local offset rotates by turnDeg around the translated origin.
		childPt := pose.NewPoint(childSys, 10, 5)
		gotPt := tr.ApplyPoint(childPt)
		assertNear(t, "childPt.X", gotPt.X(), wantX+10*math.Cos(rad)-5*math.Sin(rad), 1e-12)
		assertNear(t, "childPt.Y", gotPt.Y(), wantY+10*math.Sin(rad)+5*math.Cos(rad), 1e-12)
	})
}

func TestSlideJointValidationPanics(t *testing.T) {
	parentSys := pose.NewCoordinateSystem("test:panic_parent")
	childSys := pose.NewCoordinateSystem("test:panic_child")
	otherSys := pose.NewCoordinateSystem("test:panic_other")
	allowed, err := bounds.NewBoundedRange(0, 50, "lo", "hi")
	if err != nil {
		t.Fatalf("NewBoundedRange: %v", err)
	}
	origin := pose.Origin(parentSys)
	unitX := pose.NewVector(parentSys, 1, 0)

	// Zero-value SlideJoint panics on every method.
	var zero bounds.SlideJoint
	assertPanics(t, "zero Field", func() { _ = zero.Field() })
	assertPanics(t, "zero Allowed", func() { _ = zero.Allowed() })
	assertPanics(t, "zero StepPerMm", func() { _ = zero.StepPerMm() })
	assertPanics(t, "zero ParentSystem", func() { _ = zero.ParentSystem() })
	assertPanics(t, "zero ChildSystem", func() { _ = zero.ChildSystem() })
	assertPanics(t, "zero TransformAt", func() { _ = zero.TransformAt(0) })
	assertPanics(t, "zero Bind", func() { _, _, _ = zero.Bind(0) })

	// Invalid constructor arguments panic deterministically.
	var zeroRange bounds.BoundedRange
	var zeroSys pose.CoordinateSystem
	var zeroPt pose.Point
	var zeroVec pose.Vector

	assertPanics(t, "invalid BoundedRange", func() {
		_ = bounds.NewSlideJoint(bounds.InputStemSpacers, zeroRange, origin, unitX, 0, childSys)
	})
	assertPanics(t, "uninitialized originAtZero", func() {
		_ = bounds.NewSlideJoint(bounds.InputStemSpacers, allowed, zeroPt, unitX, 0, childSys)
	})
	assertPanics(t, "uninitialized stepPerMm", func() {
		_ = bounds.NewSlideJoint(bounds.InputStemSpacers, allowed, origin, zeroVec, 0, childSys)
	})
	assertPanics(t, "mismatched origin and step systems", func() {
		_ = bounds.NewSlideJoint(bounds.InputStemSpacers, allowed, origin, pose.NewVector(otherSys, 1, 0), 0, childSys)
	})
	assertPanics(t, "uninitialized childSystem", func() {
		_ = bounds.NewSlideJoint(bounds.InputStemSpacers, allowed, origin, unitX, 0, zeroSys)
	})
	assertPanics(t, "identical parent and child systems", func() {
		_ = bounds.NewSlideJoint(bounds.InputStemSpacers, allowed, origin, unitX, 0, parentSys)
	})

	for _, tc := range []struct {
		name   string
		dx, dy float64
	}{
		{"zero vector", 0, 0},
		{"too long", 2, 0},
		{"too short", 0.5, 0},
		{"outside tolerance", 1 + 1e-8, 0},
		{"NaN", math.NaN(), 0},
		{"Inf", math.Inf(1), 0},
	} {
		assertPanics(t, "non-unit stepPerMm "+tc.name, func() {
			_ = bounds.NewSlideJoint(bounds.InputStemSpacers, allowed, origin, pose.NewVector(parentSys, tc.dx, tc.dy), 0, childSys)
		})
	}
}

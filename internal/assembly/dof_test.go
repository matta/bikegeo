package assembly_test

import (
	"math"
	"slices"
	"testing"

	"github.com/matta/bikegeo/internal/assembly"
	"github.com/matta/bikegeo/internal/bounds"
	"github.com/matta/bikegeo/internal/pose"
)

func mustRange(t *testing.T, lo, hi float64) bounds.BoundedRange {
	t.Helper()
	r, err := bounds.NewBoundedRange(lo, hi, "min", "max")
	if err != nil {
		t.Fatalf("NewBoundedRange(%v, %v): %v", lo, hi, err)
	}
	return r
}

func TestSolvePlanar2DoFInteriorEdgeCornerAndUnbounded(t *testing.T) {
	// Orthogonal axes in DynamicWorld: AxisA along +Y (90°, Sign -1, range [100, 200], Probe 200),
	// AxisB along +X (0°, Sign +1, range [-15, 15], Probe 0), Origin = (0, 500).
	// Parameter (valA, valB) maps to (valB, 500 + (200 - valA)).
	sheet := assembly.Planar2DoF{
		Origin: pose.NewPoint(bounds.DynamicWorld, 0, 500),
		AxisA: assembly.LinearDoF{
			Field:   bounds.InputSeatpostInsertion,
			Heading: pose.NewHeading(bounds.DynamicWorld, 90),
			Range:   mustRange(t, 100, 200),
			Probe:   200,
			Sign:    -1,
		},
		AxisB: assembly.LinearDoF{
			Field:   bounds.InputSaddleRailOffset,
			Heading: pose.NewHeading(bounds.DynamicWorld, 0),
			Range:   mustRange(t, -15, 15),
			Probe:   0,
			Sign:    +1,
		},
	}

	// 1. Interior reachable target: (7, 540) => valA = 160, valB = 7, limited = nil.
	va, vb, limited, ok := assembly.SolvePlanar2DoF(sheet, pose.NewPoint(bounds.DynamicWorld, 7, 540), assembly.PriorityNearest)
	if !ok || limited != nil {
		t.Fatalf("interior: ok=%v limited=%v, want ok=true limited=nil", ok, limited)
	}
	assertNear(t, "interior valA", va, 160, 1e-12)
	assertNear(t, "interior valB", vb, 7, 1e-12)

	// 2. Single-axis edge limited: (30, 540) => valA = 160, valB = 15, limited = [RailOffset].
	va, vb, limited, ok = assembly.SolvePlanar2DoF(sheet, pose.NewPoint(bounds.DynamicWorld, 30, 540), assembly.PriorityNearest)
	if !ok || !slices.Equal(limited, []bounds.InputField{bounds.InputSaddleRailOffset}) {
		t.Fatalf("edge: ok=%v limited=%v, want [RailOffset]", ok, limited)
	}
	assertNear(t, "edge valA", va, 160, 1e-12)
	assertNear(t, "edge valB", vb, 15, 1e-12)

	// 3. Corner limited: (-30, 650) => valA = 100 (min insertion), valB = -15, limited = [Insertion, RailOffset].
	va, vb, limited, ok = assembly.SolvePlanar2DoF(sheet, pose.NewPoint(bounds.DynamicWorld, -30, 650), assembly.PriorityNearest)
	if !ok || !slices.Equal(limited, []bounds.InputField{bounds.InputSeatpostInsertion, bounds.InputSaddleRailOffset}) {
		t.Fatalf("corner: ok=%v limited=%v, want [Insertion, RailOffset]", ok, limited)
	}
	assertNear(t, "corner valA", va, 100, 1e-12)
	assertNear(t, "corner valB", vb, -15, 1e-12)

	// 4. Unbounded upper axis (+Inf): AxisB in [0, +Inf) reaches arbitrarily far along +X.
	unbounded := sheet
	unbounded.AxisB.Range = mustRange(t, 0, math.Inf(1))
	va, vb, limited, ok = assembly.SolvePlanar2DoF(unbounded, pose.NewPoint(bounds.DynamicWorld, 250, 550), assembly.PriorityNearest)
	if !ok || limited != nil {
		t.Fatalf("unbounded reachable: ok=%v limited=%v", ok, limited)
	}
	assertNear(t, "unbounded valA", va, 150, 1e-12)
	assertNear(t, "unbounded valB", vb, 250, 1e-12)

	// Unbounded axis still clamps at its finite lower bound (0) when target is negative X.
	va, vb, limited, ok = assembly.SolvePlanar2DoF(unbounded, pose.NewPoint(bounds.DynamicWorld, -10, 550), assembly.PriorityNearest)
	if !ok || !slices.Equal(limited, []bounds.InputField{bounds.InputSaddleRailOffset}) {
		t.Fatalf("unbounded lower bound: ok=%v limited=%v", ok, limited)
	}
	assertNear(t, "unbounded clamped valB", vb, 0, 1e-12)

	// 5. Parallel axes return ok = false.
	parallel := sheet
	parallel.AxisB.Heading = pose.NewHeading(bounds.DynamicWorld, 90)
	if _, _, _, ok := assembly.SolvePlanar2DoF(parallel, pose.NewPoint(bounds.DynamicWorld, 0, 550), assembly.PriorityNearest); ok {
		t.Error("parallel axes: want ok=false")
	}
}

func TestSolvePlanar2DoFBRPGateClipping(t *testing.T) {
	// Tilted seat axis (105° = 75° STA) + horizontal rail axis (180°, range [-15, 15]).
	// Origin at (0, 500), insertion [100, 200] (Probe 200, Sign -1).
	// At max rearward rail (+15 mm => -15 mm X), pulling the post out along 105°
	// moves rearward (-cos 75° = -0.2588 per mm) and upward (+sin 75° = +0.9659 per mm).
	// Pick a target just past the rearward rail bound at Y = 550:
	//   Exact Y = 550 on the +15 rail edge lies at X = -15 - 50*cot(75°) = -28.397 mm.
	//   Set target at (-39.0, 550.0): exact Y misses X by 10.6 mm (outside ±10 mm BRP X gate!),
	//   while raising Y by +2 mm (inside ±2 mm BRP Y gate) moves X further rearward by -0.536 mm
	//   to -28.933 mm -> |ΔX| = 10.067 (still outside); pick target (-38.8, 550.0):
	//   - At exact Y=550: |ΔY| = 0, |ΔX| = 10.403 mm (fails X gate!)
	//   - Raising Y to 551.8: |ΔY| = 1.8 <= 2.0, X = -28.880 -> |ΔX| = 9.920 <= 10.0 (PASSES BRP gate!)
	sheet := assembly.Planar2DoF{
		Origin: pose.NewPoint(bounds.DynamicWorld, 0, 500),
		AxisA: assembly.LinearDoF{
			Field:   bounds.InputSeatpostInsertion,
			Heading: pose.NewHeading(bounds.DynamicWorld, 105),
			Range:   mustRange(t, 100, 200),
			Probe:   200,
			Sign:    -1,
		},
		AxisB: assembly.LinearDoF{
			Field:   bounds.InputSaddleRailOffset,
			Heading: pose.NewHeading(bounds.DynamicWorld, 180),
			Range:   mustRange(t, -15, 15),
			Probe:   0,
			Sign:    +1,
		},
	}

	target := pose.NewPoint(bounds.DynamicWorld, -38.8, 550.0)
	ins, rail, limited, ok := assembly.SolvePlanar2DoF(sheet, target, assembly.PriorityBRPGate)
	if !ok || !slices.Equal(limited, []bounds.InputField{bounds.InputSaddleRailOffset}) {
		t.Fatalf("BRP gate clip: ok=%v limited=%v", ok, limited)
	}
	assertNear(t, "rail clamped at +15", rail, 15.0, 1e-12)
	// Evaluate placed point from solved (ins, rail).
	ext := 200 - ins
	placed := sheet.Origin.
		Add(pose.Along(sheet.AxisA.Heading, ext)).
		Add(pose.Along(sheet.AxisB.Heading, rail))
	miss := target.Sub(placed)
	if math.Abs(miss.Y()) > assembly.FitGateBrpYMm+1e-9 || math.Abs(miss.X()) > assembly.FitGateBrpXMm+1e-9 {
		t.Fatalf("expected PriorityBRPGate to clip into gate, got miss = (%.4f, %.4f)", miss.X(), miss.Y())
	}
	// Verify it traded the minimum Y needed to bring |ΔX| just to 10.0 mm.
	assertNear(t, "clipped |ΔX|", math.Abs(miss.X()), assembly.FitGateBrpXMm, 1e-6)
}

func TestSolvePlanar2DoFCoordinateSystemPanics(t *testing.T) {
	sheet := assembly.Planar2DoF{
		Origin: pose.NewPoint(bounds.DynamicWorld, 0, 500),
		AxisA: assembly.LinearDoF{
			Field:   bounds.InputSeatpostInsertion,
			Heading: pose.NewHeading(bounds.DynamicWorld, 90),
			Range:   mustRange(t, 100, 200),
			Probe:   200,
			Sign:    -1,
		},
		AxisB: assembly.LinearDoF{
			Field:   bounds.InputSaddleRailOffset,
			Heading: pose.NewHeading(bounds.DynamicWorld, 0),
			Range:   mustRange(t, -15, 15),
			Probe:   0,
			Sign:    +1,
		},
	}
	assertPanics(t, "target in StaticWorld vs DynamicWorld sheet", func() {
		_, _, _, _ = assembly.SolvePlanar2DoF(sheet, pose.NewPoint(bounds.StaticWorld, 0, 550), assembly.PriorityNearest)
	})
	badAxis := sheet
	badAxis.AxisA.Heading = pose.NewHeading(assembly.FrameLocal, 90)
	assertPanics(t, "AxisA in FrameLocal", func() {
		_, _, _, _ = assembly.SolvePlanar2DoF(badAxis, pose.NewPoint(bounds.DynamicWorld, 0, 550), assembly.PriorityNearest)
	})
	badSign := sheet
	badSign.AxisA.Sign = 0
	assertPanics(t, "invalid Sign 0", func() {
		_, _, _, _ = assembly.SolvePlanar2DoF(badSign, pose.NewPoint(bounds.DynamicWorld, 0, 550), assembly.PriorityNearest)
	})
}

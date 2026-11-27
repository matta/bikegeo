package assembly

import (
	"fmt"
	"math"

	"github.com/matta/bikegeo/internal/pose"
)

const (
	fitBoundTolMm   = 1e-9
	fitDirectionTol = 1e-12 // treat round-off in a horizontal or vertical direction as zero
)

// FitPriority selects the boundary optimization objective when a 2-DoF target
// lies outside the feasible parameter box.
type FitPriority int

const (
	// PriorityNearest minimizes Euclidean miss over the feasible boundary.
	PriorityNearest FitPriority = iota
	// PriorityBRPGate prefers any feasible point inside the BRP fit gate
	// (|ΔY| <= FitGateBrpYMm and |ΔX| <= FitGateBrpXMm) first, then minimizes
	// (|ΔY|, |ΔX|) lexicographically.
	PriorityBRPGate
)

// LinearDoF describes one bounded scalar input whose landmark moves linearly
// along Heading in the target coordinate system:
//   - Field: input identifier reported in limited when clamped to a bound.
//   - Heading: unit-step movement direction of the landmark in the coordinate system.
//   - Range: legal parameter interval [Min, Max] in millimeters.
//   - Probe: parameter value corresponding to Planar2DoF.Origin.
//   - Sign: +1 if increasing the parameter advances the landmark along Heading,
//     −1 if increasing the parameter moves opposite Heading.
type LinearDoF struct {
	Field   InputField
	Heading pose.Heading
	Range   BoundedRange
	Probe   float64
	Sign    float64
}

// Planar2DoF represents a 2D landmark surface parameterized affinely by two
// bounded scalar degrees of freedom (AxisA, AxisB) anchored at Origin (the
// landmark position when AxisA == AxisA.Probe and AxisB == AxisB.Probe).
type Planar2DoF struct {
	Origin pose.Point
	AxisA  LinearDoF
	AxisB  LinearDoF
}

func (d LinearDoF) toAdjustment() fitAdjustment {
	if !d.Range.Valid() {
		panic(fmt.Sprintf("assembly: LinearDoF %q has uninitialized BoundedRange", d.Field))
	}
	if d.Sign != 1 && d.Sign != -1 {
		panic(fmt.Sprintf("assembly: LinearDoF %q requires Sign +1 or -1, got %v", d.Field, d.Sign))
	}
	if !isFinite(d.Probe) {
		panic(fmt.Sprintf("assembly: LinearDoF %q requires finite Probe, got %v", d.Field, d.Probe))
	}
	var lo, hi float64
	if d.Sign > 0 {
		lo = d.Range.Min() - d.Probe
		hi = d.Range.Max() - d.Probe
	} else {
		lo = d.Probe - d.Range.Max()
		hi = d.Probe - d.Range.Min()
	}
	return fitAdjustment{
		heading: d.Heading,
		lo:      lo, hi: hi,
		field: d.Field,
	}
}

func (d LinearDoF) paramFromTravel(travel float64) float64 {
	return d.Range.Clamp(d.Probe + d.Sign*travel)
}

// SolvePlanar2DoF solves for parameter values (valA, valB) inside sheet's
// bounded box so the affine landmark reaches target (or optimizes the boundary
// miss according to priority). For a reachable target, limited is nil
// (including at a bound within round-off). Otherwise limited lists the limiting
// inputs in (AxisA, AxisB) order. Parallel axes return ok = false.
// Coordinate-system mismatches between Origin, target, and axis headings panic
// deterministically.
func SolvePlanar2DoF(sheet Planar2DoF, target pose.Point, priority FitPriority) (valA, valB float64, limited []InputField, ok bool) {
	switch priority {
	case PriorityNearest, PriorityBRPGate:
	default:
		panic(fmt.Sprintf("assembly: invalid FitPriority %d", priority))
	}
	delta := target.Sub(sheet.Origin)
	adjA := sheet.AxisA.toAdjustment()
	adjB := sheet.AxisB.toAdjustment()
	travelA, travelB, limited, ok := fitBox(delta, adjA, adjB, priority)
	if !ok {
		return 0, 0, nil, false
	}
	return sheet.AxisA.paramFromTravel(travelA), sheet.AxisB.paramFromTravel(travelB), limited, true
}

// fitInterval is a closed range of signed travel values in millimeters.
type fitInterval struct{ lo, hi float64 }

func (r fitInterval) clamp(value float64) float64 { return min(max(value, r.lo), r.hi) }

func (r fitInterval) contains(value float64) bool {
	return value >= r.lo-fitBoundTolMm && value <= r.hi+fitBoundTolMm
}

// clip clips the interval to the portion where the line with intercept c and
// slope -s lies between -limit and +limit.
func (r fitInterval) clip(c, s, limit float64) (fitInterval, bool) {
	if math.Abs(s) < fitDirectionTol {
		if math.Abs(c) <= limit+fitBoundTolMm {
			return r, true
		}
		return fitInterval{}, false
	}
	left, right := (c-limit)/s, (c+limit)/s
	lo, hi := max(r.lo, min(left, right)), min(r.hi, max(left, right))
	if lo > hi+fitBoundTolMm {
		return fitInterval{}, false
	}
	if lo > hi {
		touch := r.clamp((lo + hi) / 2)
		return fitInterval{touch, touch}, true
	}
	return fitInterval{lo, hi}, true
}

// heightFirst returns the multiplier in r (in mm) that brings travel*direction
// closest to target in Y, breaking ties by distance in X. direction is a
// unit-length direction vector in target's coordinate system.
func (r fitInterval) heightFirst(target, direction pose.Vector) float64 {
	if math.Abs(direction.Y()) >= fitDirectionTol {
		return r.clamp(target.Y() / direction.Y())
	}
	if math.Abs(direction.X()) >= fitDirectionTol {
		return r.clamp(target.X() / direction.X())
	}
	return r.lo
}

// fitAdjustment is one fitting input in travel coordinates relative to the probe
// landmark.
type fitAdjustment struct {
	heading pose.Heading
	fitInterval
	field InputField
}

// fitEdge holds one input at a finite bound and lets the other input vary.
type fitEdge struct {
	fixed, moving fitAdjustment
	bound         float64
}

func (edge fitEdge) candidates(delta pose.Vector, priority FitPriority) []float64 {
	switch priority {
	case PriorityNearest:
		target := delta.Sub(pose.Along(edge.fixed.heading, edge.bound))
		return []float64{edge.moving.clamp(pose.ProjectOnto(target, edge.moving.heading))}
	case PriorityBRPGate:
		fixedDir := pose.Along(edge.fixed.heading, 1) // unit vector along fixed direction
		target := delta.Sub(fixedDir.Scale(edge.bound))
		dir := pose.Along(edge.moving.heading, 1) // unit vector along moving direction
		travel := edge.moving.heightFirst(target, dir)
		gate, ok := edge.moving.clip(target.Y(), dir.Y(), FitGateBrpYMm)
		if ok {
			if gate, ok = gate.clip(target.X(), dir.X(), FitGateBrpXMm); ok {
				return []float64{travel, gate.heightFirst(target, dir)}
			}
		}
		return []float64{travel}
	default:
		panic(fmt.Sprintf("assembly: invalid FitPriority %d", priority))
	}
}

func (p FitPriority) miss(delta pose.Vector, adjustments [2]fitAdjustment, travel [2]float64) pose.Vector {
	switch p {
	case PriorityNearest:
		return delta.Sub(pose.Along(adjustments[0].heading, travel[0])).Sub(pose.Along(adjustments[1].heading, travel[1]))
	case PriorityBRPGate:
		dirA, dirB := pose.Along(adjustments[0].heading, 1), pose.Along(adjustments[1].heading, 1)
		return delta.Sub(dirA.Scale(travel[0]).Add(dirB.Scale(travel[1])))
	default:
		panic(fmt.Sprintf("assembly: invalid FitPriority %d", p))
	}
}

func (p FitPriority) prefers(miss, best pose.Vector) bool {
	switch p {
	case PriorityNearest:
		return miss.Norm() < best.Norm()-fitBoundTolMm
	case PriorityBRPGate:
		x, y := math.Abs(miss.X()), math.Abs(miss.Y())
		bestX, bestY := math.Abs(best.X()), math.Abs(best.Y())
		inGate := func(x, y float64) bool {
			return y <= FitGateBrpYMm+fitBoundTolMm && x <= FitGateBrpXMm+fitBoundTolMm
		}
		if pass := inGate(x, y); pass != inGate(bestX, bestY) {
			return pass
		}
		if math.Abs(y-bestY) > fitBoundTolMm {
			return y < bestY
		}
		return x < bestX-fitBoundTolMm
	default:
		panic(fmt.Sprintf("assembly: invalid FitPriority %d", p))
	}
}

// fitBox returns signed travel (in mm) along directions a and b to reach delta.
func fitBox(delta pose.Vector, a, b fitAdjustment, priority FitPriority) (da, db float64, limited []InputField, ok bool) {
	da, db, ok = pose.DecomposeAlong(delta, a.heading, b.heading)
	if !ok {
		return 0, 0, nil, false
	}
	if a.contains(da) && b.contains(db) {
		return a.clamp(da), b.clamp(db), nil, true
	}

	adjustments := [2]fitAdjustment{a, b}
	var best [2]float64
	var bestMiss pose.Vector
	haveBest := false
	for fixed, adj := range adjustments {
		moving := 1 - fixed
		for _, bound := range [...]float64{adj.lo, adj.hi} {
			if math.IsInf(bound, 0) {
				continue
			}
			edge := fitEdge{fixed: adj, moving: adjustments[moving], bound: bound}
			for _, travel := range edge.candidates(delta, priority) {
				var setting [2]float64
				setting[fixed], setting[moving] = bound, travel
				miss := priority.miss(delta, adjustments, setting)
				if !haveBest || priority.prefers(miss, bestMiss) {
					best, bestMiss, haveBest = setting, miss, true
				}
			}
		}
	}

	boundTol := 0.0
	if priority == PriorityBRPGate {
		boundTol = fitBoundTolMm
	}
	for i, adj := range adjustments {
		if math.Abs(best[i]-adj.lo) <= boundTol || math.Abs(best[i]-adj.hi) <= boundTol {
			limited = append(limited, adj.field)
		}
	}
	return best[0], best[1], limited, true
}

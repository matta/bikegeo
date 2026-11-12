// Package curve is the single home of every score shape used by bikegeo.
//
// The shapes here are built so that a jump or a flat stretch cannot be expressed:
// two different inputs on the same side of a target always produce two different
// scores, and the worse input always produces the lower one. Scoring code must
// obtain its shapes from this package and must never clamp, round, or add
// stepped bonuses on top of them.
package curve

import (
	"fmt"
	"math"
)

// Peak is the score at distance zero from the target.
const Peak = 100.0

// Knot is a point the falloff passes through: Score at distance D from the target.
type Knot struct {
	D     float64
	Score float64
}

// Falloff maps a distance d >= 0 from a target to a score.
//
// Score(0) is Peak. The curve runs in straight segments through each knot, then
// continues from the last knot as a hyperbolic tail that crosses zero at zeroAt
// and approaches the floor without ever reaching it. It is continuous and
// strictly decreasing over its whole domain.
//
// The zero value is not a usable curve; build one with NewFalloff.
type Falloff struct {
	built bool
	knots []Knot  // strictly increasing D, strictly decreasing Score, all in (0, Peak)
	tailD float64 // distance where the tail starts (last knot, or 0)
	tailS float64 // score where the tail starts (last knot, or Peak)
	h     float64 // tail half-distance scale
	floor float64 // asymptote
}

// NewFalloff builds a Falloff. It returns an error for any input that could
// produce a flat, rising, or discontinuous curve:
//   - knot distances must be positive and strictly increasing;
//   - knot scores must be strictly decreasing, below Peak and above zero;
//   - zeroAt must lie strictly beyond the last knot (or beyond zero with no knots);
//   - floor must be negative.
func NewFalloff(knots []Knot, zeroAt, floor float64) (Falloff, error) {
	if !isFinite(zeroAt) || !isFinite(floor) {
		return Falloff{}, fmt.Errorf("curve: zeroAt (%v) and floor (%v) must be finite", zeroAt, floor)
	}
	if floor >= 0 {
		return Falloff{}, fmt.Errorf("curve: floor (%v) must be negative", floor)
	}

	prevD, prevS := 0.0, Peak
	for i, k := range knots {
		if !isFinite(k.D) || !isFinite(k.Score) {
			return Falloff{}, fmt.Errorf("curve: knot %d (%v, %v) must be finite", i, k.D, k.Score)
		}
		if k.D <= prevD {
			return Falloff{}, fmt.Errorf("curve: knot %d distance (%v) must be greater than %v", i, k.D, prevD)
		}
		if k.Score >= prevS {
			return Falloff{}, fmt.Errorf("curve: knot %d score (%v) must be less than %v", i, k.Score, prevS)
		}
		if k.Score <= 0 {
			return Falloff{}, fmt.Errorf("curve: knot %d score (%v) must be positive", i, k.Score)
		}
		prevD, prevS = k.D, k.Score
	}
	if zeroAt <= prevD {
		return Falloff{}, fmt.Errorf("curve: zeroAt (%v) must be greater than %v", zeroAt, prevD)
	}

	owned := make([]Knot, len(knots))
	copy(owned, knots)

	// Tail: s(x) = floor + (tailS - floor) * h / (h + x), x = d - tailD.
	// Choosing h so that s(zeroAt - tailD) = 0.
	return Falloff{
		built: true,
		knots: owned,
		tailD: prevD,
		tailS: prevS,
		h:     (zeroAt - prevD) * (-floor) / prevS,
		floor: floor,
	}, nil
}

// MustFalloff is NewFalloff for package-level curves with constant inputs.
// It panics if the inputs are invalid.
func MustFalloff(knots []Knot, zeroAt, floor float64) Falloff {
	f, err := NewFalloff(knots, zeroAt, floor)
	if err != nil {
		panic(err)
	}
	return f
}

// Score returns the score at distance d from the target. Negative distances are
// treated as zero. It panics on a Falloff that was not built by NewFalloff.
func (f Falloff) Score(d float64) float64 {
	if !f.built {
		panic("curve: Score called on a zero-value Falloff; use NewFalloff")
	}
	if d <= 0 {
		return Peak
	}

	prevD, prevS := 0.0, Peak
	for _, k := range f.knots {
		if d <= k.D {
			t := (d - prevD) / (k.D - prevD)
			return prevS + (k.Score-prevS)*t
		}
		prevD, prevS = k.D, k.Score
	}

	x := d - f.tailD
	return f.floor + (f.tailS-f.floor)*f.h/(f.h+x)
}

// Floor returns the asymptote the curve approaches but never reaches.
func (f Falloff) Floor() float64 {
	return f.floor
}

// Saturating rises continuously and strictly from 0 (at x = 0) toward max as x
// grows, reaching half of max at x = half. Non-positive x returns 0.
func Saturating(max, half, x float64) float64 {
	if x <= 0 || half <= 0 {
		return 0
	}
	return max * x / (x + half)
}

// MaxStemPenalty is the penalty for a stem exactly at either end of the
// expected stem length range.
const MaxStemPenalty = 15.0

// StemPenalty returns the penalty (in points) and the normalized deviation u for
// a stem length relative to the expected range [minLen, maxLen].
//
// At the midpoint u = 0 and the penalty is 0. At either boundary u = 1 and the
// penalty is MaxStemPenalty. Beyond the boundaries it keeps growing, more
// steeply. The penalty is continuous and strictly increasing in u.
func StemPenalty(stemLen, minLen, maxLen float64) (penalty float64, u float64) {
	if minLen <= 0 || maxLen <= minLen {
		return 0, 0
	}
	mid := (minLen + maxLen) / 2.0
	halfSpan := (maxLen - minLen) / 2.0

	u = math.Abs(stemLen-mid) / halfSpan
	if u <= 1.0 {
		return MaxStemPenalty * u * u, u
	}
	over := u - 1.0
	return MaxStemPenalty * (1.0 + 2.0*over*over + over), u
}

func isFinite(v float64) bool {
	return !math.IsNaN(v) && !math.IsInf(v, 0)
}

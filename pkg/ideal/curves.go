package ideal

import (
	"fmt"

	"bikegeo/pkg/curve"
	"bikegeo/pkg/model"
)

// sideCurve is the compiled falloff for one side of an attribute's ideal range.
// It is a function of the distance outside the ideal range on that side.
type sideCurve struct {
	// acceptableDist is how far past the ideal bound the acceptable bound lies
	// (0 when acceptable and ideal coincide on this side).
	acceptableDist float64
	falloff        curve.Falloff
}

// specCurves holds the compiled curves of an AttributeSpec. A side is nil when
// the ideal range is unbounded on that side.
type specCurves struct {
	below, above *sideCurve
}

// compileCurves turns the declarative AttributeSpec fields into curves from
// package curve. All validation of curve shape (no flat stretches, no jumps,
// strictly decreasing, floor never reached) is done by curve.NewFalloff.
func (s AttributeSpec) compileCurves() (specCurves, error) {
	var out specCurves
	if s.Ideal == nil {
		return out, nil
	}

	floor := s.Floor.Or(-200.0)

	build := func(side string, acceptableDist float64, fall model.Opt[float64]) (*sideCurve, error) {
		fVal, ok := fall.Get()
		if !ok || fVal <= 0 {
			return nil, fmt.Errorf("attribute %q must define positive falloff.%s", s.ID, side)
		}
		var knots []curve.Knot
		if acceptableDist > 0 {
			edge, ok := s.EdgeScore.Get()
			if !ok {
				return nil, fmt.Errorf("attribute %q acceptable range is wider than ideal on the %s side, but edge_score is not defined", s.ID, side)
			}
			knots = append(knots, curve.Knot{D: acceptableDist, Score: edge})
		}
		f, err := curve.NewFalloff(knots, acceptableDist+fVal, floor)
		if err != nil {
			return nil, fmt.Errorf("attribute %q %s curve: %w", s.ID, side, err)
		}
		return &sideCurve{acceptableDist: acceptableDist, falloff: f}, nil
	}

	var falloffBelow, falloffAbove model.Opt[float64]
	if s.Falloff != nil {
		falloffBelow, falloffAbove = s.Falloff.Below, s.Falloff.Above
	}

	var err error
	if idealMin, ok := s.Ideal.Min.Get(); ok {
		dist := 0.0
		if s.Acceptable != nil {
			if accMin, ok := s.Acceptable.Min.Get(); ok && accMin < idealMin {
				dist = idealMin - accMin
			}
		}
		if out.below, err = build("below", dist, falloffBelow); err != nil {
			return specCurves{}, err
		}
	}
	if idealMax, ok := s.Ideal.Max.Get(); ok {
		dist := 0.0
		if s.Acceptable != nil {
			if accMax, ok := s.Acceptable.Max.Get(); ok && accMax > idealMax {
				dist = accMax - idealMax
			}
		}
		if out.above, err = build("above", dist, falloffAbove); err != nil {
			return specCurves{}, err
		}
	}
	return out, nil
}

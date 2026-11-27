// Package catalog holds the frames, forks, handlebars, stems, seatposts,
// saddle, and default fit targets as Go values. Every value is labelled
// published, catalog, measured, owner-reported, or illustrative.
package catalog

import (
	"github.com/matta/bikegeo/internal/bounds"
	"github.com/matta/bikegeo/internal/pose"
)

// WheelRadiusMm is the shared 29" × 2.4" wheel radius (legacy DefaultWheelRadius29Mm).
const WheelRadiusMm = 372.0 // illustrative: standard loaded 29" × 2.4" radius

// WheelLabel names the shared wheel.
const WheelLabel = `29" × 2.4"`

// DefaultBrpTarget returns the idmatch fit report's BRP (measured; docs/RIDER_FIT.md).
func DefaultBrpTarget() pose.Point {
	return pose.NewPoint(bounds.DynamicWorld, -219.0, 713.1)
}

// DefaultGripTarget returns the idmatch fit report's HandGrip.
func DefaultGripTarget() pose.Point {
	return pose.NewPoint(bounds.DynamicWorld, 481.0, 726.0)
}

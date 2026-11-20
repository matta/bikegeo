// Package catalog holds the reboot's frames, forks, handlebars, seatposts,
// saddle, and default fit targets as Go values. Every value is labelled
// published, catalog, measured, owner-reported, or illustrative.
package catalog

import "bikegeo/internal/reboot/pose"

// WheelRadiusMm is the shared 29" × 2.4" wheel radius (legacy DefaultWheelRadius29Mm).
const WheelRadiusMm = 372.0 // illustrative: standard loaded 29" × 2.4" radius

// WheelLabel names the shared wheel.
const WheelLabel = `29" × 2.4"`

// SuspensionSagPcts returns the sag steps a suspension fork is enumerated at (Q31, Q56).
func SuspensionSagPcts() []float64 { return []float64{15, 20, 25} }

// DefaultBrpTarget returns the idmatch fit report's BRP (measured; docs/RIDER_FIT.md, Q53).
func DefaultBrpTarget() pose.Point { return pose.Point{X: -219.0, Y: 713.1} }

// DefaultGripTarget returns the idmatch fit report's HandGrip (measured; Q53, Q64).
func DefaultGripTarget() pose.Point { return pose.Point{X: 481.0, Y: 726.0} }

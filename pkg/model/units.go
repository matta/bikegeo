package model

import (
	"math"

	"bikegeo/pkg/geom2d"
)

// Deg is an angle in degrees, stored exactly as published. It has no
// trigonometry of its own: convert with Angle first.
type Deg float64

// Angle converts d to a geom2d.Angle for computation.
func (d Deg) Angle() geom2d.Angle { return geom2d.FromDeg(float64(d)) }

// Abs returns the magnitude of d, for comparing an angular difference to a tolerance.
func (d Deg) Abs() Deg { return Deg(math.Abs(float64(d))) }

// DegOf converts a computed angle back to degrees for storage or display.
func DegOf(a geom2d.Angle) Deg { return Deg(a.Deg()) }

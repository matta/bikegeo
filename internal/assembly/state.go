package assembly

import (
	"fmt"

	"github.com/matta/bikegeo/internal/pose"
)

// LoadState selects the physical load attitude of a bicycle assembly:
// Static is the unloaded workshop attitude at 0% suspension sag, and Dynamic
// is the rider-seated riding attitude at seated sag.
type LoadState string

const (
	// Static is the unloaded workshop state (0% fork and seatpost sag).
	Static LoadState = "static"

	// Dynamic is the rider-seated state (seated fork and seatpost sag).
	Dynamic LoadState = "dynamic"
)

// Valid reports whether s is a recognized LoadState.
func (s LoadState) Valid() bool {
	switch s {
	case Static, Dynamic:
		return true
	default:
		return false
	}
}

// WorldSystem returns the ambient world CoordinateSystem corresponding to s.
// It panics if s is not a valid LoadState.
func (s LoadState) WorldSystem() pose.CoordinateSystem {
	switch s {
	case Static:
		return StaticWorld
	case Dynamic:
		return DynamicWorld
	default:
		panic(fmt.Sprintf("assembly: invalid LoadState %q", s))
	}
}

// LoadStateForWorld returns the LoadState corresponding to a leveled ambient
// world coordinate system (StaticWorld → Static, DynamicWorld → Dynamic). For
// any other coordinate system (including UnpitchedChart or an uninitialized
// value), it returns ("", false).
func LoadStateForWorld(system pose.CoordinateSystem) (LoadState, bool) {
	switch system {
	case StaticWorld:
		return Static, true
	case DynamicWorld:
		return Dynamic, true
	default:
		return "", false
	}
}

// Ambient world coordinate systems share the bottom bracket as their origin
// BB = (0, 0), with +X horizontally forward, +Y vertically up, and 1 unit = 1 mm.
var (
	// UnpitchedChart is the chart reference attitude (0° pitch, +X along reach, +Y along stack).
	UnpitchedChart = pose.NewCoordinateSystem("world:unpitched_chart")

	// StaticWorld is the level-ground workshop coordinate system at 0% sag.
	StaticWorld = pose.NewCoordinateSystem("world:static")

	// DynamicWorld is the level-ground riding coordinate system at seated sag.
	DynamicWorld = pose.NewCoordinateSystem("world:dynamic")
)

package assembly

import "github.com/matta/bikegeo/internal/pose"

// Ambient world coordinate frames share the bottom bracket as their origin
// BB = (0, 0), with +X horizontally forward, +Y vertically up, and 1 unit = 1 mm.
var (
	// UnpitchedChart is the chart reference attitude (0° pitch, +X along reach, +Y along stack).
	UnpitchedChart = pose.NewFrame("world:unpitched_chart")

	// StaticWorld is the level-ground workshop coordinate frame at 0% sag.
	StaticWorld = pose.NewFrame("world:static")

	// DynamicWorld is the level-ground riding coordinate frame at seated sag.
	DynamicWorld = pose.NewFrame("world:dynamic")
)

// Component local coordinate frames for steerer mating joints.
var (
	// SteererLocal has origin (0, 0) at HeadTubeTop and +X pointing up and rearward along the steerer.
	SteererLocal = pose.NewFrame("part:steerer")

	// LowerHeadsetLocal has origin (0, 0) at HeadTubeBottom and +X pointing down and forward along the steerer.
	LowerHeadsetLocal = pose.NewFrame("part:lower_headset")
)

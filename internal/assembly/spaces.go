package assembly

import "github.com/matta/bikegeo/internal/pose"

// Component local coordinate systems shared by frameset and steerer mating
// joints.
var (
	// FrameLocal has origin (0, 0) at BB, +X along chart reach, and +Y along chart stack.
	FrameLocal = pose.NewCoordinateSystem("part:frame")

	// SteererLocal has origin (0, 0) at HeadTubeTop and +X pointing up and rearward along the steerer.
	SteererLocal = pose.NewCoordinateSystem("part:steerer")

	// LowerHeadsetLocal has origin (0, 0) at HeadTubeBottom and +X pointing down and forward along the steerer.
	LowerHeadsetLocal = pose.NewCoordinateSystem("part:lower_headset")
)

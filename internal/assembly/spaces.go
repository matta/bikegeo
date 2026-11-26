package assembly

// Ambient coordinate spaces share the bottom bracket as their origin BB = (0, 0),
// with +X forward, +Y up, and 1 unit = 1 millimeter.

// UnpitchedChart is the chart reference attitude (BB = (0, 0)), with +X along reach and +Y along stack.
type UnpitchedChart struct{}

// StaticWorld is the level-ground workshop coordinate space at 0% sag (BB = (0, 0)), with +X forward and +Y up.
type StaticWorld struct{}

// DynamicWorld is the level-ground riding coordinate space at seated sag (BB = (0, 0)), with +X forward and +Y up.
type DynamicWorld struct{}

// Orientation marker types name which way a part's local +X points (local +Y
// is 90° CCW), with no position attached. Each pose.Pose[Dir, W] measures its
// local coordinates from its own Origin() in W.

// FrameDir has local +X along chart horizontal, rotated by the frame pitch, with local +Y 90° CCW (upward).
type FrameDir struct{}

// SeatTubeDir points up the seat tube and seatpost shaft, with local +Y 90° CCW (rearward/downward).
type SeatTubeDir struct{}

// SaddleRailDir points along the saddle rails toward the nose, with local +Y 90° CCW (upward).
type SaddleRailDir struct{}

// SteererUpDir points up and rearward along the steerer.
type SteererUpDir struct{}

// SteererDownDir points down and forward along the steerer.
type SteererDownDir struct{}

// ForkRakeDir points forward, perpendicular to the steerer in the rake direction.
type ForkRakeDir struct{}

// StemDir points along the stem extension from the steerer clamp toward the bar clamp.
type StemDir struct{}

// HandlebarDir describes the rolled side-view orientation, with local +X forward
// and local +Y 90° CCW (along rise and upsweep).
type HandlebarDir struct{}

// PlumbDownDir points straight down (-90° in world space).
type PlumbDownDir struct{}

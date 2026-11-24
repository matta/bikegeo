package assembly

// These coordinate spaces share the bottom bracket as their origin.

// UnpitchedChart is the chart reference attitude, with +X along reach and +Y along stack.
type UnpitchedChart struct{}

// StaticWorld is the level-ground workshop coordinate space at 0% sag, with +X forward and +Y up.
type StaticWorld struct{}

// DynamicWorld is the level-ground riding coordinate space at seated sag, with +X forward and +Y up.
type DynamicWorld struct{}

// FrameAxis has local +X along chart horizontal, rotated by the frame pitch.
type FrameAxis struct{}

// SeatTubeAxis points up the seat tube and seatpost shaft, with local +Y rearward.
type SeatTubeAxis struct{}

// SaddleRailAxis points along the saddle rails toward the nose, with local +Y upward.
type SaddleRailAxis struct{}

// SteererUpAxis points up and rearward along the steerer.
type SteererUpAxis struct{}

// SteererDownAxis points down and forward along the steerer.
type SteererDownAxis struct{}

// ForkRakeAxis points forward, perpendicular to the steerer in the rake direction.
type ForkRakeAxis struct{}

// StemAxis points along the stem extension from the steerer clamp toward the bar clamp.
type StemAxis struct{}

// HandlebarAxis describes the rolled side-view plane, with local +X forward
// and local +Y along rise and upsweep.
type HandlebarAxis struct{}

// PlumbDownAxis points straight down from an axle toward the ground.
type PlumbDownAxis struct{}

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

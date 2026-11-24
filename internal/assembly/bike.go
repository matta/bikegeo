package assembly

// BikeSetup specifies the complete single-bike physical build inputs for PlaceBike.
type BikeSetup struct {
	Frame     FrameSetup
	LowerCup  LowerCupSetup
	Fork      ForkSetup
	Wheels    WheelSetup
	Seatpost  SeatpostSetup
	Saddle    SaddleSetup
	Stem      StemSetup
	Handlebar HandlebarSetup
}

// RailOffsetLimitMm returns the symmetric saddle rail offset bound
// L = (RailLengthMm − ClampLengthMm)/2. It errors when the clamp is longer
// than the rails or the dimensions are not finite.
func (s BikeSetup) RailOffsetLimitMm() (float64, error) {
	return railOffsetLimitMm(s.Saddle.RailLengthMm, s.Seatpost.Spec.RailClampLengthMm)
}

// BikePlacement holds the leveled whole-bike assembly alongside the BikeSetup
// that produced it, so callers and renderers always read both landmarks and
// physical dimensions from one consistent build.
type BikePlacement struct {
	Setup     BikeSetup
	Frameset  FramesetPlacement[DynamicWorld]
	Seatpost  SeatpostPlacement
	Saddle    SaddlePlacement
	Stem      StemPlacement
	Handlebar HandlebarPlacement
}

// SeatAxisToPostTopMm returns the axial distance from BB along the seat tube
// axis to the top of the exposed seatpost shaft (ShaftTop).
func (b BikePlacement) SeatAxisToPostTopMm() float64 {
	return b.Frameset.SeatAxis.Origin().DistanceTo(b.Seatpost.ShaftTop.Origin())
}

// SaddleHeightMm returns the radial distance from BB to the saddle BRP (|BRP - BB|).
func (b BikePlacement) SaddleHeightMm() float64 {
	return b.Frameset.BB.Origin().DistanceTo(b.Saddle.BRP.Origin())
}

// BrpToGripDistMm returns the sagittal Euclidean distance from saddle BRP to HandGrip
// (the cockpit reach-distance reference anchor).
func (b BikePlacement) BrpToGripDistMm() float64 {
	return b.Saddle.BRP.Origin().DistanceTo(b.Handlebar.HandGrip.Origin())
}

// SitBonesToGripRiseMm returns the vertical rise of HandGrip above SitBones.
// Measured from SitBones rather than BRP so rider-fit cockpit elevation carries
// consistently across saddles through the sit-bone contact point.
func (b BikePlacement) SitBonesToGripRiseMm() float64 {
	return b.Saddle.SitBones.Origin().VerticalOffsetTo(b.Handlebar.HandGrip.Origin())
}

// PlaceBike constructs the leveled frameset, mounts the rear cockpit
// (Seatpost -> Saddle) on SeatTubeTop, and mounts the front cockpit
// (Stem -> Handlebar) on HeadTubeTop in a single Pose tree rooted at BB.
func PlaceBike(setup BikeSetup) (BikePlacement, error) {
	frameset, err := PlaceFrameset(setup.Frame, setup.LowerCup, setup.Fork, setup.Wheels)
	if err != nil {
		return BikePlacement{}, err
	}
	seatpost, err := PlaceSeatpost(frameset.SeatTubeTop, setup.Frame, setup.Seatpost)
	if err != nil {
		return BikePlacement{}, err
	}
	saddle, err := PlaceSaddle(seatpost.RailCradle, setup.Seatpost.Spec.RailClampLengthMm, setup.Saddle)
	if err != nil {
		return BikePlacement{}, err
	}
	stem, err := PlaceStem(frameset.HeadTubeTop, setup.Stem)
	if err != nil {
		return BikePlacement{}, err
	}
	handlebar, err := PlaceHandlebar(stem.BarClamp, setup.Handlebar)
	if err != nil {
		return BikePlacement{}, err
	}
	return BikePlacement{
		Setup:     setup,
		Frameset:  frameset,
		Seatpost:  seatpost,
		Saddle:    saddle,
		Stem:      stem,
		Handlebar: handlebar,
	}, nil
}

package assembly

import "github.com/matta/bikegeo/internal/pose"

// SaddleSetup specifies a saddle's geometry in SaddleLocal coordinates
// (anchored at RailCenter), world-relative rail pitch, and fore-aft clamp
// offset along the rails for the flat *Setup API:
//   - RailCenterToBrpMm: longitudinal offset along the rail line (+X toward nose) from RailCenter to BRP.
//   - RailCenterToSitBonesMm: longitudinal offset along the rail line (+X toward nose) from RailCenter to SitBones.
//   - RailToBrpHeightMm: perpendicular offset (rail +Y, 90° CCW) from the rail line up to BRP.
//   - SitBonesElevationMm: perpendicular offset (rail +Y) of SitBones above BRP, so
//     SitBones sits RailToBrpHeightMm + SitBonesElevationMm above the rail line.
//   - RailLengthMm: straight clamping span of the saddle rails.
//   - RailPitchDeg: saddle rail pitch relative to world horizontal (0° level; positive raises nose CCW).
//   - RailOffsetMm: clamp position along the rails; positive moves the saddle
//     body rearward relative to RailPivot (advancing RailCenter by -RailOffsetMm).
//     It must stay within ±(RailLengthMm − clamp length)/2 so the clamp stays on the rails.
type SaddleSetup struct {
	RailCenterToBrpMm      float64
	RailCenterToSitBonesMm float64
	RailToBrpHeightMm      float64
	SitBonesElevationMm    float64
	RailLengthMm           float64
	RailPitchDeg           pose.Deg
	RailOffsetMm           float64
}

// PlaceSaddle mounts a saddle at railPivot (in StaticWorld or DynamicWorld),
// computes the physical clamp angle for world-relative RailPitchDeg, and
// delegates to NewSaddle, MateSaddle, Clamp, and Place.
func PlaceSaddle(railPivot pose.Pose, clampLengthMm float64, setup SaddleSetup) (SaddlePlacement, error) {
	// Assert ambient world membership before scalar validation so passing a
	// local-space pose panics deterministically.
	railTurn := RailTurnForWorldPitch(railPivot.Heading(), setup.RailPitchDeg)
	if err := validatePose("RailPivot", railPivot); err != nil {
		return SaddlePlacement{}, err
	}
	saddle, err := NewSaddle(SaddleDimensions{
		RailCenterToBrpMm:      setup.RailCenterToBrpMm,
		RailCenterToSitBonesMm: setup.RailCenterToSitBonesMm,
		RailToBrpHeightMm:      setup.RailToBrpHeightMm,
		SitBonesElevationMm:    setup.SitBonesElevationMm,
		RailLengthMm:           setup.RailLengthMm,
	})
	if err != nil {
		return SaddlePlacement{}, err
	}
	cradleInWorld := railPivot.Anchor(SeatpostCradle)
	dof, err := MateSaddle(RailClampPort{
		CradleInParent: cradleInWorld,
		ClampLengthMm:  clampLengthMm,
	}, saddle)
	if err != nil {
		return SaddlePlacement{}, err
	}
	clamped, err := dof.Clamp(setup.RailOffsetMm, railTurn)
	if err != nil {
		return SaddlePlacement{}, err
	}
	placed := clamped.Place(cradleInWorld)
	if err := validatePlacementPoints("saddle", railPivot.Origin(), placed.RailCenter.Origin(), placed.BRP.Origin(), placed.SitBones.Origin()); err != nil {
		return SaddlePlacement{}, err
	}
	return placed, nil
}

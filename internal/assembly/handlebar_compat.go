package assembly

import "github.com/matta/bikegeo/internal/pose"

// HandlebarSetup specifies a handlebar's geometry, grip inset, and
// world-relative roll for the flat *Setup API:
//   - StockWidthMm: uncut handlebar width in mm.
//   - CurrentWidthMm: installed width in mm (0 means uncut, i.e., StockWidthMm;
//     must not exceed StockWidthMm when StockWidthMm > 0).
//   - BacksweepDeg: rearward sweep angle in the top view (outboard of BendStartMm;
//     positive sweeps rearward, in (-90°, 90°)).
//   - UpsweepDeg: upward sweep angle in the front view (outboard of BendStartMm;
//     positive sweeps upward, in (-90°, 90°)).
//   - StatedRiseMm: published rise in mm according to RiseMode.
//   - RiseMode: standard knuckle or bar-end center measurement convention (required).
//   - BendStartMm: lateral half-width from clamp center to the sweep/rise bend intersection.
//   - GripInsetMm: inward distance from the cut bar end to the hand-grip center.
//   - BarRollDeg: handlebar roll in the side view relative to leveled world horizontal (+X);
//     positive rolls the top of the bar forward (clockwise in the sagittal +X/+Y plane).
type HandlebarSetup struct {
	StockWidthMm   float64
	CurrentWidthMm float64
	BacksweepDeg   pose.Deg
	UpsweepDeg     pose.Deg
	StatedRiseMm   float64
	RiseMode       RiseMeasurementMode
	BendStartMm    float64
	GripInsetMm    float64
	BarRollDeg     pose.Deg
}

// PlaceHandlebar mounts a handlebar at barClamp (in StaticWorld or
// DynamicWorld), computes the physical clamp angle for world-relative
// -BarRollDeg, and delegates to NewHandlebar, MountHandlebar, and Place.
func PlaceHandlebar(barClamp pose.Pose, setup HandlebarSetup) (HandlebarPlacement, error) {
	// Assert ambient world membership before scalar validation so passing a
	// local-space pose panics deterministically.
	rollTurn := BarTurnForWorldRoll(barClamp.Heading(), setup.BarRollDeg)
	if err := validatePose("barClamp", barClamp); err != nil {
		return HandlebarPlacement{}, err
	}
	bar, err := NewHandlebar(HandlebarDimensions{
		StockWidthMm:   setup.StockWidthMm,
		CurrentWidthMm: setup.CurrentWidthMm,
		BacksweepDeg:   setup.BacksweepDeg,
		UpsweepDeg:     setup.UpsweepDeg,
		StatedRiseMm:   setup.StatedRiseMm,
		RiseMode:       setup.RiseMode,
		BendStartMm:    setup.BendStartMm,
		GripInsetMm:    setup.GripInsetMm,
	})
	if err != nil {
		return HandlebarPlacement{}, err
	}
	barClampInWorld := barClamp.Anchor(StemLocal)
	clamped, err := MountHandlebar(BarClampPort{ClampInParent: barClampInWorld}, bar, rollTurn)
	if err != nil {
		return HandlebarPlacement{}, err
	}
	placed := clamped.Place(barClampInWorld)
	if err := validatePlacementPoints("handlebar", barClamp.Origin(), placed.BarCenter.Origin(), placed.HandGrip.Origin()); err != nil {
		return HandlebarPlacement{}, err
	}
	return placed, nil
}

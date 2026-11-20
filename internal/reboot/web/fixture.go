package web

import (
	"fmt"
	"math"

	"bikegeo/internal/reboot/assembly"
	"bikegeo/internal/reboot/pose"
)

const cadForkOptionID = "cad"

type forkOption struct {
	ID              string
	Label           string
	Spec            assembly.ForkSpec
	DefaultSagPct   float64
	DefaultLowerCup assembly.LowerCupSetup
}

func makeForkOption(id, name string, spec assembly.ForkSpec, defaultSagPct float64, defaultLowerCup assembly.LowerCupSetup) forkOption {
	var kind string
	if spec.TravelMm == 0 {
		kind = "Rigid"
	} else {
		kind = fmt.Sprintf("%.0f mm travel", spec.TravelMm)
	}
	return forkOption{
		ID:              id,
		Label:           fmt.Sprintf("%s (%s, %.0f mm A2C / %.0f mm rake)", name, kind, spec.StaticA2cMm, spec.RakeMm),
		Spec:            spec,
		DefaultSagPct:   defaultSagPct,
		DefaultLowerCup: defaultLowerCup,
	}
}

// handlebarOption is one handlebar the workbench can install. Setup.BarRollDeg
// is always 0: the roll comes from the workbench state (Q21).
type handlebarOption struct {
	ID    string
	Name  string
	Setup assembly.HandlebarSetup
}

// seatpostOption is one seatpost the workbench can install. Setup.ExtensionMm
// is always 0: the extension comes from the workbench state (Q27).
type seatpostOption struct {
	ID    string
	Name  string
	Setup assembly.SeatpostSetup
}

// illustrative: the rider's grips (legacy WingGripInsetMm), the same on every bar
const gripInsetMm = 70.0

// legacyHandlebar builds an uncut handlebar with the rider's grips and no roll.
func legacyHandlebar(id, name string, widthMm, riseMm float64, mode assembly.RiseMeasurementMode, backsweep, upsweep pose.Deg, bendStartMm float64) handlebarOption {
	return handlebarOption{
		ID:   id,
		Name: name,
		Setup: assembly.HandlebarSetup{
			StockWidthMm:   widthMm,
			CurrentWidthMm: widthMm,
			StatedRiseMm:   riseMm,
			RiseMode:       mode,
			BacksweepDeg:   backsweep,
			UpsweepDeg:     upsweep,
			BendStartMm:    bendStartMm,
			GripInsetMm:    gripInsetMm,
			BarRollDeg:     0,
		},
	}
}

type bikeFixture struct {
	Name               string
	Chart              assembly.FrameChart
	Wheels             assembly.WheelSetup
	WheelLabel         string
	DefaultForkID      string
	Forks              []forkOption
	DefaultStem        assembly.StemSetup
	CockpitStock       assembly.CockpitStock
	CockpitPreferences assembly.CockpitPreferences
	DefaultHandlebarID string
	Handlebars         []handlebarOption
	DefaultSeatpostID  string
	DefaultPostExtMm   float64
	Seatposts          []seatpostOption
	SaddleName         string
	DefaultSaddle      assembly.SaddleSetup
	// ExampleBrpTarget is a placeholder for the BRP fit target inputs, never a default.
	ExampleBrpTarget pose.Point
	// ExampleGripTarget is a placeholder for the HandGrip fit target inputs, never a default.
	ExampleGripTarget pose.Point
}

func (f bikeFixture) cadForkOption() forkOption {
	return forkOption{
		ID: cadForkOptionID,
		Label: fmt.Sprintf(
			"CAD Baseline Fork (%.0f mm A2C / %.0f mm rake, %.0f mm cup)",
			f.Chart.CadFork.A2cMm, f.Chart.CadFork.RakeMm, f.Chart.CadLowerCup.StackMm,
		),
		Spec: assembly.ForkSpec{
			StaticA2cMm: f.Chart.CadFork.A2cMm,
			TravelMm:    0.0,
			RakeMm:      f.Chart.CadFork.RakeMm,
		},
		DefaultSagPct:   0.0,
		DefaultLowerCup: f.Chart.CadLowerCup,
	}
}

func (f bikeFixture) resolveForkOption(requestedID string) (forkOption, bool) {
	if requestedID == "" {
		requestedID = f.DefaultForkID
	}
	if requestedID == cadForkOptionID {
		return f.cadForkOption(), true
	}
	for _, opt := range f.Forks {
		if opt.ID == requestedID {
			return opt, true
		}
	}
	return forkOption{}, false
}

// handlebarSetups returns each handlebar option's setup, in list order.
func handlebarSetups(bars []handlebarOption) []assembly.HandlebarSetup {
	setups := make([]assembly.HandlebarSetup, len(bars))
	for i, bar := range bars {
		setups[i] = bar.Setup
	}
	return setups
}

// resolveHandlebar returns the handlebar with requestedID, or the default
// handlebar when requestedID is empty.
func (f bikeFixture) resolveHandlebar(requestedID string) (handlebarOption, bool) {
	if requestedID == "" {
		requestedID = f.DefaultHandlebarID
	}
	for _, opt := range f.Handlebars {
		if opt.ID == requestedID {
			return opt, true
		}
	}
	return handlebarOption{}, false
}

// resolveSeatpost returns the seatpost with requestedID, or the default
// seatpost when requestedID is empty.
func (f bikeFixture) resolveSeatpost(requestedID string) (seatpostOption, bool) {
	if requestedID == "" {
		requestedID = f.DefaultSeatpostID
	}
	for _, opt := range f.Seatposts {
		if opt.ID == requestedID {
			return opt, true
		}
	}
	return seatpostOption{}, false
}

func (f bikeFixture) validate() error {
	if st := f.DefaultStem; !(!math.IsNaN(st.MinLengthMm) && !math.IsInf(st.MinLengthMm, 0) &&
		!math.IsNaN(st.MaxLengthMm) && !math.IsInf(st.MaxLengthMm, 0) &&
		st.MinLengthMm > 0 && st.MinLengthMm <= st.MaxLengthMm) {
		return fmt.Errorf("fixture %q default stem length range must be finite with 0 < min <= max, got [%v, %v]", f.Name, st.MinLengthMm, st.MaxLengthMm)
	}
	if err := f.CockpitStock.Validate(); err != nil {
		return fmt.Errorf("fixture %q cockpit stock invalid: %w", f.Name, err)
	}
	if err := f.CockpitPreferences.Validate(); err != nil {
		return fmt.Errorf("fixture %q cockpit preferences invalid: %w", f.Name, err)
	}
	if f.DefaultForkID == "" {
		return fmt.Errorf("fixture %q missing DefaultForkID", f.Name)
	}
	if f.DefaultForkID == cadForkOptionID {
		return fmt.Errorf("fixture %q DefaultForkID cannot be reserved ID %q", f.Name, cadForkOptionID)
	}
	defaultFound := false
	for _, opt := range f.Forks {
		if opt.ID == "" {
			return fmt.Errorf("fixture %q has fork option with empty ID", f.Name)
		}
		if opt.ID == cadForkOptionID {
			return fmt.Errorf("fixture %q has fork option using reserved ID %q", f.Name, cadForkOptionID)
		}
		if err := (assembly.ForkSetup{Spec: opt.Spec, SagPct: opt.DefaultSagPct}).Validate(); err != nil {
			return fmt.Errorf("fixture %q fork option %q invalid: %w", f.Name, opt.ID, err)
		}
		if err := opt.DefaultLowerCup.Validate(); err != nil {
			return fmt.Errorf("fixture %q fork option %q lower cup invalid: %w", f.Name, opt.ID, err)
		}
		if opt.DefaultLowerCup.Standard == assembly.LowerCupBareHeadTube {
			return fmt.Errorf("fixture %q installable fork option %q cannot use %q lower cup", f.Name, opt.ID, assembly.LowerCupBareHeadTube)
		}
		if opt.ID == f.DefaultForkID {
			defaultFound = true
		}
	}
	if !defaultFound {
		return fmt.Errorf("fixture %q DefaultForkID %q not found in Forks", f.Name, f.DefaultForkID)
	}
	if err := (assembly.ForkSetup{Spec: f.cadForkOption().Spec, SagPct: 0.0}).Validate(); err != nil {
		return fmt.Errorf("fixture %q Chart.CadFork invalid: %w", f.Name, err)
	}
	if err := f.Chart.CadLowerCup.Validate(); err != nil {
		return fmt.Errorf("fixture %q Chart.CadLowerCup invalid: %w", f.Name, err)
	}
	if f.DefaultHandlebarID == "" {
		return fmt.Errorf("fixture %q missing DefaultHandlebarID", f.Name)
	}
	if _, ok := f.resolveHandlebar(f.DefaultHandlebarID); !ok {
		return fmt.Errorf("fixture %q DefaultHandlebarID %q not found in Handlebars", f.Name, f.DefaultHandlebarID)
	}
	seenBars := make(map[string]bool, len(f.Handlebars))
	for _, opt := range f.Handlebars {
		if opt.ID == "" {
			return fmt.Errorf("fixture %q has handlebar option with empty ID", f.Name)
		}
		if seenBars[opt.ID] {
			return fmt.Errorf("fixture %q has duplicate handlebar ID %q", f.Name, opt.ID)
		}
		seenBars[opt.ID] = true
		if opt.Setup.BarRollDeg != 0 {
			return fmt.Errorf("fixture %q handlebar %q must not set BarRollDeg", f.Name, opt.ID)
		}
		if _, err := assembly.PlaceHandlebar(pose.NewPose(pose.Point{}, 0, nil), opt.Setup); err != nil {
			return fmt.Errorf("fixture %q handlebar %q invalid: %w", f.Name, opt.ID, err)
		}
	}
	if f.DefaultSeatpostID == "" {
		return fmt.Errorf("fixture %q missing DefaultSeatpostID", f.Name)
	}
	defaultPost, ok := f.resolveSeatpost(f.DefaultSeatpostID)
	if !ok {
		return fmt.Errorf("fixture %q DefaultSeatpostID %q not found in Seatposts", f.Name, f.DefaultSeatpostID)
	}
	seenPosts := make(map[string]bool, len(f.Seatposts))
	for _, opt := range f.Seatposts {
		if opt.ID == "" {
			return fmt.Errorf("fixture %q has seatpost option with empty ID", f.Name)
		}
		if seenPosts[opt.ID] {
			return fmt.Errorf("fixture %q has duplicate seatpost ID %q", f.Name, opt.ID)
		}
		seenPosts[opt.ID] = true
		if opt.Setup.ExtensionMm != 0 {
			return fmt.Errorf("fixture %q seatpost %q must not set ExtensionMm", f.Name, opt.ID)
		}
		setup := opt.Setup
		setup.ExtensionMm = setup.MinExtensionMm
		if _, err := assembly.PlaceSeatpost(pose.NewPose(pose.Point{}, 0, nil), setup); err != nil {
			return fmt.Errorf("fixture %q seatpost %q invalid: %w", f.Name, opt.ID, err)
		}
	}
	defaultPostSetup := defaultPost.Setup
	defaultPostSetup.ExtensionMm = f.DefaultPostExtMm
	if _, err := assembly.PlaceSeatpost(pose.NewPose(pose.Point{}, 0, nil), defaultPostSetup); err != nil {
		return fmt.Errorf("fixture %q default seatpost %q at DefaultPostExtMm invalid: %w", f.Name, f.DefaultSeatpostID, err)
	}
	return nil
}

// otsoFenrirFixture returns the Milestone 1 Otso Fenrir Stainless (2024, Size L)
// fixture with its sourced published chart dimensions and selectable forks and handlebars.
func otsoFenrirFixture() bikeFixture {
	const (
		// Model the generic zero-offset post with the Ritchey Comp Trail Zero's
		// nominal length and minimum insertion. The workbench uses a simplified clamp.
		// https://ritcheylogic.com/bike/seatposts/comp-trail-zero-seatpost
		postLengthMm       = 400.0
		postMinInsertionMm = 100.0
		// Otso lists 290 mm of insertion depth for the Large Fenrir Stainless.
		// https://otsocycles.com/pages/bike-specs-tables
		frameInsertionDepthMm = 290.0
	)
	// illustrative: generic posts with the Ritchey-based extension limits above
	genericSeatpost := func(id string, setbackMm float64) seatpostOption {
		return seatpostOption{
			ID:   id,
			Name: fmt.Sprintf("Generic %.0f mm setback post", setbackMm),
			Setup: assembly.SeatpostSetup{
				MinExtensionMm: postLengthMm - frameInsertionDepthMm,
				MaxExtensionMm: postLengthMm - postMinInsertionMm,
				PostSetbackMm:  setbackMm,
				ClampLengthMm:  30.0, // measured
			},
		}
	}
	ec44Cup := assembly.LowerCupSetup{
		Standard: assembly.LowerCupEC44,
		StackMm:  12.0,
	}
	return bikeFixture{
		Name: "Otso Fenrir Stainless (2024, Size L)",
		Chart: assembly.FrameChart{
			Frame: assembly.FrameSetup{
				ReachMm:          428.0,
				StackMm:          642.0,
				HeadTubeAngleDeg: 68.25,
				SeatTubeAngleDeg: 75.25,
				HeadTubeLengthMm: 140.0,
				BbDropMm:         72.0,
				ChainstayMm:      450.0,
				SeatTubeLengthMm: 490.0,
			},
			CadLowerCup: assembly.LowerCupSetup{
				Standard: assembly.LowerCupBareHeadTube,
				StackMm:  0.0,
			},
			PublishedWheelbaseMm: 1158.0,
			CadFork: assembly.ForkGeometry{
				A2cMm:  495.0,
				RakeMm: 54.0,
			},
		},
		Wheels:        assembly.WheelSetup{RadiusMm: 372.0},
		WheelLabel:    `29" × 2.4"`,
		DefaultForkID: "lithic_carbon_mountain",
		Forks: []forkOption{
			makeForkOption("lithic_carbon_mountain", "Lithic Carbon Mountain", assembly.ForkSpec{
				StaticA2cMm: 495.0,
				TravelMm:    0.0,
				RakeMm:      54.0,
			}, 0.0, ec44Cup),
			makeForkOption("sid_100", "RockShox SID", assembly.ForkSpec{
				StaticA2cMm: 506.0,
				TravelMm:    100.0,
				RakeMm:      44.0,
			}, 20.0, ec44Cup),
			makeForkOption("fox34_120", "Fox 34", assembly.ForkSpec{
				StaticA2cMm: 530.0,
				TravelMm:    120.0,
				RakeMm:      44.0,
			}, 20.0, ec44Cup),
		},
		DefaultStem: assembly.StemSetup{
			UpperCoverMm:  11.0,
			SpacersMm:     20.0,
			ClampHeightMm: 40.0,
			LengthMm:      60.0,
			AngleDeg:      -6.0,
			MaxSpacersMm:  40.0, // illustrative: steerer length not measured
			MinLengthMm:   40.0, // illustrative stem length range
			MaxLengthMm:   130.0,
		},
		CockpitStock: assembly.CockpitStock{ // illustrative stock stems and spacers
			StemLengthStepMm: 10,
			StemAnglesDeg:    []pose.Deg{-17, -6, 0, 6, 17},
			SpacerStepMm:     5,
		},
		CockpitPreferences: assembly.CockpitPreferences{
			GripOval:   assembly.FitOval{SemiMinorMm: 10, SemiMajorMm: 20, MajorAxisHeadingDeg: 60, CostPerR: 22}, // legacy idmatch oval (Q17)
			StemLength: assembly.PricedBand{FreeMin: 70, FreeMax: 80, CostBelow: 0.2, CostAbove: 0.2},             // illustrative
			StemAngle:  assembly.PricedBand{CostBelow: 0.2, CostAbove: 0.2},                                       // illustrative
			Spacers:    assembly.PricedBand{FreeMax: 20, CostAbove: 0.4},                                          // illustrative
			BarSweep:   assembly.PricedBand{FreeMin: 16, FreeMax: 20, CostBelow: 2, CostAbove: 2},                 // Matt's preference (Q30)
		},
		DefaultHandlebarID: "persuader20_800",
		// catalog: ported from legacy internal/catalog/handlebars.go; sources not recorded (Q20)
		Handlebars: []handlebarOption{
			legacyHandlebar("persuader20_800", "Tumbleweed Persuader 20° (800 mm)", 800, 50, assembly.RiseModeBarEndCenter, 20, 4, 135),
			legacyHandlebar("persuader20_840", "Tumbleweed Persuader 20° (840 mm)", 840, 50, assembly.RiseModeBarEndCenter, 20, 4, 135),
			// illustrative: legacy SQlabEffectiveBendStartMm = 55 + 0.9·rise
			legacyHandlebar("sqlab3ox_16_15", "SQlab 3OX 16°, 15 mm rise (780 mm)", 780, 15, assembly.RiseModeStandardKnuckle, 16, 4, 68.5),
			legacyHandlebar("sqlab3ox_16_30", "SQlab 3OX 16°, 30 mm rise (780 mm)", 780, 30, assembly.RiseModeStandardKnuckle, 16, 4, 82.0),
			legacyHandlebar("sqlab3ox_16_45", "SQlab 3OX 16°, 45 mm rise (780 mm)", 780, 45, assembly.RiseModeStandardKnuckle, 16, 4, 95.5),
			legacyHandlebar("sqlab3ox_12_15", "SQlab 3OX 12°, 15 mm rise (780 mm)", 780, 15, assembly.RiseModeStandardKnuckle, 12, 4, 68.5),
			legacyHandlebar("sqlab3ox_12_30", "SQlab 3OX 12°, 30 mm rise (780 mm)", 780, 30, assembly.RiseModeStandardKnuckle, 12, 4, 82.0),
			legacyHandlebar("sqlab3ox_12_45", "SQlab 3OX 12°, 45 mm rise (780 mm)", 780, 45, assembly.RiseModeStandardKnuckle, 12, 4, 95.5),
			legacyHandlebar("sqlab311_20_16_15", "SQlab 311 2.0 Alloy 16°, 15 mm rise (740 mm)", 740, 15, assembly.RiseModeStandardKnuckle, 16, 5, 68.5),
			legacyHandlebar("sqlab311_20_16_25", "SQlab 311 2.0 Alloy 16°, 25 mm rise (740 mm)", 740, 25, assembly.RiseModeStandardKnuckle, 16, 5, 77.5),
			legacyHandlebar("sqlab311_20_16_50", "SQlab 311 2.0 Alloy 16°, 50 mm rise (740 mm)", 740, 50, assembly.RiseModeStandardKnuckle, 16, 5, 100.0),
			legacyHandlebar("sqlab311_flx_16_15", "SQlab 311 FL-X Carbon 16°, 15 mm rise (740 mm)", 740, 15, assembly.RiseModeStandardKnuckle, 16, 4, 68.5),
			legacyHandlebar("sqlab311_flx_16_30", "SQlab 311 FL-X Carbon 16°, 30 mm rise (740 mm)", 740, 30, assembly.RiseModeStandardKnuckle, 16, 4, 82.0),
			legacyHandlebar("sqlab311_flx_12_15", "SQlab 311 FL-X Carbon 12°, 15 mm rise (740 mm)", 740, 15, assembly.RiseModeStandardKnuckle, 12, 4, 68.5),
			legacyHandlebar("sqlab311_flx_12_30", "SQlab 311 FL-X Carbon 12°, 30 mm rise (740 mm)", 740, 30, assembly.RiseModeStandardKnuckle, 12, 4, 82.0),
		},
		DefaultSeatpostID: "setback_0",
		DefaultPostExtMm:  263.5,
		Seatposts: []seatpostOption{
			genericSeatpost("setback_0", 0),
			genericSeatpost("setback_15", 15),
			genericSeatpost("setback_25", 25),
			// owner-reported 2026-11-19: 12 mm setback when sagged (Q28). The fit
			// report was transferred with this post unsagged (docs/RIDER_FIT.md).
			// illustrative: extension limits and clamp length not yet measured; generic values.
			{ID: "redshift_shockstop_pro", Name: "Redshift ShockStop PRO (12 mm setback, sagged)", Setup: assembly.SeatpostSetup{
				MinExtensionMm: postLengthMm - frameInsertionDepthMm,
				MaxExtensionMm: postLengthMm - postMinInsertionMm,
				PostSetbackMm:  12.0,
				ClampLengthMm:  30.0,
			}},
		},
		SaddleName: "SQlab 610 Ergolux Active 2.1",
		DefaultSaddle: assembly.SaddleSetup{
			RailCenterToBrpMm:      32.0,
			RailCenterToSitBonesMm: -48.0,
			SitBonesElevationMm:    10.0,
			RailLengthMm:           60.0, // measured
			RailPitchDeg:           0.0,
			RailOffsetMm:           0.0,
		},
		// measured: Matt's idmatch fit report, docs/RIDER_FIT.md
		ExampleBrpTarget: pose.Point{X: -219.0, Y: 713.1},
		// measured: idmatch fit report handlebar coordinates (docs/RIDER_FIT.md); whether the report point is HandGrip or the stem clamp is unresolved
		ExampleGripTarget: pose.Point{X: 481.0, Y: 726.0},
	}
}

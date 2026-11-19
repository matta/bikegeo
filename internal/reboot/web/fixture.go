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

type bikeFixture struct {
	Name             string
	Chart            assembly.FrameChart
	Wheels           assembly.WheelSetup
	WheelLabel       string
	DefaultForkID    string
	Forks            []forkOption
	DefaultStem      assembly.StemSetup
	HandlebarName    string
	DefaultHandlebar assembly.HandlebarSetup
	DefaultSeatpost  assembly.SeatpostSetup
	SaddleName       string
	DefaultSaddle    assembly.SaddleSetup
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

func (f bikeFixture) validate() error {
	if st := f.DefaultStem; !(!math.IsNaN(st.MinLengthMm) && !math.IsInf(st.MinLengthMm, 0) &&
		!math.IsNaN(st.MaxLengthMm) && !math.IsInf(st.MaxLengthMm, 0) &&
		st.MinLengthMm > 0 && st.MinLengthMm <= st.MaxLengthMm) {
		return fmt.Errorf("fixture %q default stem length range must be finite with 0 < min <= max, got [%v, %v]", f.Name, st.MinLengthMm, st.MaxLengthMm)
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
	return nil
}

// otsoFenrirFixture returns the Milestone 1 Otso Fenrir Stainless (2024, Size L)
// fixture with its sourced published chart dimensions and selectable forks.
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
		HandlebarName: "Tumbleweed Persuader 20° (800 mm)",
		DefaultHandlebar: assembly.HandlebarSetup{
			StockWidthMm:   800.0,
			CurrentWidthMm: 800.0,
			BacksweepDeg:   20.0,
			UpsweepDeg:     4.0,
			StatedRiseMm:   50.0,
			RiseMode:       assembly.RiseModeBarEndCenter,
			BendStartMm:    135.0,
			GripInsetMm:    70.0,
			BarRollDeg:     0.0,
		},
		DefaultSeatpost: assembly.SeatpostSetup{
			ExtensionMm:    263.5,
			MinExtensionMm: postLengthMm - frameInsertionDepthMm,
			MaxExtensionMm: postLengthMm - postMinInsertionMm,
			PostSetbackMm:  0.0,
			ClampLengthMm:  30.0, // measured
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

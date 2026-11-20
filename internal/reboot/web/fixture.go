package web

import (
	"fmt"
	"math"

	"bikegeo/internal/reboot/assembly"
	"bikegeo/internal/reboot/catalog"
	"bikegeo/internal/reboot/pose"
)

const cadForkOptionID = "cad"

type forkOption struct {
	ID              string
	Name            string
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
		Name:            name,
		Label:           fmt.Sprintf("%s (%s, %.0f mm A2C / %.0f mm rake)", name, kind, spec.StaticA2cMm, spec.RakeMm),
		Spec:            spec,
		DefaultSagPct:   defaultSagPct,
		DefaultLowerCup: defaultLowerCup,
	}
}

type bikeFixture struct {
	Name                   string
	Chart                  assembly.FrameChart
	Wheels                 assembly.WheelSetup
	WheelLabel             string
	DefaultForkID          string
	Forks                  []forkOption
	DefaultStem            assembly.StemSetup
	CockpitStock           assembly.CockpitStock
	CockpitPreferences     assembly.CockpitPreferences
	StockSagPcts           []float64
	DefaultHandlebarID     string
	Handlebars             []catalog.Handlebar
	DefaultSeatpostID      string
	DefaultPostInsertionMm float64
	Seatposts              []catalog.Seatpost
	SaddleName             string
	DefaultSaddle          assembly.SaddleSetup
	// ExampleBrpTarget is a placeholder for the BRP fit target inputs, never a default.
	ExampleBrpTarget pose.Point
	// ExampleGripTarget is a placeholder for the HandGrip fit target inputs, never a default.
	ExampleGripTarget pose.Point
}

func (f bikeFixture) cadForkOption() forkOption {
	return forkOption{
		ID:   cadForkOptionID,
		Name: "CAD Baseline Fork",
		Label: fmt.Sprintf(
			"CAD Baseline Fork (%.0f mm A2C / %.0f mm rake, %.0f mm cup)",
			f.Chart.CadFork.A2cMm, f.Chart.CadFork.RakeMm, f.Chart.CadLowerCup.StackMm,
		),
		Spec: assembly.ForkSpec{
			StaticA2cMm: f.Chart.CadFork.A2cMm,
			TravelMm:    0.0,
			RakeMm:      f.Chart.CadFork.RakeMm,
			Steerer:     assembly.SteererCAD, // sentinel: the CAD fork has no physical steerer
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

// stockForkOption is one fork configuration offered to stock fitting (Q31).
type stockForkOption struct {
	Option forkOption
	SagPct float64
}

// stockForks returns the fork configurations stock fitting offers, in fixture
// order: a rigid fork once at 0% sag, a suspension fork at each StockSagPcts.
func (f bikeFixture) stockForks() []stockForkOption {
	var offered []stockForkOption
	for _, opt := range f.Forks {
		if opt.Spec.TravelMm == 0 {
			offered = append(offered, stockForkOption{Option: opt})
			continue
		}
		for _, sag := range f.StockSagPcts {
			offered = append(offered, stockForkOption{Option: opt, SagPct: sag})
		}
	}
	return offered
}

// stockForkSetups returns each offered configuration as an assembly.StockFork,
// in the same order: the option's catalog Spec, the sag, and its DefaultLowerCup.
func stockForkSetups(offered []stockForkOption) []assembly.StockFork {
	setups := make([]assembly.StockFork, len(offered))
	for i, opt := range offered {
		setups[i] = assembly.StockFork{
			Fork:     assembly.ForkSetup{Spec: opt.Option.Spec, SagPct: opt.SagPct},
			LowerCup: opt.Option.DefaultLowerCup,
		}
	}
	return setups
}

// forkCellName names a fork configuration in a stock row: the fork name, plus
// ", N% sag" for a suspension fork.
func forkCellName(opt forkOption, sagPct float64) string {
	if opt.Spec.TravelMm == 0 {
		return opt.Name
	}
	return fmt.Sprintf("%s, %.0f%% sag", opt.Name, sagPct)
}

// handlebarSetups returns each handlebar option's setup, in list order.
func handlebarSetups(bars []catalog.Handlebar) []assembly.HandlebarSetup {
	setups := make([]assembly.HandlebarSetup, len(bars))
	for i, bar := range bars {
		setups[i] = bar.Setup
	}
	return setups
}

// resolveHandlebar returns the handlebar with requestedID, or the default
// handlebar when requestedID is empty.
func (f bikeFixture) resolveHandlebar(requestedID string) (catalog.Handlebar, bool) {
	if requestedID == "" {
		requestedID = f.DefaultHandlebarID
	}
	for _, opt := range f.Handlebars {
		if opt.ID == requestedID {
			return opt, true
		}
	}
	return catalog.Handlebar{}, false
}

// resolveSeatpost returns the seatpost with requestedID, or the default
// seatpost when requestedID is empty.
func (f bikeFixture) resolveSeatpost(requestedID string) (catalog.Seatpost, bool) {
	if requestedID == "" {
		requestedID = f.DefaultSeatpostID
	}
	for _, opt := range f.Seatposts {
		if opt.ID == requestedID {
			return opt, true
		}
	}
	return catalog.Seatpost{}, false
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
		if _, err := assembly.PlaceFrameset(f.Chart.Frame, opt.DefaultLowerCup, assembly.ForkSetup{Spec: opt.Spec, SagPct: opt.DefaultSagPct}, f.Wheels); err != nil {
			return fmt.Errorf("fixture %q fork option %q does not fit the frame: %w", f.Name, opt.ID, err)
		}
		if opt.ID == f.DefaultForkID {
			defaultFound = true
		}
	}
	if !defaultFound {
		return fmt.Errorf("fixture %q DefaultForkID %q not found in Forks", f.Name, f.DefaultForkID)
	}
	if len(f.StockSagPcts) == 0 {
		return fmt.Errorf("fixture %q missing StockSagPcts", f.Name)
	}
	for _, opt := range f.stockForks() {
		if err := (assembly.ForkSetup{Spec: opt.Option.Spec, SagPct: opt.SagPct}).Validate(); err != nil {
			return fmt.Errorf("fixture %q stock fork %q at %v%% sag invalid: %w", f.Name, opt.Option.ID, opt.SagPct, err)
		}
	}
	if err := (assembly.ForkSetup{Spec: f.cadForkOption().Spec, SagPct: 0.0}).Validate(); err != nil {
		return fmt.Errorf("fixture %q Chart.CadFork invalid: %w", f.Name, err)
	}
	if err := f.Chart.CadLowerCup.Validate(); err != nil {
		return fmt.Errorf("fixture %q Chart.CadLowerCup invalid: %w", f.Name, err)
	}
	if _, err := assembly.PlaceCadBaselineFork(f.Chart, f.Wheels); err != nil {
		return fmt.Errorf("fixture %q CAD baseline fork does not fit the frame: %w", f.Name, err)
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
		if _, err := assembly.PlaceHandlebar(pose.NewPose(pose.Point{}, 0), opt.Setup); err != nil {
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
		if _, err := assembly.SeatpostInsertionBounds(f.Chart.Frame, opt.Spec); err != nil {
			return fmt.Errorf("fixture %q seatpost %q invalid: %w", f.Name, opt.ID, err)
		}
	}
	if _, err := assembly.PlaceSeatpost(pose.NewPose(pose.Point{}, 0), f.Chart.Frame, assembly.SeatpostSetup{Spec: defaultPost.Spec, InsertionMm: f.DefaultPostInsertionMm}); err != nil {
		return fmt.Errorf("fixture %q default seatpost %q at DefaultPostInsertionMm invalid: %w", f.Name, f.DefaultSeatpostID, err)
	}
	return nil
}

// defaultSuspensionSagPct is the sag a suspension fork option starts at.
const defaultSuspensionSagPct = 20.0 // Q32 ideal sag

// otsoFenrirFixture returns the Otso Fenrir Stainless (2024, Size L) workbench,
// built from the catalog's frame, forks, parts, and fit targets plus the
// Fenrir-L workbench defaults below.
func otsoFenrirFixture() bikeFixture {
	const frameID, sizeName = "otso-fenrir-stainless", "L"
	frame, ok := catalog.FrameByID(frameID)
	if !ok {
		panic(fmt.Sprintf("catalog has no frame %q", frameID))
	}
	size, ok := frame.Size(sizeName)
	if !ok {
		panic(fmt.Sprintf("catalog frame %q has no size %q", frameID, sizeName))
	}
	chart := size.Chart
	// Q67: sliding dropouts at max chainstay (450 mm, the L's published value)
	chart.Frame = frame.InstalledFrame(size)
	forks := make([]forkOption, 0, len(frame.Forks))
	for _, ff := range frame.Forks {
		sag := 0.0
		if ff.Fork.TravelMm != 0 {
			sag = defaultSuspensionSagPct
		}
		forks = append(forks, makeForkOption(ff.Fork.ID, ff.Fork.Name, ff.Spec(), sag, chart.Frame.DefaultLowerCup))
	}
	saddle := catalog.DefaultSaddle()
	return bikeFixture{
		Name:          fmt.Sprintf("%s %s (%d, Size %s)", frame.Manufacturer, frame.Model, frame.Year, size.Name),
		Chart:         chart,
		Wheels:        assembly.WheelSetup{RadiusMm: catalog.WheelRadiusMm},
		WheelLabel:    catalog.WheelLabel,
		DefaultForkID: "lithic_carbon_mountain",
		Forks:         forks,
		DefaultStem: assembly.StemSetup{
			UpperStackMm:  assembly.DefaultUpperStackMm,
			SpacersMm:     20.0,
			ClampHeightMm: 40.0,
			LengthMm:      60.0,
			AngleDeg:      -6.0,
			MaxSpacersMm:  assembly.DefaultMaxSpacersMm,
			MinLengthMm:   40.0, // illustrative stem length range
			MaxLengthMm:   130.0,
		},
		CockpitStock: assembly.CockpitStock{ // illustrative stock stems and spacers
			StemLengthStepMm: 10,
			StemAnglesDeg:    []pose.Deg{-17, -6, 0, 6, 17},
			SpacerStepMm:     5,
		},
		CockpitPreferences: assembly.CockpitPreferences{
			GripOval:   assembly.FitOval{SemiMinorMm: 10, SemiMajorMm: 20, MajorAxisHeadingDeg: 60, PointsPerR: 22},      // legacy idmatch oval (Q17)
			StemLength: assembly.PenaltyBand{FreeMin: 70, FreeMax: 80, PointsPerUnitBelow: 0.2, PointsPerUnitAbove: 0.2}, // illustrative
			StemAngle:  assembly.PenaltyBand{PointsPerUnitBelow: 0.2, PointsPerUnitAbove: 0.2},                           // illustrative
			Spacers:    assembly.PenaltyBand{FreeMax: 20, PointsPerUnitAbove: 0.4},                                       // illustrative
			BarSweep:   assembly.PenaltyBand{FreeMin: 16, FreeMax: 20, PointsPerUnitBelow: 2, PointsPerUnitAbove: 2},     // Matt's preference (Q30)
			ForkSag:    assembly.PenaltyBand{FreeMin: 20, FreeMax: 20, PointsPerUnitBelow: 2, PointsPerUnitAbove: 2},     // Matt's preference (Q32)
			BrpMiss:    assembly.PenaltyBand{FreeMax: 1, PointsPerUnitAbove: 2},                                          // illustrative (Q32)
		},
		StockSagPcts:       catalog.SuspensionSagPcts(),
		DefaultHandlebarID: "persuader20_800",
		Handlebars:         catalog.Handlebars(),
		DefaultSeatpostID:  "setback_0",
		// Q92: 180.576 mm of insertion exposes 219.424 mm of the generic post and puts the
		// BRP 746.000 mm radially from BB in the unpitched attitude.
		DefaultPostInsertionMm: 180.576,
		Seatposts:              catalog.Seatposts(),
		SaddleName:             saddle.Name,
		DefaultSaddle:          saddle.Setup,
		ExampleBrpTarget:       catalog.DefaultBrpTarget(),
		ExampleGripTarget:      catalog.DefaultGripTarget(),
	}
}

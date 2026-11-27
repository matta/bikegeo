package assembly

import (
	"fmt"

	"github.com/matta/bikegeo/internal/bounds"
	"github.com/matta/bikegeo/internal/pose"
)

// RearCockpitDoF represents a Frameset mated with a seatpost and saddle whose
// insertion depth and fore-aft rail offset are still open. Dynamic saddle rail
// pitch is carried so Clamp and FitToBRP lock the physical clamp angle in
// DynamicWorld.
type RearCockpitDoF struct {
	frameset            *Frameset
	postDoF             *SeatpostDoF
	saddleDoF           *SaddleDoF
	dynamicRailPitchDeg pose.Deg
	dynFrameset         FramesetPlacement
}

// OpenRearCockpit mates post and saddle onto frameset's seat-tube socket,
// validating insertion and rail-clamp compatibility immediately.
func OpenRearCockpit(frameset *Frameset, post postMounter, saddle Saddle, dynamicRailPitchDeg pose.Deg) (*RearCockpitDoF, error) {
	frameset.requireValid()
	saddle.requireValid()
	if !isFinite(float64(dynamicRailPitchDeg)) {
		return nil, invalidInput(bounds.InputSaddleRailPitch, "saddle rail pitch must be finite, got %v°", dynamicRailPitchDeg)
	}
	postDoF, err := MateSeatpost(frameset.SeatTube(), post)
	if err != nil {
		return nil, err
	}
	saddleDoF, err := MateSaddle(post.RailClamp(bounds.Dynamic), saddle)
	if err != nil {
		return nil, err
	}
	return &RearCockpitDoF{
		frameset:            frameset,
		postDoF:             postDoF,
		saddleDoF:           saddleDoF,
		dynamicRailPitchDeg: dynamicRailPitchDeg,
		dynFrameset:         frameset.Place(bounds.Dynamic),
	}, nil
}

func (d *RearCockpitDoF) requireValid() {
	if d == nil || d.frameset == nil || d.postDoF == nil || d.saddleDoF == nil {
		panic("assembly: uninitialized RearCockpitDoF")
	}
}

// AllowedInsertion returns the intersected seatpost insertion bounds.
func (d *RearCockpitDoF) AllowedInsertion() bounds.BoundedRange {
	d.requireValid()
	return d.postDoF.AllowedInsertion()
}

// Planar2DoF returns the 2-DoF affine BRP surface in DynamicWorld parameterized
// by (SeatpostInsertion, SaddleRailOffset).
func (d *RearCockpitDoF) Planar2DoF() Planar2DoF {
	d.requireValid()
	insRange := d.postDoF.AllowedInsertion()
	railRange := d.saddleDoF.AllowedRailOffset()

	// Probe at maximum insertion (shortest exposure) and 0 mm rail offset:
	// pulling the post out lowers insertion (Sign == -1) while advancing BRP
	// upward along the seatpost shaft heading; increasing rail offset slides
	// the saddle rearward (180° opposite the rail heading, Sign == +1).
	probeIns := insRange.Max()
	clampedPost, err := d.postDoF.Clamp(probeIns)
	if err != nil {
		panic(fmt.Sprintf("assembly: internal probe Clamp(%v) failed: %v", probeIns, err))
	}
	postDyn := clampedPost.PlaceInWorld(d.dynFrameset.FrameToWorld)
	railTurn := RailTurnForWorldPitch(postDyn.CradleInWorld.HeadingInDst(), d.dynamicRailPitchDeg)
	clampedSaddle, err := d.saddleDoF.Clamp(0, railTurn)
	if err != nil {
		panic(fmt.Sprintf("assembly: internal probe saddle Clamp failed: %v", err))
	}
	saddleDyn := clampedSaddle.Place(postDyn.CradleInWorld)

	return Planar2DoF{
		Origin: saddleDyn.BRP.Origin(),
		AxisA: LinearDoF{
			Field:   bounds.InputSeatpostInsertion,
			Heading: postDyn.ShaftTop.Heading(),
			Range:   insRange,
			Probe:   probeIns,
			Sign:    -1,
		},
		AxisB: LinearDoF{
			Field:   bounds.InputSaddleRailOffset,
			Heading: saddleDyn.RailCenter.Heading().Turn(180),
			Range:   railRange,
			Probe:   0,
			Sign:    +1,
		},
	}
}

// Clamp locks the seatpost at insertionMm and the saddle at railOffsetMm with
// its physical clamp angle solved in DynamicWorld to achieve
// dynamicRailPitchDeg, returning an immutable SeatedFrameset.
func (d *RearCockpitDoF) Clamp(insertionMm, railOffsetMm float64) (*SeatedFrameset, error) {
	d.requireValid()
	clampedPost, err := d.postDoF.Clamp(insertionMm)
	if err != nil {
		return nil, err
	}
	postDyn := clampedPost.PlaceInWorld(d.dynFrameset.FrameToWorld)
	railTurn := RailTurnForWorldPitch(postDyn.CradleInWorld.HeadingInDst(), d.dynamicRailPitchDeg)
	clampedSaddle, err := d.saddleDoF.Clamp(railOffsetMm, railTurn)
	if err != nil {
		return nil, err
	}
	saddleDyn := clampedSaddle.Place(postDyn.CradleInWorld)
	if err := validatePlacementPoints("seatpost", d.dynFrameset.SeatTubeTop.Origin(), postDyn.ShaftTop.Origin(), postDyn.RailPivot.Origin()); err != nil {
		return nil, err
	}
	if err := validatePlacementPoints("saddle", postDyn.RailPivot.Origin(), saddleDyn.RailCenter.Origin(), saddleDyn.BRP.Origin(), saddleDyn.SitBones.Origin()); err != nil {
		return nil, err
	}
	return &SeatedFrameset{
		frameset:            d.frameset,
		seatpost:            clampedPost,
		saddle:              clampedSaddle,
		dynamicRailPitchDeg: d.dynamicRailPitchDeg,
		dynFrameset:         d.dynFrameset,
		dynSeatpost:         postDyn,
		dynSaddle:           saddleDyn,
	}, nil
}

// FitToBRP solves the rear cockpit's Planar2DoF for target with PriorityBRPGate
// and clamps the resulting SeatedFrameset.
func (d *RearCockpitDoF) FitToBRP(target pose.Point) (*SeatedFrameset, []bounds.InputField, error) {
	d.requireValid()
	if !isFinitePoint(target) {
		return nil, nil, invalidInput(bounds.InputFitTargetBRP, "BRP fit target must be finite, got (%v, %v) mm", target.X(), target.Y())
	}
	ins, rail, limited, ok := SolvePlanar2DoF(d.Planar2DoF(), target, PriorityBRPGate)
	if !ok {
		return nil, nil, fmt.Errorf("saddle rail pitch is parallel to the seat axis; cannot fit the saddle")
	}
	seated, err := d.Clamp(ins, rail)
	if err != nil {
		return nil, nil, err
	}
	if len(limited) == 0 {
		if miss := target.Sub(seated.dynSaddle.BRP.Origin()); miss.Norm() > 1e-6 {
			return nil, nil, fmt.Errorf("saddle fitting forward check missed target by %v mm", miss.Norm())
		}
	}
	return seated, limited, nil
}

// SeatedFrameset is an immutable frameset with its seatpost and saddle clamped
// at validated bounds and locked physical clamp angle.
type SeatedFrameset struct {
	frameset            *Frameset
	seatpost            *ClampedSeatpost
	saddle              *ClampedSaddle
	dynamicRailPitchDeg pose.Deg
	dynFrameset         FramesetPlacement
	dynSeatpost         SeatpostPlacement
	dynSaddle           SaddlePlacement
}

func (s *SeatedFrameset) requireValid() {
	if s == nil || s.frameset == nil || s.seatpost == nil || s.saddle == nil {
		panic("assembly: uninitialized SeatedFrameset")
	}
}

// Frameset returns the underlying mated Frameset.
func (s *SeatedFrameset) Frameset() *Frameset {
	s.requireValid()
	return s.frameset
}

// Seatpost returns the clamped seatpost.
func (s *SeatedFrameset) Seatpost() *ClampedSeatpost {
	s.requireValid()
	return s.seatpost
}

// Saddle returns the clamped saddle.
func (s *SeatedFrameset) Saddle() *ClampedSaddle {
	s.requireValid()
	return s.saddle
}

// DynamicFrameset returns the frameset's DynamicWorld placement.
func (s *SeatedFrameset) DynamicFrameset() FramesetPlacement {
	s.requireValid()
	return s.dynFrameset
}

// DynamicSeatpost returns the clamped seatpost's DynamicWorld placement.
func (s *SeatedFrameset) DynamicSeatpost() SeatpostPlacement {
	s.requireValid()
	return s.dynSeatpost
}

// DynamicSaddle returns the clamped saddle's DynamicWorld placement.
func (s *SeatedFrameset) DynamicSaddle() SaddlePlacement {
	s.requireValid()
	return s.dynSaddle
}

// AttachFront mounts spacers, stem, and handlebar onto s's steerer port,
// locking the physical handlebar roll turn in DynamicWorld to achieve
// dynamicBarRollDeg, and returns an immutable Build.
func (s *SeatedFrameset) AttachFront(spacersMm float64, stem Stem, bar Handlebar, dynamicBarRollDeg pose.Deg) (*Build, error) {
	s.requireValid()
	bar.requireValid()
	if !isFinite(float64(dynamicBarRollDeg)) {
		return nil, invalidInput(bounds.InputHandlebarRoll, "handlebar roll must be finite, got %v°", dynamicBarRollDeg)
	}
	clampedStem, err := MountStem(s.frameset.Steerer(), spacersMm, stem)
	if err != nil {
		return nil, err
	}
	stemDyn := clampedStem.Place(s.dynFrameset.FrameToWorld)
	if err := validatePlacementPoints("stem", stemDyn.HeadTubeTop.Origin(), stemDyn.SteererClamp.Origin(), stemDyn.BarClamp.Origin()); err != nil {
		return nil, err
	}
	rollTurn := BarTurnForWorldRoll(stemDyn.BarClampInWorld.HeadingInDst(), dynamicBarRollDeg)
	clampedBar, err := MountHandlebar(clampedStem.BarClampInFrame(), bar, rollTurn)
	if err != nil {
		return nil, err
	}
	barDyn := clampedBar.Place(stemDyn.BarClampInWorld)
	if err := validatePlacementPoints("handlebar", barDyn.BarClamp.Origin(), barDyn.BarCenter.Origin(), barDyn.HandGrip.Origin()); err != nil {
		return nil, err
	}
	return &Build{
		seated:            s,
		stem:              clampedStem,
		handlebar:         clampedBar,
		dynamicBarRollDeg: dynamicBarRollDeg,
	}, nil
}

// Build is a complete, immutable, physically clamped bicycle assembly whose
// joints move rigidly across Static and Dynamic load states.
type Build struct {
	seated            *SeatedFrameset
	stem              ClampedStem
	handlebar         ClampedHandlebar
	dynamicBarRollDeg pose.Deg
}

func (b *Build) requireValid() {
	if b == nil || b.seated == nil {
		panic("assembly: uninitialized Build")
	}
}

// SeatedFrameset returns the rear sub-assembly of b.
func (b *Build) SeatedFrameset() *SeatedFrameset {
	b.requireValid()
	return b.seated
}

// Bounds returns the clamped hardware bound statuses for b's three bounded inputs.
func (b *Build) Bounds() BuildBounds {
	b.requireValid()
	return BuildBounds{
		SeatpostInsertion: b.seated.seatpost.Insertion(),
		SaddleRailOffset:  b.seated.saddle.RailOffset(),
		StemSpacers:       b.stem.Spacers(),
	}
}

// Place runs the unified forward transform chain for state (Static or Dynamic),
// moving all clamped saddle, stem, and handlebar joints rigidly with the
// frameset pitch and seatpost cradle motion.
func (b *Build) Place(state bounds.LoadState) Placement {
	b.requireValid()
	var fsPlaced FramesetPlacement
	var postPlaced SeatpostPlacement
	var saddlePlaced SaddlePlacement
	switch state {
	case bounds.Dynamic:
		fsPlaced = b.seated.dynFrameset
		postPlaced = b.seated.dynSeatpost
		saddlePlaced = b.seated.dynSaddle
	case bounds.Static:
		fsPlaced = b.seated.frameset.Place(bounds.Static)
		postPlaced = b.seated.seatpost.PlaceInWorld(fsPlaced.FrameToWorld)
		saddlePlaced = b.seated.saddle.Place(postPlaced.CradleInWorld)
	default:
		panic(fmt.Sprintf("assembly: invalid LoadState %q", state))
	}
	stemPlaced := b.stem.Place(fsPlaced.FrameToWorld)
	barPlaced := b.handlebar.Place(stemPlaced.BarClampInWorld)

	p := Placement{
		State:     state,
		Setup:     b.setupSnapshot(state),
		Frameset:  fsPlaced,
		Seatpost:  postPlaced,
		Saddle:    saddlePlaced,
		Stem:      stemPlaced,
		Handlebar: barPlaced,
		Bounds:    b.Bounds(),
	}
	p.Summary = summarizePlacement(p)
	return p
}

func (b *Build) setupSnapshot(state bounds.LoadState) BikeSetup {
	saddleDims := b.seated.saddle.Saddle().Dimensions()
	barDims := b.handlebar.Handlebar().Dimensions()
	stem := b.stem.Stem()
	return BikeSetup{
		Frame:    b.seated.frameset.frameSetupSnapshot(),
		LowerCup: b.seated.frameset.LowerCup(),
		Fork:     b.seated.frameset.forkSetupSnapshot(),
		Wheels:   b.seated.frameset.Wheels(),
		Seatpost: b.seated.seatpost.setupSnapshot(state),
		Saddle: SaddleSetup{
			RailCenterToBrpMm:      saddleDims.RailCenterToBrpMm,
			RailCenterToSitBonesMm: saddleDims.RailCenterToSitBonesMm,
			RailToBrpHeightMm:      saddleDims.RailToBrpHeightMm,
			SitBonesElevationMm:    saddleDims.SitBonesElevationMm,
			RailLengthMm:           saddleDims.RailLengthMm,
			RailPitchDeg:           b.seated.dynamicRailPitchDeg,
			RailOffsetMm:           b.seated.saddle.RailOffset().Value,
		},
		Stem: StemSetup{
			UpperStackMm:  b.stem.UpperStackMm(),
			SpacersMm:     b.stem.Spacers().Value,
			ClampHeightMm: stem.ClampHeightMm(),
			LengthMm:      stem.LengthMm(),
			AngleDeg:      stem.AngleDeg(),
			MaxSpacersMm:  b.stem.Spacers().Max,
		},
		Handlebar: HandlebarSetup{
			StockWidthMm:   barDims.StockWidthMm,
			CurrentWidthMm: barDims.CurrentWidthMm,
			BacksweepDeg:   barDims.BacksweepDeg,
			UpsweepDeg:     barDims.UpsweepDeg,
			StatedRiseMm:   barDims.StatedRiseMm,
			RiseMode:       barDims.RiseMode,
			BendStartMm:    barDims.BendStartMm,
			GripInsetMm:    barDims.GripInsetMm,
			BarRollDeg:     b.dynamicBarRollDeg,
		},
	}
}

// BuildBounds holds the BoundStatus snapshots for a build's bounded hardware inputs.
type BuildBounds struct {
	SeatpostInsertion bounds.BoundStatus
	SaddleRailOffset  bounds.BoundStatus
	StemSpacers       bounds.BoundStatus
}

// BuildSummary holds derived geometric readouts computed from a Placement.
type BuildSummary struct {
	PitchDeg             pose.Deg
	LeveledHeadTubeDeg   pose.Deg
	LeveledSeatTubeDeg   pose.Deg
	WheelbaseMm          float64
	TrailMm              float64
	RearCenterMm         float64
	RcWbRatioPct         float64
	LeveledBbDropMm      float64
	LeveledReachMm       float64
	LeveledStackMm       float64
	EttMm                float64
	SeatAxisToPostTopMm  float64
	SaddleHeightMm       float64
	BrpToGripDistMm      float64
	SitBonesToGripRiseMm float64
}

func summarizePlacement(p Placement) BuildSummary {
	return BuildSummary{
		PitchDeg:             p.Frameset.PitchDeg(),
		LeveledHeadTubeDeg:   p.Frameset.LeveledHeadTubeAngleDeg(),
		LeveledSeatTubeDeg:   p.Frameset.LeveledSeatTubeAngleDeg(),
		WheelbaseMm:          p.Frameset.WheelbaseMm(),
		TrailMm:              p.Frameset.TrailMm(),
		RearCenterMm:         p.Frameset.RearCenterMm(),
		RcWbRatioPct:         p.Frameset.RcWbRatioPct(),
		LeveledBbDropMm:      p.Frameset.LeveledBbDropMm(),
		LeveledReachMm:       p.Frameset.LeveledReachMm(),
		LeveledStackMm:       p.Frameset.LeveledStackMm(),
		EttMm:                p.Frameset.EttMm(),
		SeatAxisToPostTopMm:  p.SeatAxisToPostTopMm(),
		SaddleHeightMm:       p.SaddleHeightMm(),
		BrpToGripDistMm:      p.BrpToGripDistMm(),
		SitBonesToGripRiseMm: p.SitBonesToGripRiseMm(),
	}
}

// Placement holds the leveled whole-bike landmark readouts for a LoadState
// alongside its fitted input snapshot, hardware bound statuses, and derived
// geometry summary.
type Placement struct {
	State     bounds.LoadState
	Setup     BikeSetup
	Frameset  FramesetPlacement
	Seatpost  SeatpostPlacement
	Saddle    SaddlePlacement
	Stem      StemPlacement
	Handlebar HandlebarPlacement
	Bounds    BuildBounds
	Summary   BuildSummary
}

// SeatAxisToPostTopMm returns the axial distance from BB along the seat tube
// axis to the top of the exposed seatpost shaft (ShaftTop).
func (b Placement) SeatAxisToPostTopMm() float64 {
	return b.Frameset.SeatAxis.Origin().DistanceTo(b.Seatpost.ShaftTop.Origin())
}

// SaddleHeightMm returns the radial distance from BB to the saddle BRP (|BRP - BB|).
func (b Placement) SaddleHeightMm() float64 {
	return b.Frameset.BB.Origin().DistanceTo(b.Saddle.BRP.Origin())
}

// BrpToGripDistMm returns the sagittal Euclidean distance from saddle BRP to HandGrip
// (the cockpit reach-distance reference anchor).
func (b Placement) BrpToGripDistMm() float64 {
	return b.Saddle.BRP.Origin().DistanceTo(b.Handlebar.HandGrip.Origin())
}

// SitBonesToGripRiseMm returns the vertical rise of HandGrip above SitBones.
// Measured from SitBones rather than BRP so rider-fit cockpit elevation carries
// consistently across saddles through the sit-bone contact point.
func (b Placement) SitBonesToGripRiseMm() float64 {
	return b.Saddle.SitBones.Origin().VerticalOffsetTo(b.Handlebar.HandGrip.Origin())
}

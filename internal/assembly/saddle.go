package assembly

import (
	"fmt"

	"github.com/matta/bikegeo/internal/bounds"
	"github.com/matta/bikegeo/internal/pose"
)

// SaddleLocal has origin (0, 0) at RailCenter (midpoint of the straight rail
// clamping span), +X pointing forward along the rails toward the nose, and +Y
// pointing 90° CCW (perpendicular upward from the rail line).
var SaddleLocal = pose.NewCoordinateSystem("part:saddle")

// SaddleDimensions specifies a saddle's intrinsic geometry in SaddleLocal
// coordinates (anchored at RailCenter):
//   - RailCenterToBrpMm: longitudinal offset along the rail line (+X toward nose) from RailCenter to BRP.
//   - RailCenterToSitBonesMm: longitudinal offset along the rail line (+X toward nose) from RailCenter to SitBones.
//   - RailToBrpHeightMm: perpendicular offset (rail +Y, 90° CCW) from the rail line up to BRP (>= 0).
//   - SitBonesElevationMm: perpendicular offset (rail +Y) of SitBones above BRP, so
//     SitBones sits RailToBrpHeightMm + SitBonesElevationMm above the rail line.
//   - RailLengthMm: straight clamping span of the saddle rails (> 0).
type SaddleDimensions struct {
	RailCenterToBrpMm      float64
	RailCenterToSitBonesMm float64
	RailToBrpHeightMm      float64
	SitBonesElevationMm    float64
	RailLengthMm           float64
}

// Saddle is a validated saddle geometry in SaddleLocal with unexported fields.
// Construct instances with NewSaddle.
type Saddle struct {
	dims  SaddleDimensions
	brp   pose.Point
	sit   pose.Point
	valid bool
}

// NewSaddle validates dims and constructs a Saddle whose BRP and SIT landmarks
// are precomputed in SaddleLocal.
func NewSaddle(dims SaddleDimensions) (Saddle, error) {
	if !isFinite(dims.RailCenterToBrpMm) {
		return Saddle{}, fmt.Errorf("saddle rail-center-to-BRP offset must be finite, got %v mm", dims.RailCenterToBrpMm)
	}
	if !isFinite(dims.RailCenterToSitBonesMm) {
		return Saddle{}, fmt.Errorf("saddle rail-center-to-sit-bones offset must be finite, got %v mm", dims.RailCenterToSitBonesMm)
	}
	if !isFinite(dims.RailToBrpHeightMm) || dims.RailToBrpHeightMm < 0 {
		return Saddle{}, fmt.Errorf("saddle rail-to-BRP height must be finite and non-negative, got %v mm", dims.RailToBrpHeightMm)
	}
	if !isFinite(dims.SitBonesElevationMm) {
		return Saddle{}, fmt.Errorf("saddle sit-bones elevation must be finite, got %v mm", dims.SitBonesElevationMm)
	}
	if !isFinite(dims.RailLengthMm) || dims.RailLengthMm <= 0 {
		return Saddle{}, fmt.Errorf("saddle rail length must be finite and positive, got %v mm", dims.RailLengthMm)
	}
	sitHeight := dims.RailToBrpHeightMm + dims.SitBonesElevationMm
	if !isFinite(sitHeight) {
		return Saddle{}, fmt.Errorf("saddle sit-bones height must be finite, got %v mm", sitHeight)
	}
	return Saddle{
		dims:  dims,
		brp:   pose.NewPoint(SaddleLocal, dims.RailCenterToBrpMm, dims.RailToBrpHeightMm),
		sit:   pose.NewPoint(SaddleLocal, dims.RailCenterToSitBonesMm, sitHeight),
		valid: true,
	}, nil
}

func (s Saddle) requireValid() {
	if !s.valid {
		panic("assembly: uninitialized zero-value Saddle")
	}
}

// BRP returns the Biomechanical Reference Point in SaddleLocal.
func (s Saddle) BRP() pose.Point {
	s.requireValid()
	return s.brp
}

// SIT returns the sit-bone support landmark in SaddleLocal.
func (s Saddle) SIT() pose.Point {
	s.requireValid()
	return s.sit
}

// RailLengthMm returns the straight clamping span of the saddle rails.
func (s Saddle) RailLengthMm() float64 {
	s.requireValid()
	return s.dims.RailLengthMm
}

// Dimensions returns the validated SaddleDimensions used to construct s.
func (s Saddle) Dimensions() SaddleDimensions {
	s.requireValid()
	return s.dims
}

// SaddleDoF represents a saddle mated onto a RailClampPort with its fore-aft
// rail slide and tilt angle still open.
type SaddleDoF struct {
	saddle            Saddle
	allowedRailOffset bounds.BoundedRange
}

// MateSaddle mates saddle onto clamp, verifying that clamp.CradleInParent maps
// SeatpostCradle into a valid parent coordinate system and that the clamp
// footprint fits on the saddle rails. It panics deterministically on a zero
// Saddle or wrong-CoordinateSystem RailClampPort.
func MateSaddle(clamp RailClampPort, saddle Saddle) (*SaddleDoF, error) {
	saddle.requireValid()
	if !clamp.CradleInParent.Src().Valid() || !clamp.CradleInParent.Dst().Valid() {
		panic("assembly: RailClampPort.CradleInParent has uninitialized CoordinateSystem")
	}
	if clamp.CradleInParent.Src() != SeatpostCradle {
		panic(fmt.Sprintf("assembly: MateSaddle coordinate system mismatch (want source %s, got %s → %s)",
			SeatpostCradle, clamp.CradleInParent.Src(), clamp.CradleInParent.Dst()))
	}
	if !isFinitePoint(clamp.CradleInParent.OriginInDst()) || !isFinite(float64(clamp.CradleInParent.HeadingInDst().Deg())) {
		return nil, fmt.Errorf("rail clamp transform must be finite")
	}
	if !isFinite(clamp.ClampLengthMm) || clamp.ClampLengthMm <= 0 {
		return nil, fmt.Errorf("seatpost clamp length must be finite and positive, got %v mm", clamp.ClampLengthMm)
	}
	limit, err := railOffsetLimitMm(saddle.RailLengthMm(), clamp.ClampLengthMm)
	if err != nil {
		return nil, err
	}
	allowed, err := bounds.NewBoundedRange(-limit, limit, "saddle rail forward limit", "saddle rail rearward limit")
	if err != nil {
		return nil, err
	}
	return &SaddleDoF{
		saddle:            saddle,
		allowedRailOffset: allowed,
	}, nil
}

// AllowedRailOffset returns the legal fore-aft rail offset interval
// [−(RailLengthMm − ClampLengthMm)/2, +(RailLengthMm − ClampLengthMm)/2].
func (d *SaddleDoF) AllowedRailOffset() bounds.BoundedRange {
	if d == nil || !d.allowedRailOffset.Valid() {
		panic("assembly: uninitialized SaddleDoF")
	}
	return d.allowedRailOffset
}

// RailTurnForWorldPitch returns the physical clamp rotation (CCW relative to
// SeatpostCradle's +X shaft-parallel heading) that sets the saddle rails to
// worldPitchDeg in cradleHeadingInWorld's ambient world coordinate system
// (StaticWorld or DynamicWorld). Positive worldPitchDeg raises the nose CCW.
// It panics deterministically if cradleHeadingInWorld is not in StaticWorld or
// DynamicWorld.
func RailTurnForWorldPitch(cradleHeadingInWorld pose.Heading, worldPitchDeg pose.Deg) pose.Deg {
	if _, ok := bounds.LoadStateForWorld(cradleHeadingInWorld.CoordinateSystem()); !ok {
		panic(fmt.Sprintf("assembly: RailTurnForWorldPitch requires %s or %s heading, got %s",
			bounds.StaticWorld, bounds.DynamicWorld, cradleHeadingInWorld.CoordinateSystem()))
	}
	return pose.NewHeading(cradleHeadingInWorld.CoordinateSystem(), worldPitchDeg).Sub(cradleHeadingInWorld)
}

// Clamp locks the saddle at railOffsetMm along the rails (positive slides the
// saddle rearward, advancing RailCenter by −railOffsetMm along the rail heading)
// and physical bolt angle railTurnFromCradle (CCW relative to SeatpostCradle's
// +X axis).
func (d *SaddleDoF) Clamp(railOffsetMm float64, railTurnFromCradle pose.Deg) (*ClampedSaddle, error) {
	if d == nil || !d.allowedRailOffset.Valid() {
		panic("assembly: uninitialized SaddleDoF")
	}
	if !isFinite(float64(railTurnFromCradle)) {
		return nil, invalidInput(bounds.InputSaddleRailPitch, "saddle rail pitch must be finite, got %v°", railTurnFromCradle)
	}
	status, err := d.allowedRailOffset.Bind(bounds.InputSaddleRailOffset, railOffsetMm)
	if err != nil {
		return nil, err
	}
	// Rotate SeatpostCradle's origin to the clamped rail heading, then slide
	// RailCenter rearward by -railOffsetMm along that rail heading.
	railClampInCradle := pose.OriginPose(SeatpostCradle).Turn(railTurnFromCradle)
	railCenterInCradle := railClampInCradle.Advance(-railOffsetMm)
	saddleInCradle := railCenterInCradle.Anchor(SaddleLocal)
	return &ClampedSaddle{
		saddle:             d.saddle,
		railOffset:         status,
		railTurnFromCradle: railTurnFromCradle,
		railClampInCradle:  railClampInCradle,
		saddleInCradle:     saddleInCradle,
	}, nil
}

// ClampedSaddle is a saddle locked at a validated rail offset and physical
// clamp angle relative to SeatpostCradle. Placing it through different
// cradleInTarget transforms (StaticWorld vs. DynamicWorld) moves the saddle
// rigidly with the post cradle without re-leveling to gravity.
type ClampedSaddle struct {
	saddle             Saddle
	railOffset         bounds.BoundStatus
	railTurnFromCradle pose.Deg
	railClampInCradle  pose.Pose
	saddleInCradle     pose.Transform
}

func (c *ClampedSaddle) requireValid() {
	if c == nil || !c.saddle.valid {
		panic("assembly: uninitialized ClampedSaddle")
	}
}

// Saddle returns the clamped Saddle specification.
func (c *ClampedSaddle) Saddle() Saddle {
	c.requireValid()
	return c.saddle
}

// RailOffset returns the clamped fore-aft rail offset and its bound provenance.
func (c *ClampedSaddle) RailOffset() bounds.BoundStatus {
	c.requireValid()
	return c.railOffset
}

// SaddlePlacement holds the Pose-valued landmarks produced for a clamped saddle
// in a target coordinate system:
//   - RailPivot: incoming cradle attachment pose (oriented along the seatpost shaft).
//   - RailClamp: the seatpost's rail clamp, centered on RailPivot and turned to
//     lie along the clamped saddle rail orientation.
//   - RailCenter: midpoint of the straight rail clamping section (along the rail line).
//   - BRP: Biomechanical Reference Point (+RailCenterToBrpMm, +RailToBrpHeightMm) in SaddleLocal.
//   - SitBones: modeled sit-bone support landmark (RailCenterToSitBonesMm,
//     RailToBrpHeightMm + SitBonesElevationMm) in SaddleLocal.
//
// RailClamp, RailCenter, BRP, and SitBones all point forward along the rail
// toward the nose (local +Y 90° CCW, perpendicular upward), anchored at each
// landmark's own Origin().
type SaddlePlacement struct {
	RailPivot  pose.Pose
	RailClamp  pose.Pose
	RailCenter pose.Pose
	BRP        pose.Pose
	SitBones   pose.Pose
}

// Place maps the clamped saddle's landmarks through cradleInTarget
// (SeatpostCradle → target coordinate system, such as StaticWorld, DynamicWorld,
// or FrameLocal). It panics deterministically if cradleInTarget does not have
// SeatpostCradle as its source coordinate system.
func (c *ClampedSaddle) Place(cradleInTarget pose.Transform) SaddlePlacement {
	c.requireValid()
	saddleInTarget := pose.Compose(c.saddleInCradle, cradleInTarget)
	railPivot := cradleInTarget.PoseInDst()
	railClamp := cradleInTarget.ApplyPose(c.railClampInCradle)
	railCenter := saddleInTarget.PoseInDst()
	brp := saddleInTarget.ApplyPose(pose.NewPose(c.saddle.BRP(), 0))
	sitBones := saddleInTarget.ApplyPose(pose.NewPose(c.saddle.SIT(), 0))
	return SaddlePlacement{
		RailPivot:  railPivot,
		RailClamp:  railClamp,
		RailCenter: railCenter,
		BRP:        brp,
		SitBones:   sitBones,
	}
}

// railOffsetLimitMm returns the symmetric rail offset bound
// (railLengthMm − clampLengthMm)/2: the clamp head must stay on the rails.
func railOffsetLimitMm(railLengthMm, clampLengthMm float64) (float64, error) {
	limit := (railLengthMm - clampLengthMm) / 2
	if !isFinite(limit) || limit < 0 {
		return 0, fmt.Errorf("seatpost clamp length %v mm exceeds saddle rail length %v mm", clampLengthMm, railLengthMm)
	}
	return limit, nil
}

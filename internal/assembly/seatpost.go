package assembly

import (
	"fmt"

	"bikegeo/internal/pose"
)

// frameInsertionFallbackMm is how far below the seat tube's C-T length the post
// may reach when the frame declares no maximum insertion.
const frameInsertionFallbackMm = 20.0

// SeatpostSpec describes a catalog seatpost:
//   - TotalLengthMm: overall post length.
//   - MaxInsertionMm, MinInsertionMm: the post's own insertion limits (its
//     max-insertion and minimum-insertion marks).
//   - SetbackMm: perpendicular rearward offset from the shaft axis to the rail cradle.
//   - RailClampLengthMm: fore-aft footprint of the post's rail clamp head.
type SeatpostSpec struct {
	TotalLengthMm, MaxInsertionMm, MinInsertionMm float64
	SetbackMm, RailClampLengthMm                  float64
}

// SeatpostSetup is an installed seatpost: its spec and how far the shaft
// reaches into the seat tube.
type SeatpostSetup struct {
	Spec        SeatpostSpec
	InsertionMm float64
}

// ExposedMm returns the post length above the seat tube top.
func (s SeatpostSetup) ExposedMm() float64 {
	return s.Spec.TotalLengthMm - s.InsertionMm
}

// InsertionBounds is the insertion interval a frame and post allow together.
type InsertionBounds struct {
	EffectiveMinInsertionMm, MaxAllowedInsertionMm float64
}

// Validate checks the post's own dimensions.
func (p SeatpostSpec) Validate() error {
	for _, v := range []float64{p.TotalLengthMm, p.MaxInsertionMm, p.MinInsertionMm} {
		if !isFinite(v) {
			return fmt.Errorf("seatpost length and insertion limits must be finite, got %v, %v, %v mm", p.TotalLengthMm, p.MaxInsertionMm, p.MinInsertionMm)
		}
	}
	if !(0 < p.MinInsertionMm && p.MinInsertionMm <= p.MaxInsertionMm && p.MaxInsertionMm < p.TotalLengthMm) {
		return fmt.Errorf("seatpost needs 0 < min insertion <= max insertion < total length, got min %v, max %v, total %v mm", p.MinInsertionMm, p.MaxInsertionMm, p.TotalLengthMm)
	}
	if !isFinite(p.SetbackMm) || p.SetbackMm < 0 {
		return invalidInput(InputSeatpostSetback, "seatpost setback must be finite and non-negative, got %v mm", p.SetbackMm)
	}
	if !isFinite(p.RailClampLengthMm) || p.RailClampLengthMm <= 0 {
		return fmt.Errorf("seatpost clamp length must be finite and positive, got %v mm", p.RailClampLengthMm)
	}
	return nil
}

// SeatpostInsertionBounds returns the insertion interval the frame and post
// allow together. The lower bound is the greater of the two minimums. The upper
// bound is the smaller of the post's maximum and the frame's cap: its declared
// MaxPostInsertionMm, or the seat tube's C-T length minus 20 mm when that is
// zero. An empty interval is an error; equal endpoints permit one installation.
func SeatpostInsertionBounds(frame FrameSetup, post SeatpostSpec) (InsertionBounds, error) {
	if !isFinite(frame.SeatTubeLengthMm) || frame.SeatTubeLengthMm <= 0 {
		return InsertionBounds{}, fmt.Errorf("seat tube length must be finite and positive, got %v mm", frame.SeatTubeLengthMm)
	}
	if !isFinite(frame.MaxPostInsertionMm) || frame.MaxPostInsertionMm < 0 ||
		!isFinite(frame.MinFrameInsertionMm) || frame.MinFrameInsertionMm < 0 {
		return InsertionBounds{}, fmt.Errorf("frame insertion limits must be finite and non-negative, got max %v, min %v mm", frame.MaxPostInsertionMm, frame.MinFrameInsertionMm)
	}
	if err := post.Validate(); err != nil {
		return InsertionBounds{}, err
	}
	frameCap := frame.MaxPostInsertionMm
	if frameCap == 0 {
		frameCap = frame.SeatTubeLengthMm - frameInsertionFallbackMm
	}
	bounds := InsertionBounds{
		EffectiveMinInsertionMm: max(post.MinInsertionMm, frame.MinFrameInsertionMm),
		MaxAllowedInsertionMm:   min(post.MaxInsertionMm, frameCap),
	}
	if bounds.EffectiveMinInsertionMm > bounds.MaxAllowedInsertionMm {
		return InsertionBounds{}, fmt.Errorf("frame and seatpost allow no insertion: minimum %v mm exceeds maximum %v mm", bounds.EffectiveMinInsertionMm, bounds.MaxAllowedInsertionMm)
	}
	return bounds, nil
}

func (p SeatpostSpec) setbackOffset() pose.Vec[SeatTubeAxis] {
	return pose.Vec[SeatTubeAxis]{Y: p.SetbackMm}
}

// SeatpostPlacement holds the Pose-valued landmarks produced by PlaceSeatpost.
// Each landmark carries the leveled seat-tube axis orientation (local +X
// pointing up and rearward along the seat tube axis; local +Y pointing 90°
// counter-clockwise, perpendicular rearward/downward).
type SeatpostPlacement struct {
	SeatTubeTop pose.Pose[SeatTubeAxis, DynamicWorld]
	ShaftTop    pose.Pose[SeatTubeAxis, DynamicWorld]
	RailCradle  pose.Pose[SeatTubeAxis, DynamicWorld]
}

// PlaceSeatpost mounts a seatpost in frame starting at seatTubeTop (positioned
// at the top of the frame's seat tube and oriented up the seat tube axis). The
// exposed length above the seat tube is the post's total length less its
// insertion, which must lie within SeatpostInsertionBounds(frame, setup.Spec).
func PlaceSeatpost(seatTubeTop pose.Pose[SeatTubeAxis, DynamicWorld], frame FrameSetup, setup SeatpostSetup) (SeatpostPlacement, error) {
	if err := validatePose("seatTubeTop", seatTubeTop); err != nil {
		return SeatpostPlacement{}, err
	}
	bounds, err := SeatpostInsertionBounds(frame, setup.Spec)
	if err != nil {
		return SeatpostPlacement{}, err
	}
	if !isFinite(setup.InsertionMm) {
		return SeatpostPlacement{}, invalidInput(InputSeatpostInsertion, "seatpost insertion must be finite, got %v mm", setup.InsertionMm)
	}
	if setup.InsertionMm < bounds.EffectiveMinInsertionMm || setup.InsertionMm > bounds.MaxAllowedInsertionMm {
		return SeatpostPlacement{}, invalidInput(InputSeatpostInsertion, "seatpost insertion must be within %v–%v mm, got %v mm", bounds.EffectiveMinInsertionMm, bounds.MaxAllowedInsertionMm, setup.InsertionMm)
	}

	shaftTop := seatTubeTop.Advance(setup.ExposedMm())
	// Setback is along local +Y from the shaft top, which points rearward.
	railCradle := shaftTop.Translate(setup.Spec.setbackOffset())

	if err := validatePlacementPoints("seatpost", seatTubeTop.Origin(), shaftTop.Origin(), railCradle.Origin()); err != nil {
		return SeatpostPlacement{}, err
	}

	return SeatpostPlacement{
		SeatTubeTop: seatTubeTop,
		ShaftTop:    shaftTop,
		RailCradle:  railCradle,
	}, nil
}

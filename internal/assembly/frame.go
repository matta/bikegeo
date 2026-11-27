package assembly

import (
	"fmt"
	"math"

	"github.com/matta/bikegeo/internal/pose"
)

// frameInsertionCushionMm is subtracted from the seat tube's C-T length to
// leave clearance above the BB shell when a frame declares no maximum
// insertion.
const frameInsertionCushionMm = 20.0

// FrameDimensions specifies the bare frame dimensions in its unpitched
// reference attitude (FrameLocal: BB at (0, 0), +X along chart reach, +Y along
// chart stack):
//   - ReachMm, StackMm: horizontal and vertical offsets from BB to HeadTubeTop (> 0).
//   - HeadTubeAngleDeg, SeatTubeAngleDeg: angles in (0°, 180°).
//   - HeadTubeLengthMm, SeatTubeLengthMm: tube lengths (> 0).
//   - BbDropMm, ChainstayMm: rear-triangle geometry with ChainstayMm > |BbDropMm|.
//   - LowerBore: head tube lower bore standard.
//   - DefaultLowerCup: stock physical lower headset cup fitting LowerBore.
//   - MaxPostInsertionMm: published seatpost insertion cap (0 = undeclared,
//     falling back to SeatTubeLengthMm − 20 mm).
//   - MinFrameInsertionMm: frame minimum seatpost insertion (0 = none).
type FrameDimensions struct {
	ReachMm             float64
	StackMm             float64
	HeadTubeAngleDeg    pose.Deg
	HeadTubeLengthMm    float64
	SeatTubeAngleDeg    pose.Deg
	SeatTubeLengthMm    float64
	BbDropMm            float64
	ChainstayMm         float64
	LowerBore           LowerBore
	DefaultLowerCup     LowerCupSetup
	MaxPostInsertionMm  float64
	MinFrameInsertionMm float64
}

// HeadTubePort is the sealed head-tube mating interface exposed by Frame in
// FrameLocal. SteererLocal ((0, 0) at HeadTubeTop, +X up the steerer) and
// LowerHeadsetLocal ((0, 0) at HeadTubeBottom, +X down the steerer) are
// distinct coordinate systems so upper-steerer and lower-headset transforms
// cannot be swapped.
type HeadTubePort struct {
	steererInFrame      pose.Transform
	lowerHeadsetInFrame pose.Transform
	lowerBore           LowerBore
	defaultLowerCup     LowerCupSetup
	valid               bool
}

func (p HeadTubePort) requireValid() {
	if !p.valid {
		panic("assembly: uninitialized zero-value HeadTubePort")
	}
}

// SteererInFrame returns the transform mapping SteererLocal into FrameLocal.
func (p HeadTubePort) SteererInFrame() pose.Transform {
	p.requireValid()
	return p.steererInFrame
}

// LowerHeadsetInFrame returns the transform mapping LowerHeadsetLocal into FrameLocal.
func (p HeadTubePort) LowerHeadsetInFrame() pose.Transform {
	p.requireValid()
	return p.lowerHeadsetInFrame
}

// LowerBore returns the frame's lower head-tube bore.
func (p HeadTubePort) LowerBore() LowerBore {
	p.requireValid()
	return p.lowerBore
}

// DefaultLowerCup returns the frame's stock physical lower headset cup.
func (p HeadTubePort) DefaultLowerCup() LowerCupSetup {
	p.requireValid()
	return p.defaultLowerCup
}

// Frame is a validated bare bicycle frame in FrameLocal with unexported fields.
// Construct instances with NewFrame.
type Frame struct {
	dims     FrameDimensions
	rearAxle pose.Point
	seatAxis pose.Pose
	headTube HeadTubePort
	seatTube SeatTubeSocket
	valid    bool
}

// NewFrame validates dims and precomputes the frame's rear axle, head-tube
// port, and seat-tube socket in FrameLocal.
func NewFrame(dims FrameDimensions) (Frame, error) {
	if !isFinite(dims.ReachMm) || dims.ReachMm <= 0 {
		return Frame{}, fmt.Errorf("reach must be finite and positive, got %v mm", dims.ReachMm)
	}
	if !isFinite(dims.StackMm) || dims.StackMm <= 0 {
		return Frame{}, fmt.Errorf("stack must be finite and positive, got %v mm", dims.StackMm)
	}
	if !isFinite(float64(dims.HeadTubeAngleDeg)) || dims.HeadTubeAngleDeg <= 0 || dims.HeadTubeAngleDeg >= 180 {
		return Frame{}, fmt.Errorf("head tube angle must be in (0°, 180°), got %v°", dims.HeadTubeAngleDeg)
	}
	if !isFinite(float64(dims.SeatTubeAngleDeg)) || dims.SeatTubeAngleDeg <= 0 || dims.SeatTubeAngleDeg >= 180 {
		return Frame{}, fmt.Errorf("seat tube angle must be in (0°, 180°), got %v°", dims.SeatTubeAngleDeg)
	}
	if !isFinite(dims.HeadTubeLengthMm) || dims.HeadTubeLengthMm <= 0 {
		return Frame{}, fmt.Errorf("head tube length must be finite and positive, got %v mm", dims.HeadTubeLengthMm)
	}
	if !isFinite(dims.BbDropMm) || !isFinite(dims.ChainstayMm) || dims.ChainstayMm <= math.Abs(dims.BbDropMm) {
		return Frame{}, fmt.Errorf("chainstay (%v mm) must exceed |BB drop| (%v mm)", dims.ChainstayMm, dims.BbDropMm)
	}
	if !isFinite(dims.SeatTubeLengthMm) || dims.SeatTubeLengthMm <= 0 {
		return Frame{}, fmt.Errorf("seat tube length must be finite and positive, got %v mm", dims.SeatTubeLengthMm)
	}
	if !isFinite(dims.MaxPostInsertionMm) || dims.MaxPostInsertionMm < 0 ||
		!isFinite(dims.MinFrameInsertionMm) || dims.MinFrameInsertionMm < 0 {
		return Frame{}, fmt.Errorf("frame insertion limits must be finite and non-negative, got max %v, min %v mm", dims.MaxPostInsertionMm, dims.MinFrameInsertionMm)
	}
	if dims.DefaultLowerCup.Standard == LowerCupBareHeadTube {
		return Frame{}, invalidInput(InputLowerCup, "frame default lower cup must be a physical cup, not %s", dims.DefaultLowerCup.Standard)
	}
	if err := CheckSHIS(dims.LowerBore, dims.DefaultLowerCup, SteererBasis); err != nil {
		return Frame{}, err
	}

	frameCap := dims.MaxPostInsertionMm
	maxSource := "frame max insertion"
	if frameCap == 0 {
		frameCap = dims.SeatTubeLengthMm - frameInsertionCushionMm
		maxSource = "frame seat tube clearance"
	}
	allowedInsertion, err := NewBoundedRange(dims.MinFrameInsertionMm, frameCap, "frame min insertion", maxSource)
	if err != nil {
		return Frame{}, fmt.Errorf("frame allows no seatpost insertion: %w", err)
	}

	frameRoot := pose.OriginPose(FrameLocal)
	rearX := -pose.RightTriangleLeg(dims.ChainstayMm, dims.BbDropMm)
	rearAxle := pose.NewPoint(FrameLocal, rearX, dims.BbDropMm)

	headTubeTop := frameRoot.TranslateLocal(dims.ReachMm, dims.StackMm).Turn(180.0 - dims.HeadTubeAngleDeg)
	headTubeBottom := headTubeTop.Turn(180.0).Advance(dims.HeadTubeLengthMm)

	seatAxis := frameRoot.Turn(180.0 - dims.SeatTubeAngleDeg)
	seatTubeTop := seatAxis.Advance(dims.SeatTubeLengthMm)

	if err := validatePlacementPoints("frame", rearAxle, headTubeTop.Origin(), headTubeBottom.Origin(), seatTubeTop.Origin()); err != nil {
		return Frame{}, err
	}

	return Frame{
		dims:     dims,
		rearAxle: rearAxle,
		seatAxis: seatAxis,
		headTube: HeadTubePort{
			steererInFrame:      headTubeTop.Anchor(SteererLocal),
			lowerHeadsetInFrame: headTubeBottom.Anchor(LowerHeadsetLocal),
			lowerBore:           dims.LowerBore,
			defaultLowerCup:     dims.DefaultLowerCup,
			valid:               true,
		},
		seatTube: SeatTubeSocket{
			CollarInFrame:    seatTubeTop.Anchor(SeatTubeCollar),
			AllowedInsertion: allowedInsertion,
		},
		valid: true,
	}, nil
}

func (f Frame) requireValid() {
	if !f.valid {
		panic("assembly: uninitialized zero-value Frame")
	}
}

// RearAxle returns the rear axle position in FrameLocal.
func (f Frame) RearAxle() pose.Point {
	f.requireValid()
	return f.rearAxle
}

// SeatAxis returns the seat-tube axis anchored at BB in FrameLocal.
func (f Frame) SeatAxis() pose.Pose {
	f.requireValid()
	return f.seatAxis
}

// HeadTube returns the sealed HeadTubePort in FrameLocal.
func (f Frame) HeadTube() HeadTubePort {
	f.requireValid()
	return f.headTube
}

// SeatTube returns the frame's SeatTubeSocket in FrameLocal.
func (f Frame) SeatTube() SeatTubeSocket {
	f.requireValid()
	return f.seatTube
}

// LowerBore returns the head tube's lower bore standard.
func (f Frame) LowerBore() LowerBore {
	f.requireValid()
	return f.dims.LowerBore
}

// DefaultLowerCup returns the frame's stock physical lower headset cup.
func (f Frame) DefaultLowerCup() LowerCupSetup {
	f.requireValid()
	return f.dims.DefaultLowerCup
}

// Dimensions returns the validated FrameDimensions used to construct f.
func (f Frame) Dimensions() FrameDimensions {
	f.requireValid()
	return f.dims
}

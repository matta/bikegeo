package assembly

import (
	"fmt"

	"github.com/matta/bikegeo/internal/pose"
)

// frameInsertionCushionMm is subtracted from the seat tube's C-T length to
// leave clearance above the BB shell when a frame declares no maximum
// insertion.
const frameInsertionCushionMm = 20.0

// SeatpostSpec describes a catalog seatpost for the flat *Setup API.
type SeatpostSpec struct {
	TotalLengthMm, MaxInsertionMm, MinInsertionMm float64
	SetbackMm, RailClampLengthMm                  float64
}

// SeatpostSetup defines an installed seatpost for the flat *Setup API.
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

// Validate checks the post's own dimensions via NewRigidPost.
func (p SeatpostSpec) Validate() error {
	_, err := NewRigidPost(p.TotalLengthMm, p.MinInsertionMm, p.MaxInsertionMm, p.SetbackMm, p.RailClampLengthMm)
	return err
}

func seatTubeSocketFromFrameSetup(frame FrameSetup) (SeatTubeSocket, error) {
	if !isFinite(frame.SeatTubeLengthMm) || frame.SeatTubeLengthMm <= 0 {
		return SeatTubeSocket{}, fmt.Errorf("seat tube length must be finite and positive, got %v mm", frame.SeatTubeLengthMm)
	}
	if !isFinite(frame.MaxPostInsertionMm) || frame.MaxPostInsertionMm < 0 ||
		!isFinite(frame.MinFrameInsertionMm) || frame.MinFrameInsertionMm < 0 {
		return SeatTubeSocket{}, fmt.Errorf("frame insertion limits must be finite and non-negative, got max %v, min %v mm", frame.MaxPostInsertionMm, frame.MinFrameInsertionMm)
	}
	frameCap := frame.MaxPostInsertionMm
	maxSource := "frame max insertion"
	if frameCap == 0 {
		frameCap = frame.SeatTubeLengthMm - frameInsertionCushionMm
		maxSource = "frame seat tube clearance"
	}
	allowed, err := NewBoundedRange(frame.MinFrameInsertionMm, frameCap, "frame min insertion", maxSource)
	if err != nil {
		return SeatTubeSocket{}, fmt.Errorf("frame allows no seatpost insertion: %w", err)
	}
	return SeatTubeSocket{
		CollarInFrame:    pose.NewTransform(SeatTubeCollar, FrameLocal, pose.Origin(FrameLocal), 0),
		AllowedInsertion: allowed,
	}, nil
}

// SeatpostInsertionBounds returns the insertion interval the frame and post
// allow together by mating a RigidPost into the frame's SeatTubeSocket.
func SeatpostInsertionBounds(frame FrameSetup, post SeatpostSpec) (InsertionBounds, error) {
	socket, err := seatTubeSocketFromFrameSetup(frame)
	if err != nil {
		return InsertionBounds{}, err
	}
	rigid, err := NewRigidPost(post.TotalLengthMm, post.MinInsertionMm, post.MaxInsertionMm, post.SetbackMm, post.RailClampLengthMm)
	if err != nil {
		return InsertionBounds{}, err
	}
	dof, err := MateSeatpost(socket, rigid)
	if err != nil {
		return InsertionBounds{}, err
	}
	allowed := dof.AllowedInsertion()
	return InsertionBounds{
		EffectiveMinInsertionMm: allowed.Min(),
		MaxAllowedInsertionMm:   allowed.Max(),
	}, nil
}

// PlaceSeatpost mounts a seatpost in frame starting at seatTubeTop (positioned
// at the top of the frame's seat tube and oriented up the seat tube direction)
// by delegating to NewRigidPost, MateSeatpost, Clamp, and PlaceInWorld.
func PlaceSeatpost(seatTubeTop pose.Pose, frame FrameSetup, setup SeatpostSetup) (SeatpostPlacement, error) {
	if err := validatePose("seatTubeTop", seatTubeTop); err != nil {
		return SeatpostPlacement{}, err
	}
	socket, err := seatTubeSocketFromFrameSetup(frame)
	if err != nil {
		return SeatpostPlacement{}, err
	}
	rigid, err := NewRigidPost(setup.Spec.TotalLengthMm, setup.Spec.MinInsertionMm, setup.Spec.MaxInsertionMm, setup.Spec.SetbackMm, setup.Spec.RailClampLengthMm)
	if err != nil {
		return SeatpostPlacement{}, err
	}
	dof, err := MateSeatpost(socket, rigid)
	if err != nil {
		return SeatpostPlacement{}, err
	}
	clamped, err := dof.Clamp(setup.InsertionMm)
	if err != nil {
		return SeatpostPlacement{}, err
	}
	placed := clamped.PlaceInWorld(seatTubeTop.Anchor(FrameLocal), Dynamic)
	if err := validatePlacementPoints("seatpost", seatTubeTop.Origin(), placed.ShaftTop.Origin(), placed.RailPivot.Origin()); err != nil {
		return SeatpostPlacement{}, err
	}
	return placed, nil
}

// Package assembly constructs physical bicycle components and assemblies
// from Pose operations.
package assembly

import (
	"fmt"

	"github.com/matta/bikegeo/internal/bounds"
	"github.com/matta/bikegeo/internal/pose"
)

// DefaultUpperStackMm is the illustrative upper headset bearing and dust cover
// stack above HeadTubeTop. DefaultMaxSpacersMm is the conservative spacer stack
// ceiling every physical build must respect.
const (
	DefaultUpperStackMm = 11.0
	DefaultMaxSpacersMm = 30.0
)

// StemLocal has origin (0, 0) at BarClamp center, +X pointing along the stem
// extension direction away from SteererClamp, and +Y pointing 90° CCW.
var StemLocal = pose.NewCoordinateSystem("part:stem")

// SteererPort is the sealed upper headset and steerer mating port in
// FrameLocal. Its fields are unexported so a port can only be constructed via
// NewSteererPort with a verified SteererLocal → FrameLocal transform, a
// non-negative upper headset stack, and a spacer ceiling at most
// DefaultMaxSpacersMm.
type SteererPort struct {
	steererInFrame pose.Transform
	upperStackMm   float64
	allowedSpacers bounds.BoundedRange
}

// NewSteererPort validates and constructs a SteererPort:
//   - steererInFrame: rigid transform mapping SteererLocal ((0, 0) at
//     HeadTubeTop, +X up the steerer) into FrameLocal. Passing a zero transform
//     or any other coordinate-system pair (such as LowerHeadsetLocal →
//     FrameLocal) panics deterministically.
//   - upperStackMm: axial height of the upper headset bearing and dust cover (>= 0).
//   - maxSpacersMm: maximum spacer stack permitted above the upper cover
//     (0 <= maxSpacersMm <= DefaultMaxSpacersMm).
func NewSteererPort(steererInFrame pose.Transform, upperStackMm, maxSpacersMm float64) (SteererPort, error) {
	if !steererInFrame.Src().Valid() || !steererInFrame.Dst().Valid() {
		panic("assembly: SteererPort steererInFrame has uninitialized CoordinateSystem")
	}
	if steererInFrame.Src() != SteererLocal || steererInFrame.Dst() != FrameLocal {
		panic(fmt.Sprintf("assembly: SteererPort coordinate system mismatch (want %s → %s, got %s → %s)",
			SteererLocal, FrameLocal, steererInFrame.Src(), steererInFrame.Dst()))
	}
	if !isFinitePoint(steererInFrame.OriginInDst()) || !isFinite(float64(steererInFrame.HeadingInDst().Deg())) {
		return SteererPort{}, fmt.Errorf("steerer transform must be finite")
	}
	if !isFinite(upperStackMm) || upperStackMm < 0 {
		return SteererPort{}, fmt.Errorf("upper cover must be finite and non-negative, got %v mm", upperStackMm)
	}
	if !isFinite(maxSpacersMm) || maxSpacersMm < 0 {
		return SteererPort{}, fmt.Errorf("max spacers must be finite and non-negative, got %v mm", maxSpacersMm)
	}
	if maxSpacersMm > DefaultMaxSpacersMm {
		return SteererPort{}, fmt.Errorf("max spacers must be at most %v mm, got %v mm", DefaultMaxSpacersMm, maxSpacersMm)
	}
	allowed, err := bounds.NewBoundedRange(0, maxSpacersMm, "steerer min spacers", "steerer max spacers")
	if err != nil {
		return SteererPort{}, err
	}
	return SteererPort{
		steererInFrame: steererInFrame,
		upperStackMm:   upperStackMm,
		allowedSpacers: allowed,
	}, nil
}

func (p SteererPort) requireValid() {
	if !p.allowedSpacers.Valid() {
		panic("assembly: uninitialized zero-value SteererPort")
	}
}

// SteererInFrame returns the rigid transform mapping SteererLocal into FrameLocal.
func (p SteererPort) SteererInFrame() pose.Transform {
	p.requireValid()
	return p.steererInFrame
}

// UpperStackMm returns the axial stack of the upper headset bearing and dust cover.
func (p SteererPort) UpperStackMm() float64 {
	p.requireValid()
	return p.upperStackMm
}

// AllowedSpacers returns the legal spacer stack interval [0, maxSpacersMm].
func (p SteererPort) AllowedSpacers() bounds.BoundedRange {
	p.requireValid()
	return p.allowedSpacers
}

// BarClampPort is a stem's front mating port where a handlebar attaches.
// ClampInParent maps StemLocal ((0, 0) at BarClamp center, +X pointing along
// the stem extension axis away from SteererClamp) into the owner's coordinate
// system (FrameLocal on a ClampedStem, or a target world on a StemPlacement).
type BarClampPort struct {
	ClampInParent pose.Transform
}

// Stem holds pure physical stem dimensions with unexported fields. Construct
// instances with NewStem.
type Stem struct {
	clampHeightMm float64
	lengthMm      float64
	angleDeg      pose.Deg
	valid         bool
}

// NewStem validates and constructs a Stem:
//   - clampHeightMm: axial height of the steerer clamp (> 0).
//   - lengthMm: extension from steerer axis to bar-clamp center (> 0).
//   - angleDeg: stem angle CCW from perpendicular to the steerer axis (finite).
func NewStem(clampHeightMm, lengthMm float64, angleDeg pose.Deg) (Stem, error) {
	if !isFinite(clampHeightMm) || clampHeightMm <= 0 {
		return Stem{}, fmt.Errorf("clamp height must be finite and positive, got %v mm", clampHeightMm)
	}
	if !isFinite(lengthMm) || lengthMm <= 0 {
		return Stem{}, invalidInput(bounds.InputStemLength, "stem length must be finite and positive, got %v mm", lengthMm)
	}
	if !isFinite(float64(angleDeg)) {
		return Stem{}, invalidInput(bounds.InputStemAngle, "stem angle must be finite, got %v°", angleDeg)
	}
	return Stem{
		clampHeightMm: clampHeightMm,
		lengthMm:      lengthMm,
		angleDeg:      angleDeg,
		valid:         true,
	}, nil
}

func (s Stem) requireValid() {
	if !s.valid {
		panic("assembly: uninitialized zero-value Stem")
	}
}

// ClampHeightMm returns the axial height of the stem's steerer clamp.
func (s Stem) ClampHeightMm() float64 {
	s.requireValid()
	return s.clampHeightMm
}

// LengthMm returns the stem extension from steerer axis to bar-clamp center.
func (s Stem) LengthMm() float64 {
	s.requireValid()
	return s.lengthMm
}

// AngleDeg returns the stem angle CCW from perpendicular to the steerer axis.
func (s Stem) AngleDeg() pose.Deg {
	s.requireValid()
	return s.angleDeg
}

// ClampedStem is a stem mounted onto a SteererPort above a validated stack of
// steerer spacers. Its geometry is locked in FrameLocal and maps rigidly into
// any target world coordinate system through Place.
type ClampedStem struct {
	steerer           SteererPort
	stem              Stem
	spacers           bounds.BoundStatus
	coverTopLocal     pose.Pose
	spacerTopLocal    pose.Pose
	steererClampLocal pose.Pose
	barClampInSteerer pose.Transform
	barClampInFrame   pose.Transform
}

// MountStem binds spacersMm against steerer.AllowedSpacers(), walks in
// SteererLocal along +X through CoverTop, SpacerTop, SteererClamp, turns
// (−90° + stem.AngleDeg()) and advances stem.LengthMm() to BarClamp, and
// composes with steerer.SteererInFrame() into FrameLocal.
func MountStem(steerer SteererPort, spacersMm float64, stem Stem) (ClampedStem, error) {
	steerer.requireValid()
	stem.requireValid()

	status, err := steerer.allowedSpacers.Bind(bounds.InputStemSpacers, spacersMm)
	if err != nil {
		return ClampedStem{}, err
	}

	headTubeTopLocal := pose.OriginPose(SteererLocal)
	coverTopLocal := headTubeTopLocal.Advance(steerer.upperStackMm)
	spacerTopLocal := coverTopLocal.Advance(spacersMm)
	steererClampLocal := spacerTopLocal.Advance(stem.clampHeightMm / 2.0)
	// Turn -90° to the 0° stem datum perpendicular to the steerer, then rotate by the stem angle.
	barClampLocal := steererClampLocal.Turn(-90.0 + stem.angleDeg).Advance(stem.lengthMm)

	if err := validatePlacementPoints("stem", headTubeTopLocal.Origin(), coverTopLocal.Origin(), spacerTopLocal.Origin(), steererClampLocal.Origin(), barClampLocal.Origin()); err != nil {
		return ClampedStem{}, err
	}

	barClampInSteerer := barClampLocal.Anchor(StemLocal)
	barClampInFrame := pose.Compose(barClampInSteerer, steerer.steererInFrame)
	if err := validatePlacementPoints("stem", steerer.steererInFrame.OriginInDst(), barClampInFrame.OriginInDst()); err != nil {
		return ClampedStem{}, err
	}

	return ClampedStem{
		steerer:           steerer,
		stem:              stem,
		spacers:           status,
		coverTopLocal:     coverTopLocal,
		spacerTopLocal:    spacerTopLocal,
		steererClampLocal: steererClampLocal,
		barClampInSteerer: barClampInSteerer,
		barClampInFrame:   barClampInFrame,
	}, nil
}

func (c *ClampedStem) requireValid() {
	if c == nil || !c.stem.valid {
		panic("assembly: uninitialized ClampedStem")
	}
}

// Stem returns the mounted Stem specification.
func (c *ClampedStem) Stem() Stem {
	c.requireValid()
	return c.stem
}

// Spacers returns the bound spacer stack and its provenance status.
func (c *ClampedStem) Spacers() bounds.BoundStatus {
	c.requireValid()
	return c.spacers
}

// UpperStackMm returns the upper headset stack from the mated SteererPort.
func (c *ClampedStem) UpperStackMm() float64 {
	c.requireValid()
	return c.steerer.upperStackMm
}

// BarClampInFrame returns the stem's front BarClampPort mapped into FrameLocal.
func (c *ClampedStem) BarClampInFrame() BarClampPort {
	c.requireValid()
	return BarClampPort{ClampInParent: c.barClampInFrame}
}

// StemPlacement holds the Pose-valued landmarks produced for a mounted stem in
// a target coordinate system. Each landmark carries its local orientation
// anchored at its Origin():
//   - HeadTubeTop, CoverTop, SpacerTop, SteererClamp: oriented along +X up the steerer.
//   - BarClamp: oriented along the stem extension direction away from SteererClamp.
//   - BarClampInWorld: rigid transform mapping StemLocal into the target world
//     coordinate system (omitted from JSON snapshots).
type StemPlacement struct {
	HeadTubeTop     pose.Pose
	CoverTop        pose.Pose
	SpacerTop       pose.Pose
	SteererClamp    pose.Pose
	BarClamp        pose.Pose
	BarClampInWorld pose.Transform `json:"-"`
}

// Place maps the mounted stem's landmarks through frameToWorld
// (FrameLocal → target coordinate system, such as StaticWorld or DynamicWorld).
// It panics deterministically if frameToWorld does not have FrameLocal as its
// source coordinate system.
func (c *ClampedStem) Place(frameToWorld pose.Transform) StemPlacement {
	c.requireValid()
	steererInWorld := pose.Compose(c.steerer.steererInFrame, frameToWorld)
	barClampInWorld := pose.Compose(c.barClampInSteerer, steererInWorld)
	return StemPlacement{
		HeadTubeTop:     steererInWorld.PoseInDst(),
		CoverTop:        steererInWorld.ApplyPose(c.coverTopLocal),
		SpacerTop:       steererInWorld.ApplyPose(c.spacerTopLocal),
		SteererClamp:    steererInWorld.ApplyPose(c.steererClampLocal),
		BarClamp:        barClampInWorld.PoseInDst(),
		BarClampInWorld: barClampInWorld,
	}
}

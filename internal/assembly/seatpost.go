package assembly

import (
	"fmt"

	"github.com/matta/bikegeo/internal/bounds"
	"github.com/matta/bikegeo/internal/pose"
)

// Seatpost and seat-tube mating coordinate systems.
var (
	// SeatTubeCollar has origin (0, 0) at SeatTubeTop (top of the frame's seat
	// tube collar), +X pointing up and rearward along the seat axis, and +Y
	// pointing 90° CCW (perpendicular rearward).
	SeatTubeCollar = pose.NewCoordinateSystem("part:seat_tube_collar")

	// SeatpostLocal has origin (0, 0) at the bottom tip of the seatpost shaft,
	// +X pointing up along the shaft centerline, and +Y pointing 90° CCW
	// (perpendicular rearward).
	SeatpostLocal = pose.NewCoordinateSystem("part:seatpost")

	// SeatpostCradle has origin (0, 0) at the rail-clamp pivot (the transverse
	// axis where saddle rails sit and tilt), +X parallel to the shaft
	// centerline, and +Y pointing 90° CCW (perpendicular rearward).
	SeatpostCradle = pose.NewCoordinateSystem("part:seatpost_cradle")
)

// SeatTubeSocket is the frame's seat-tube mating port in FrameLocal.
// CollarInFrame maps SeatTubeCollar ((0, 0) at SeatTubeTop, +X up the seat
// axis) into FrameLocal, and AllowedInsertion holds the frame's permitted
// insertion interval in millimeters.
type SeatTubeSocket struct {
	CollarInFrame    pose.Transform
	AllowedInsertion bounds.BoundedRange
}

// RailClampPort is a seatpost's upper mating port where a saddle attaches.
// CradleInParent maps SeatpostCradle ((0, 0) at the rail-clamp pivot, +X
// parallel to the shaft centerline) into the owner's coordinate system
// (SeatpostLocal on a standalone post, FrameLocal on a ClampedSeatpost), and
// ClampLengthMm is the fore-aft clamping footprint along the rails.
type RailClampPort struct {
	CradleInParent pose.Transform
	ClampLengthMm  float64
}

func validatePostShaft(totalLengthMm, minInsertionMm, maxInsertionMm, setbackMm, railClampLengthMm float64) (bounds.BoundedRange, error) {
	for _, v := range []float64{totalLengthMm, maxInsertionMm, minInsertionMm} {
		if !isFinite(v) {
			return bounds.BoundedRange{}, fmt.Errorf("seatpost length and insertion limits must be finite, got %v, %v, %v mm", totalLengthMm, maxInsertionMm, minInsertionMm)
		}
	}
	if !(0 < minInsertionMm && minInsertionMm <= maxInsertionMm && maxInsertionMm < totalLengthMm) {
		return bounds.BoundedRange{}, fmt.Errorf("seatpost needs 0 < min insertion <= max insertion < total length, got min %v, max %v, total %v mm", minInsertionMm, maxInsertionMm, totalLengthMm)
	}
	if !isFinite(setbackMm) || setbackMm < 0 {
		return bounds.BoundedRange{}, invalidInput(bounds.InputSeatpostSetback, "seatpost setback must be finite and non-negative, got %v mm", setbackMm)
	}
	if !isFinite(railClampLengthMm) || railClampLengthMm <= 0 {
		return bounds.BoundedRange{}, fmt.Errorf("seatpost clamp length must be finite and positive, got %v mm", railClampLengthMm)
	}
	return bounds.NewBoundedRange(minInsertionMm, maxInsertionMm, "seatpost min insertion", "seatpost max insertion")
}

// RigidPost is a fixed-geometry seatpost whose rail-clamp cradle sits at a
// constant offset in SeatpostLocal across both Static and Dynamic load states.
// Construct instances with NewRigidPost.
type RigidPost struct {
	totalLengthMm     float64
	insertionLimits   bounds.BoundedRange
	setbackMm         float64
	railClampLengthMm float64
}

// NewRigidPost validates and constructs a RigidPost:
//   - totalLengthMm: nominal distance along the shaft from its bottom tip to
//     the rail-clamp pivot projected onto the shaft axis.
//   - minInsertionMm, maxInsertionMm: the post's minimum and maximum insertion
//     marks measured from the shaft bottom (0 < min <= max < total).
//   - setbackMm: perpendicular rearward (+Y in SeatpostLocal) offset from the
//     shaft centerline to the rail-clamp pivot (>= 0).
//   - railClampLengthMm: fore-aft length of the rail clamp along the rails (> 0).
func NewRigidPost(totalLengthMm, minInsertionMm, maxInsertionMm, setbackMm, railClampLengthMm float64) (RigidPost, error) {
	limits, err := validatePostShaft(totalLengthMm, minInsertionMm, maxInsertionMm, setbackMm, railClampLengthMm)
	if err != nil {
		return RigidPost{}, err
	}
	return RigidPost{
		totalLengthMm:     totalLengthMm,
		insertionLimits:   limits,
		setbackMm:         setbackMm,
		railClampLengthMm: railClampLengthMm,
	}, nil
}

// TotalLengthMm returns the nominal shaft length from bottom tip to clamp pivot.
func (p RigidPost) TotalLengthMm() float64 {
	if !p.insertionLimits.Valid() {
		panic("assembly: uninitialized zero-value RigidPost")
	}
	return p.totalLengthMm
}

// InsertionLimits returns the post's own valid insertion interval.
func (p RigidPost) InsertionLimits() bounds.BoundedRange {
	if !p.insertionLimits.Valid() {
		panic("assembly: uninitialized zero-value RigidPost")
	}
	return p.insertionLimits
}

// SetbackMm returns the perpendicular rearward clamp offset from the shaft centerline.
func (p RigidPost) SetbackMm() float64 {
	if !p.insertionLimits.Valid() {
		panic("assembly: uninitialized zero-value RigidPost")
	}
	return p.setbackMm
}

// RailClampLengthMm returns the fore-aft footprint of the rail clamp.
func (p RigidPost) RailClampLengthMm() float64 {
	if !p.insertionLimits.Valid() {
		panic("assembly: uninitialized zero-value RigidPost")
	}
	return p.railClampLengthMm
}

// RailClamp returns the rail-clamp port in SeatpostLocal. A rigid post places
// SeatpostCradle at (TotalLengthMm, SetbackMm) with 0° rotation in both Static
// and Dynamic load states.
func (p RigidPost) RailClamp(state bounds.LoadState) RailClampPort {
	if !p.insertionLimits.Valid() {
		panic("assembly: uninitialized zero-value RigidPost")
	}
	switch state {
	case bounds.Static, bounds.Dynamic:
		return RailClampPort{
			CradleInParent: pose.NewTransform(
				SeatpostCradle,
				SeatpostLocal,
				pose.NewPoint(SeatpostLocal, p.totalLengthMm, p.setbackMm),
				0,
			),
			ClampLengthMm: p.railClampLengthMm,
		}
	default:
		panic(fmt.Sprintf("assembly: invalid LoadState %q", state))
	}
}

// LinkagePost is a parallelogram suspension seatpost whose rail-clamp cradle
// translates by a millimeter sagStep vector in SeatpostLocal when loaded to
// Dynamic while maintaining 0° rotation relative to the shaft.
// Construct instances with NewLinkagePost.
type LinkagePost struct {
	totalLengthMm     float64
	insertionLimits   bounds.BoundedRange
	staticSetbackMm   float64
	railClampLengthMm float64
	sagStep           pose.Vector
}

// NewLinkagePost validates and constructs a LinkagePost. sagStep is a signed
// millimeter displacement vector in SeatpostLocal added to the static cradle
// position (totalLengthMm, staticSetbackMm) in the Dynamic state (for example
// (-6, +5) mm shifts the cradle 6 mm down the shaft and 5 mm rearward).
// Passing a zero-value or wrong-CoordinateSystem sagStep panics deterministically.
func NewLinkagePost(totalLengthMm, minInsertionMm, maxInsertionMm, staticSetbackMm, railClampLengthMm float64, sagStep pose.Vector) (LinkagePost, error) {
	// Assert SeatpostLocal membership before returning any validation error so
	// coordinate-system mistakes fail fast and deterministically.
	staticCradle := pose.NewPoint(SeatpostLocal, totalLengthMm, staticSetbackMm)
	dynamicCradle := staticCradle.Add(sagStep)

	limits, err := validatePostShaft(totalLengthMm, minInsertionMm, maxInsertionMm, staticSetbackMm, railClampLengthMm)
	if err != nil {
		return LinkagePost{}, err
	}
	if !isFinite(sagStep.X()) || !isFinite(sagStep.Y()) {
		return LinkagePost{}, fmt.Errorf("seatpost sag step must be finite, got (%v, %v) mm", sagStep.X(), sagStep.Y())
	}
	if !isFinitePoint(dynamicCradle) || dynamicCradle.X() <= 0 {
		return LinkagePost{}, fmt.Errorf("seatpost dynamic cradle axial position must be finite and positive, got %v mm", dynamicCradle.X())
	}
	return LinkagePost{
		totalLengthMm:     totalLengthMm,
		insertionLimits:   limits,
		staticSetbackMm:   staticSetbackMm,
		railClampLengthMm: railClampLengthMm,
		sagStep:           sagStep,
	}, nil
}

// TotalLengthMm returns the unloaded nominal shaft length from bottom tip to clamp pivot.
func (p LinkagePost) TotalLengthMm() float64 {
	if !p.insertionLimits.Valid() {
		panic("assembly: uninitialized zero-value LinkagePost")
	}
	return p.totalLengthMm
}

// InsertionLimits returns the post's own valid insertion interval.
func (p LinkagePost) InsertionLimits() bounds.BoundedRange {
	if !p.insertionLimits.Valid() {
		panic("assembly: uninitialized zero-value LinkagePost")
	}
	return p.insertionLimits
}

// StaticSetbackMm returns the unloaded perpendicular rearward clamp offset.
func (p LinkagePost) StaticSetbackMm() float64 {
	if !p.insertionLimits.Valid() {
		panic("assembly: uninitialized zero-value LinkagePost")
	}
	return p.staticSetbackMm
}

// SagStep returns the millimeter displacement vector in SeatpostLocal from the
// Static cradle position to the Dynamic cradle position.
func (p LinkagePost) SagStep() pose.Vector {
	if !p.insertionLimits.Valid() {
		panic("assembly: uninitialized zero-value LinkagePost")
	}
	return p.sagStep
}

// RailClampLengthMm returns the fore-aft footprint of the rail clamp.
func (p LinkagePost) RailClampLengthMm() float64 {
	if !p.insertionLimits.Valid() {
		panic("assembly: uninitialized zero-value LinkagePost")
	}
	return p.railClampLengthMm
}

// RailClamp returns the rail-clamp port in SeatpostLocal for state. In Static,
// SeatpostCradle sits at (TotalLengthMm, StaticSetbackMm); in Dynamic, sagStep
// is added with 0° rotation so rail pitch stays constant relative to the shaft.
func (p LinkagePost) RailClamp(state bounds.LoadState) RailClampPort {
	if !p.insertionLimits.Valid() {
		panic("assembly: uninitialized zero-value LinkagePost")
	}
	cradleOrigin := pose.NewPoint(SeatpostLocal, p.totalLengthMm, p.staticSetbackMm)
	switch state {
	case bounds.Static:
	case bounds.Dynamic:
		cradleOrigin = cradleOrigin.Add(p.sagStep)
	default:
		panic(fmt.Sprintf("assembly: invalid LoadState %q", state))
	}
	return RailClampPort{
		CradleInParent: pose.NewTransform(SeatpostCradle, SeatpostLocal, cradleOrigin, 0),
		ClampLengthMm:  p.railClampLengthMm,
	}
}

// postMounter is the thin consumer-side interface MateSeatpost requires from a
// concrete seatpost implementation (RigidPost or LinkagePost).
type postMounter interface {
	TotalLengthMm() float64
	InsertionLimits() bounds.BoundedRange
	RailClamp(state bounds.LoadState) RailClampPort
}

// postSnapshot holds the immutable post geometry captured by value when mating,
// isolating SeatpostDoF and ClampedSeatpost from later mutations if a caller
// passed a pointer to a post value.
type postSnapshot struct {
	totalLengthMm  float64
	minInsertionMm float64
	maxInsertionMm float64
	staticClamp    RailClampPort
	dynamicClamp   RailClampPort
}

func (s postSnapshot) railClamp(state bounds.LoadState) RailClampPort {
	switch state {
	case bounds.Static:
		return s.staticClamp
	case bounds.Dynamic:
		return s.dynamicClamp
	default:
		panic(fmt.Sprintf("assembly: invalid LoadState %q", state))
	}
}

func validateRailClampPort(label string, port RailClampPort) error {
	if !port.CradleInParent.Src().Valid() || !port.CradleInParent.Dst().Valid() {
		panic(fmt.Sprintf("assembly: %s RailClampPort.CradleInParent has uninitialized CoordinateSystem", label))
	}
	if port.CradleInParent.Src() != SeatpostCradle || port.CradleInParent.Dst() != SeatpostLocal {
		panic(fmt.Sprintf("assembly: %s RailClampPort.CradleInParent must map %s → %s, got %s → %s",
			label, SeatpostCradle, SeatpostLocal, port.CradleInParent.Src(), port.CradleInParent.Dst()))
	}
	if !isFinitePoint(port.CradleInParent.OriginInDst()) || !isFinite(float64(port.CradleInParent.HeadingInDst().Deg())) {
		return fmt.Errorf("seatpost %s rail clamp transform must be finite", label)
	}
	if !isFinite(port.ClampLengthMm) || port.ClampLengthMm <= 0 {
		return fmt.Errorf("seatpost %s clamp length must be finite and positive, got %v mm", label, port.ClampLengthMm)
	}
	return nil
}

// SeatpostDoF represents a seatpost mated into a SeatTubeSocket with insertion
// still open over the intersected legal range.
type SeatpostDoF struct {
	socket           SeatTubeSocket
	post             postSnapshot
	allowedInsertion bounds.BoundedRange
}

// MateSeatpost mates post into socket, snapshots the post's shaft length and
// both load-state rail-clamp ports by value, and intersects the post's
// insertion limits with the frame socket's allowed insertion interval. It
// panics deterministically if socket.CollarInFrame does not map
// SeatTubeCollar into FrameLocal or a post clamp port does not map
// SeatpostCradle into SeatpostLocal, and returns an error when the insertion
// intervals do not overlap or the post dimensions are invalid.
func MateSeatpost(socket SeatTubeSocket, post postMounter) (*SeatpostDoF, error) {
	if post == nil {
		return nil, fmt.Errorf("seatpost must not be nil")
	}
	if !socket.CollarInFrame.Src().Valid() || !socket.CollarInFrame.Dst().Valid() {
		panic("assembly: SeatTubeSocket.CollarInFrame has uninitialized CoordinateSystem")
	}
	if socket.CollarInFrame.Src() != SeatTubeCollar || socket.CollarInFrame.Dst() != FrameLocal {
		panic(fmt.Sprintf("assembly: SeatTubeSocket.CollarInFrame must map %s → %s, got %s → %s",
			SeatTubeCollar, FrameLocal, socket.CollarInFrame.Src(), socket.CollarInFrame.Dst()))
	}
	if !socket.AllowedInsertion.Valid() {
		return nil, fmt.Errorf("seat tube socket allowed insertion range is uninitialized")
	}
	postLimits := post.InsertionLimits()
	totalLengthMm := post.TotalLengthMm()
	staticClamp := post.RailClamp(bounds.Static)
	dynamicClamp := post.RailClamp(bounds.Dynamic)

	if err := validateRailClampPort("static", staticClamp); err != nil {
		return nil, err
	}
	if err := validateRailClampPort("dynamic", dynamicClamp); err != nil {
		return nil, err
	}
	if !isFinite(totalLengthMm) || totalLengthMm <= 0 {
		return nil, fmt.Errorf("seatpost total length must be finite and positive, got %v mm", totalLengthMm)
	}
	if !postLimits.Valid() {
		return nil, fmt.Errorf("seatpost insertion limits range is uninitialized")
	}
	if !isFinite(postLimits.Min()) || !isFinite(postLimits.Max()) || postLimits.Min() <= 0 || postLimits.Max() >= totalLengthMm {
		return nil, fmt.Errorf("seatpost needs 0 < min insertion <= max insertion < total length, got min %v, max %v, total %v mm",
			postLimits.Min(), postLimits.Max(), totalLengthMm)
	}
	allowed, ok := bounds.IntersectRanges(postLimits, socket.AllowedInsertion)
	if !ok {
		return nil, fmt.Errorf("frame and seatpost allow no insertion: minimum %v mm exceeds maximum %v mm",
			max(postLimits.Min(), socket.AllowedInsertion.Min()),
			min(postLimits.Max(), socket.AllowedInsertion.Max()))
	}
	return &SeatpostDoF{
		socket: socket,
		post: postSnapshot{
			totalLengthMm:  totalLengthMm,
			minInsertionMm: postLimits.Min(),
			maxInsertionMm: postLimits.Max(),
			staticClamp:    staticClamp,
			dynamicClamp:   dynamicClamp,
		},
		allowedInsertion: allowed,
	}, nil
}

// AllowedInsertion returns the intersected insertion interval permitted by both
// the seatpost and the frame's seat tube socket.
func (d *SeatpostDoF) AllowedInsertion() bounds.BoundedRange {
	return d.allowedInsertion
}

// Clamp locks the seatpost at insertionMm into the seat tube collar. Shaft
// coordinate X = insertionMm coincides with SeatTubeCollar's origin (0, 0), so
// SeatpostLocal sits at (-insertionMm, 0) in SeatTubeCollar.
func (d *SeatpostDoF) Clamp(insertionMm float64) (*ClampedSeatpost, error) {
	status, err := d.allowedInsertion.Bind(bounds.InputSeatpostInsertion, insertionMm)
	if err != nil {
		return nil, err
	}
	postInCollar := pose.NewTransform(
		SeatpostLocal,
		SeatTubeCollar,
		pose.NewPoint(SeatTubeCollar, -insertionMm, 0),
		0,
	)
	postInFrame := pose.Compose(postInCollar, d.socket.CollarInFrame)
	return &ClampedSeatpost{
		socket:       d.socket,
		post:         d.post,
		insertion:    status,
		postInCollar: postInCollar,
		postInFrame:  postInFrame,
	}, nil
}

// ClampedSeatpost is a seatpost locked at a validated insertion depth in its
// SeatTubeSocket. It evaluates its rail-clamp port and world landmarks in any
// LoadState without mutating the clamped installation.
type ClampedSeatpost struct {
	socket       SeatTubeSocket
	post         postSnapshot
	insertion    bounds.BoundStatus
	postInCollar pose.Transform
	postInFrame  pose.Transform
}

// Insertion returns the clamped insertion depth and bound provenance status.
func (c *ClampedSeatpost) Insertion() bounds.BoundStatus {
	return c.insertion
}

// ExposedMm returns the nominal shaft length above SeatTubeTop
// (TotalLengthMm − InsertionMm).
func (c *ClampedSeatpost) ExposedMm() float64 {
	return c.post.totalLengthMm - c.insertion.Value
}

func (c *ClampedSeatpost) setupSnapshot(state bounds.LoadState) SeatpostSetup {
	clamp := c.post.railClamp(state)
	return SeatpostSetup{
		Spec: SeatpostSpec{
			TotalLengthMm:     c.post.totalLengthMm,
			MaxInsertionMm:    c.post.maxInsertionMm,
			MinInsertionMm:    c.post.minInsertionMm,
			SetbackMm:         clamp.CradleInParent.OriginInDst().Y(),
			RailClampLengthMm: clamp.ClampLengthMm,
		},
		InsertionMm: c.insertion.Value,
	}
}

// RailClampInFrame returns the rail-clamp port mapped into FrameLocal for state.
func (c *ClampedSeatpost) RailClampInFrame(state bounds.LoadState) RailClampPort {
	clamp := c.post.railClamp(state)
	// Compose Cradle → SeatpostLocal → SeatTubeCollar first so the axial offset
	// resolves cleanly as (TotalLengthMm - InsertionMm) before rotating into FrameLocal.
	cradleInCollar := pose.Compose(clamp.CradleInParent, c.postInCollar)
	cradleInFrame := pose.Compose(cradleInCollar, c.socket.CollarInFrame)
	return RailClampPort{
		CradleInParent: cradleInFrame,
		ClampLengthMm:  clamp.ClampLengthMm,
	}
}

// SeatpostPlacement holds the Pose-valued landmarks produced for a clamped
// seatpost in a target world coordinate system. Each landmark carries the
// leveled seat-tube orientation anchored at its own Origin() (local +X pointing
// up and rearward along the shaft; local +Y pointing 90° CCW, perpendicular
// rearward):
//   - SeatTubeTop: collar origin on the seat axis where InsertionMm extends
//     downward into the frame and ExposedMm extends upward.
//   - ShaftTop: perpendicular projection of the unloaded clamp pivot onto the
//     shaft axis (ExposedMm above SeatTubeTop along local +X).
//   - RailPivot: rail-clamp pivot where saddle rails attach and tilt for the
//     world's LoadState (StaticWorld → Static, DynamicWorld → Dynamic).
//   - CradleInWorld: rigid transform mapping SeatpostCradle into the target
//     world coordinate system (omitted from JSON snapshots).
type SeatpostPlacement struct {
	SeatTubeTop   pose.Pose
	ShaftTop      pose.Pose
	RailPivot     pose.Pose
	CradleInWorld pose.Transform `json:"-"`
}

// PlaceInWorld maps the clamped seatpost landmarks through frameToWorld
// (FrameLocal → StaticWorld or DynamicWorld), selecting the seatpost's LoadState
// directly from frameToWorld.Dst() so sag state and world coordinate system
// cannot disagree. It panics deterministically if frameToWorld does not map
// FrameLocal into StaticWorld or DynamicWorld.
func (c *ClampedSeatpost) PlaceInWorld(frameToWorld pose.Transform) SeatpostPlacement {
	state, ok := bounds.LoadStateForWorld(frameToWorld.Dst())
	if !ok {
		panic(fmt.Sprintf("assembly: PlaceInWorld requires frameToWorld destination %s or %s, got %s",
			bounds.StaticWorld, bounds.DynamicWorld, frameToWorld.Dst()))
	}
	collarInWorld := pose.Compose(c.socket.CollarInFrame, frameToWorld)
	seatTubeTop := collarInWorld.PoseInDst()
	shaftTop := collarInWorld.ApplyPose(pose.NewPose(pose.NewPoint(SeatTubeCollar, c.ExposedMm(), 0), 0))

	cradleInFrame := c.RailClampInFrame(state).CradleInParent
	cradleInWorld := pose.Compose(cradleInFrame, frameToWorld)
	railPivot := cradleInWorld.PoseInDst()

	return SeatpostPlacement{
		SeatTubeTop:   seatTubeTop,
		ShaftTop:      shaftTop,
		RailPivot:     railPivot,
		CradleInWorld: cradleInWorld,
	}
}

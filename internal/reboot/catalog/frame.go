package catalog

import (
	"bikegeo/internal/reboot/assembly"
	"bikegeo/internal/reboot/pose"
)

// Frame is one frame model with its sizes and the forks it supports.
type Frame struct {
	ID           string
	Manufacturer string
	Model        string
	Year         int
	DefaultSize  string
	Sizes        []FrameSize // declared order (Q69)
	Forks        []FrameFork // declared order (Q69)
	// MaxChainstayMm is the rearmost sliding-dropout position, where / and
	// /bike evaluate the frame (Q67); 0 means fixed dropouts.
	MaxChainstayMm float64
}

// FrameSize is one size's published chart.
type FrameSize struct {
	Name  string
	Chart assembly.FrameChart
}

// Size returns the named size.
func (f Frame) Size(name string) (FrameSize, bool) {
	for _, s := range f.Sizes {
		if s.Name == name {
			return s, true
		}
	}
	return FrameSize{}, false
}

// InstalledFrame returns the size's frame as / and /bike build it: sliding
// dropouts at MaxChainstayMm (Q67), otherwise the published chart frame.
func (f Frame) InstalledFrame(size FrameSize) assembly.FrameSetup {
	frame := size.Chart.Frame
	if f.MaxChainstayMm != 0 {
		frame.ChainstayMm = f.MaxChainstayMm
	}
	return frame
}

// Frames returns the 19 frames in ascending ID order.
func Frames() []Frame {
	return []Frame{
		binaryHavok(),
		binaryKinetik(),
		eskerHayduke(),
		fairlightHolt2(),
		mootsMountaineer(),
		otsoFenrirStainless(),
		revelElJefe(),
		ritcheyUltra(),
		salsaCutthroat(),
		salsaFargo(),
		sevenCustom(),
		sevenExpedition926d(),
		sevenExpedition926f(),
		sklarPbj(),
		sourPastaParty(),
		surlyEcr(),
		surlyKrampus(),
		tumbleweedSunliner(),
		turnerVennTi(),
	}
}

// DefaultFrameID is the default frame for /bike (Q109).
const DefaultFrameID = "otso-fenrir-stainless"

// FrameByID returns the frame with the exact ID.
func FrameByID(id string) (Frame, bool) {
	for _, f := range Frames() {
		if f.ID == id {
			return f, true
		}
	}
	return Frame{}, false
}

// frameFamily holds what every size of a frame shares: its headset, its CAD
// hypotheses (Q44, Q81), and its declared precision (Q80).
type frameFamily struct {
	LowerBore       assembly.LowerBore
	DefaultLowerCup assembly.LowerCupSetup
	CadLowerCup     assembly.LowerCupSetup
	CadFork         assembly.ForkGeometry
	Precision       assembly.ChartPrecision
}

// publishedSize is one size's published chart columns.
type publishedSize struct {
	Name                               string
	ReachMm, StackMm                   float64
	HeadTubeAngleDeg, SeatTubeAngleDeg pose.Deg
	HeadTubeLengthMm, BbDropMm         float64
	ChainstayMm, SeatTubeLengthMm      float64
	WheelbaseMm                        float64 // 0 = unpublished (Q57)
	MaxPostInsertionMm                 float64 // 0 = unpublished (Q95)
	Allowance                          assembly.ChartAllowance
}

// sizes builds each published size's chart on the family's shared values.
func (f frameFamily) sizes(published ...publishedSize) []FrameSize {
	out := make([]FrameSize, len(published))
	for i, p := range published {
		out[i] = FrameSize{
			Name: p.Name,
			Chart: assembly.FrameChart{
				Frame: assembly.FrameSetup{
					ReachMm:             p.ReachMm,
					StackMm:             p.StackMm,
					HeadTubeAngleDeg:    p.HeadTubeAngleDeg,
					HeadTubeLengthMm:    p.HeadTubeLengthMm,
					SeatTubeAngleDeg:    p.SeatTubeAngleDeg,
					SeatTubeLengthMm:    p.SeatTubeLengthMm,
					BbDropMm:            p.BbDropMm,
					ChainstayMm:         p.ChainstayMm,
					LowerBore:           f.LowerBore,
					DefaultLowerCup:     f.DefaultLowerCup,
					MaxPostInsertionMm:  p.MaxPostInsertionMm,
					MinFrameInsertionMm: 0,
				},
				CadLowerCup:          f.CadLowerCup,
				CadFork:              f.CadFork,
				PublishedWheelbaseMm: p.WheelbaseMm,
				Precision:            f.Precision,
				Allowance:            p.Allowance,
			},
		}
	}
	return out
}

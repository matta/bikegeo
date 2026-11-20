package catalog

import "bikegeo/internal/reboot/assembly"

// sevenCollarOffsetMm converts Seven's published C-C seat tube lengths to C-T.
const sevenCollarOffsetMm = 35.0 // illustrative C-C → C-T until Seven confirms (Q75d)

// sevenCustom returns the Seven Custom.
//
// The frame is the rider's custom Seven titanium 29er (CAD preview 2026-11-11).
// Its CAD fork is the ENVE Mountain Boost at 492 mm / 52 mm rake with an 11 mm
// EC44 CAD cup. Stack, reach, and wheelbase come from the 1:5 CAD drawing.
// Seven publishes C-C seat tubes, converted with the illustrative
// sevenCollarOffsetMm (Q75d).
func sevenCustom() Frame {
	family := frameFamily{
		// Q66: inferred from legacy HeadsetCupType EC44; stack is legacy CupStackMm (illustrative)
		LowerBore:       assembly.Bore44Pressed,
		DefaultLowerCup: assembly.LowerCupSetup{Standard: assembly.LowerCupEC44, StackMm: 12},
		// hypothesis (Q44, Q81): legacy GeoHeadsetSpacerMm 11
		CadLowerCup: assembly.LowerCupSetup{Standard: assembly.LowerCupEC44, StackMm: 11},
		// hypothesis (Q44, Q81): legacy enve_mountain_boost (rigid) → 492 mm effective
		CadFork: assembly.ForkGeometry{A2cMm: 492, RakeMm: 52},
		// inferred from published digits (legacy), Q80
		Precision: assembly.ChartPrecision{
			ReachMm:          0.5,
			StackMm:          0.5,
			HeadTubeAngleDeg: 0.05,
			HeadTubeLengthMm: 0.5,
			BbDropMm:         0.5,
			ChainstayMm:      0.5,
			WheelbaseMm:      0.05,
		},
	}
	return Frame{
		ID:           "seven-custom",
		Manufacturer: "Seven",
		Model:        "Custom",
		Year:         2026,
		DefaultSize:  "Custom",
		// catalog: legacy Forks, each at this frame's A2C and rake
		Forks: []FrameFork{
			{Fork: enveMountainBoost, StaticA2cMm: 492, RakeMm: 52},
			{Fork: sid100, StaticA2cMm: 506, RakeMm: 44},
			{Fork: fox34_120, StaticA2cMm: 530, RakeMm: 44},
		},
		// published
		Sizes: family.sizes(
			publishedSize{
				Name:             "Custom",
				ReachMm:          424,
				StackMm:          631,
				HeadTubeAngleDeg: 70,
				SeatTubeAngleDeg: 73,
				HeadTubeLengthMm: 103,
				BbDropMm:         79,
				ChainstayMm:      463,
				SeatTubeLengthMm: 425 + sevenCollarOffsetMm, // published C-C
				WheelbaseMm:      1136.2,
			},
		),
	}
}

package frames

import "bikegeo/pkg/model"

var SevenCustom = model.FrameModel{
	ID:                 "seven-custom",
	Manufacturer:       "Seven",
	Model:              "Custom",
	Year:               2026,
	Category:           model.CategoryHardtail,
	WheelSize:          "29",
	HeadsetCupType:     model.CupEC44,
	GeoHeadsetSpacerMm: 11,
	MaxSpacerMm:        40,
	CanonicalStem:      model.StemSpec{LengthMm: 70, AngleDeg: -6},
	Stems:              []string{"mtb"},
	Sizes:              []string{"Custom"},
	DefaultSize:        "Custom",
	CadFork:            model.ForkRef{Fork: "enve_mountain_boost", A2cMm: 492, RakeMm: 52},
	Forks: []model.ForkRef{
		{Fork: "enve_mountain_boost", A2cMm: 492, RakeMm: 52},
		{Fork: "sid_100", A2cMm: 506, RakeMm: 44},
		{Fork: "fox34_120", A2cMm: 530, RakeMm: 44},
	},
	Notes: "Custom Straight Gauge Titanium 29er mountain frame built by Seven Cycles for Matt Armstrong (Cascade Bicycle Studio, fit technician Neil Doshi, order 2026-10-30, CAD preview 2026-11-11). Configured for 29\" wheels (optimal 2.4\" tire, max 2.6\"), Boost 148mm Asymmetric UDH rear spacing, 73mm BSA bottom bracket, 44mm head tube (EC44 lower cup for tapered steerer), 30.9mm seatpost with internal dropper port, and ENVE Mountain Fork Boost (52mm rake). Stack (631mm), Reach (424mm), and Wheelbase (1136.2mm) are derived from the 1:5 scale CAD frame preview drawing and verified against the published tube lengths (HTL 103mm, ETT 618mm, CS 463mm, ST 425mm C-C), angles (HTA 70.0°, STA 73.0°), BB drop (79mm), standover (814mm), estimated front center (679mm), and cockpit coordinates (HX 481mm, HY 727mm with 70mm -6° stem, 0mm spacer, and 50mm rise handlebar).",
	Geometries: map[string]model.SizeGeometry{
		"Custom": {
			Reach:           424,
			Stack:           631,
			HtaDeg:          70,
			StaDeg:          73,
			Htl:             103,
			BbDrop:          79,
			Wheelbase:       1136.2,
			ChainstayLength: 463,
			SeatTubeLength:  425,
			StandoverHeight: 814,
		},
	},
}


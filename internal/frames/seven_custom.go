package frames

import "bikegeo/internal/model"

var SevenCustom = model.FrameModel{
	ID:                 "seven-custom",
	Manufacturer:       "Seven",
	Model:              "Custom",
	Year:               2026,
	Category:           model.CategoryHardtail,
	WheelSize:          "29",
	HeadsetCupType:     model.CupEC44,
	GeoHeadsetSpacerMm: 11,
	ChartPrecision:     model.ChartPrecision{HtaDeg: 0.05},
	MaxSpacerMm:        40,
	CanonicalStem:      model.StemSpec{LengthMm: 70, AngleDeg: -6},
	Stems:              []string{"mtb"},
	Sizes:              []string{"Custom"},
	DefaultSize:        "Custom",
	CadFork:            model.ForkRef{Fork: "enve_mountain_boost", A2cMm: 492, RakeMm: 52},
	Forks: []model.ForkRef{
		{Fork: "enve_mountain_boost", A2cMm: 492, RakeMm: 52},
		{Fork: "sid_100", TravelMm: 100, A2cMm: 506, RakeMm: 44},
		{Fork: "fox34_120", TravelMm: 120, A2cMm: 530, RakeMm: 44},
	},
	Notes: "Custom Straight Gauge Titanium 29er mountain frame built by Seven Cycles for Matt Armstrong (Cascade Bicycle Studio, fit technician Neil Doshi, order 2026-10-30, CAD preview 2026-11-11). Configured for 29\" wheels (optimal 2.4\" tire, max 2.6\"), Boost 148mm Asymmetric UDH rear spacing, 73mm BSA bottom bracket, 44mm head tube (EC44 lower cup for tapered steerer; GeoHeadsetSpacerMm: 11 models the lower EC44 external headset cup stack in the CAD geometry chart), 30.9mm seatpost with internal dropper port, and ENVE Mountain Fork Boost (52mm rake). Stack (631mm), Reach (424mm), and Wheelbase (1136.2mm) are derived from the 1:5 scale CAD frame preview drawing and verified against the published tube lengths (HTL 103mm, ETT 618mm, CS 463mm, ST 425mm C-C), angles (HTA 70.0°, STA 73.0°), BB drop (79mm), standover (814mm), estimated front center (679mm). Fit provenance: Seven's fit sheet specifies 746mm saddle height, 76mm saddle-nose setback, 245mm saddle length, 37mm saddle-top-to-stem-center drop, and 803mm saddle-tail-to-bar reach. Using the sheet's saddle length horizontally gives X_tail = -321mm and StemClampX = 482mm. The fit-sheet reconstruction puts StemClampY at about 676mm. Matt measured his SQlab at 280mm tip to tail, 35mm longer than the length on the sheet. Using its inferred W70 station and the idmatch BRP with level rails gives modeled X_TIP = -99mm and X_TAIL = -379mm. The discrepancy is unresolved. The idmatch transfer used the same physical saddle and the source reading was taken with the Redshift post unsagged. Note that (481, 726) (700mm from BRP, +13mm above BRP; previously noted as HX 481mm, HY 727mm during AI ingestion) comes from the idmatch report: subtracting 50mm stated bar rise with no backsweep offset yields Seven's stem-center target (~(481, 676)), and a 70mm -6° stem with 0mm spacers and ~11mm upper headset cover places StemClamp at (481.3, 677.1), whereas with the Tumbleweed Persuader 20° bar (-72.8mm backsweep, +45.5mm grip rise) HandGrip sits at (412.3, 712.2) (631.3mm from BRP, matching the rider's Otso Fenrir L with 100mm stem within ~6mm).",
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

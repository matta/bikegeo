package catalog

import (
	"fmt"

	"github.com/matta/bikegeo/internal/assembly"
	"github.com/matta/bikegeo/internal/pose"
)

// Handlebar is one handlebar the workbench can install.
type Handlebar struct {
	ID   string
	Name string
	Part assembly.Handlebar
}

// illustrative: the rider's grips (WingGripInsetMm), the same on every bar
const gripInsetMm = 70.0

func mustHandlebar(dims assembly.HandlebarDimensions) assembly.Handlebar {
	bar, err := assembly.NewHandlebar(dims)
	if err != nil {
		panic(fmt.Sprintf("catalog: invalid handlebar: %v", err))
	}
	return bar
}

// catalogHandlebar builds an uncut handlebar with the rider's grip inset.
func catalogHandlebar(id, name string, widthMm, riseMm float64, mode assembly.RiseMeasurementMode, backsweep, upsweep pose.Deg, bendStartMm float64) Handlebar {
	return Handlebar{
		ID:   id,
		Name: name,
		Part: mustHandlebar(assembly.HandlebarDimensions{
			StockWidthMm:   widthMm,
			CurrentWidthMm: widthMm,
			StatedRiseMm:   riseMm,
			RiseMode:       mode,
			BacksweepDeg:   backsweep,
			UpsweepDeg:     upsweep,
			BendStartMm:    bendStartMm,
			GripInsetMm:    gripInsetMm,
		}),
	}
}

// Handlebars returns the 15 catalog handlebars in declared order.
// catalog: ported from prior catalog data; sources not recorded.
func Handlebars() []Handlebar {
	return []Handlebar{
		catalogHandlebar("persuader20_800", "Tumbleweed Persuader 20° (800 mm)", 800, 50, assembly.RiseModeBarEndCenter, 20, 4, 135),
		catalogHandlebar("persuader20_840", "Tumbleweed Persuader 20° (840 mm)", 840, 50, assembly.RiseModeBarEndCenter, 20, 4, 135),
		// illustrative: SQlab effective bend start = 55 + 0.9 * rise
		catalogHandlebar("sqlab3ox_16_15", "SQlab 3OX 16°, 15 mm rise (780 mm)", 780, 15, assembly.RiseModeStandardKnuckle, 16, 4, 68.5),
		catalogHandlebar("sqlab3ox_16_30", "SQlab 3OX 16°, 30 mm rise (780 mm)", 780, 30, assembly.RiseModeStandardKnuckle, 16, 4, 82.0),
		catalogHandlebar("sqlab3ox_16_45", "SQlab 3OX 16°, 45 mm rise (780 mm)", 780, 45, assembly.RiseModeStandardKnuckle, 16, 4, 95.5),
		catalogHandlebar("sqlab3ox_12_15", "SQlab 3OX 12°, 15 mm rise (780 mm)", 780, 15, assembly.RiseModeStandardKnuckle, 12, 4, 68.5),
		catalogHandlebar("sqlab3ox_12_30", "SQlab 3OX 12°, 30 mm rise (780 mm)", 780, 30, assembly.RiseModeStandardKnuckle, 12, 4, 82.0),
		catalogHandlebar("sqlab3ox_12_45", "SQlab 3OX 12°, 45 mm rise (780 mm)", 780, 45, assembly.RiseModeStandardKnuckle, 12, 4, 95.5),
		catalogHandlebar("sqlab311_20_16_15", "SQlab 311 2.0 Alloy 16°, 15 mm rise (740 mm)", 740, 15, assembly.RiseModeStandardKnuckle, 16, 5, 68.5),
		catalogHandlebar("sqlab311_20_16_25", "SQlab 311 2.0 Alloy 16°, 25 mm rise (740 mm)", 740, 25, assembly.RiseModeStandardKnuckle, 16, 5, 77.5),
		catalogHandlebar("sqlab311_20_16_50", "SQlab 311 2.0 Alloy 16°, 50 mm rise (740 mm)", 740, 50, assembly.RiseModeStandardKnuckle, 16, 5, 100.0),
		catalogHandlebar("sqlab311_flx_16_15", "SQlab 311 FL-X Carbon 16°, 15 mm rise (740 mm)", 740, 15, assembly.RiseModeStandardKnuckle, 16, 4, 68.5),
		catalogHandlebar("sqlab311_flx_16_30", "SQlab 311 FL-X Carbon 16°, 30 mm rise (740 mm)", 740, 30, assembly.RiseModeStandardKnuckle, 16, 4, 82.0),
		catalogHandlebar("sqlab311_flx_12_15", "SQlab 311 FL-X Carbon 12°, 15 mm rise (740 mm)", 740, 15, assembly.RiseModeStandardKnuckle, 12, 4, 68.5),
		catalogHandlebar("sqlab311_flx_12_30", "SQlab 311 FL-X Carbon 12°, 30 mm rise (740 mm)", 740, 30, assembly.RiseModeStandardKnuckle, 12, 4, 82.0),
	}
}

// DefaultHandlebarID is the default handlebar for / and /bike.
const DefaultHandlebarID = "persuader20_800"

// HandlebarByID returns the handlebar with the exact ID.
func HandlebarByID(id string) (Handlebar, bool) {
	for _, b := range Handlebars() {
		if b.ID == id {
			return b, true
		}
	}
	return Handlebar{}, false
}

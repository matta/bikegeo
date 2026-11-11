package catalog

import (
	"strings"

	"bikegeo/pkg/model"
)

// HandlebarPresets maps standard preset keys (matching reference implementations) to HandlebarSpecs.
var HandlebarPresets = map[string]model.HandlebarSpec{
	"persuader20_800": {
		ID:                   "persuader20_800",
		Name:                 "Tumbleweed Persuader 20° (800mm)",
		StockWidthMm:         800.0,
		CurrentWidthMm:       800.0,
		StatedRiseMm:         50.0,
		BacksweepDeg:         20.0,
		UpsweepDeg:           4.0,
		EffectiveBendStartMm: 135.0,
		RiseMode:             model.RiseModeBarEndCenter,
	},
	"persuader20_840": {
		ID:                   "persuader20_840",
		Name:                 "Tumbleweed Persuader 20° (840mm)",
		StockWidthMm:         840.0,
		CurrentWidthMm:       840.0,
		StatedRiseMm:         50.0,
		BacksweepDeg:         20.0,
		UpsweepDeg:           4.0,
		EffectiveBendStartMm: 135.0,
		RiseMode:             model.RiseModeBarEndCenter,
	},
	"sqlab3OX_16_15": {
		ID:                   "sqlab3OX_16_15",
		Name:                 "SQlab 3OX 16° (15mm Rise)",
		StockWidthMm:         780.0,
		CurrentWidthMm:       780.0,
		StatedRiseMm:         15.0,
		BacksweepDeg:         16.0,
		UpsweepDeg:           4.0,
		EffectiveBendStartMm: model.SQlabEffectiveBendStartMm(15.0), // 68.5mm
		RiseMode:             model.RiseModeStandardKnuckle,
	},
	"sqlab3OX_16_30": {
		ID:                   "sqlab3OX_16_30",
		Name:                 "SQlab 3OX 16° (30mm Rise)",
		StockWidthMm:         780.0,
		CurrentWidthMm:       780.0,
		StatedRiseMm:         30.0,
		BacksweepDeg:         16.0,
		UpsweepDeg:           4.0,
		EffectiveBendStartMm: model.SQlabEffectiveBendStartMm(30.0), // 82.0mm
		RiseMode:             model.RiseModeStandardKnuckle,
	},
	"sqlab3OX_16_45": {
		ID:                   "sqlab3OX_16_45",
		Name:                 "SQlab 3OX 16° (45mm Rise)",
		StockWidthMm:         780.0,
		CurrentWidthMm:       780.0,
		StatedRiseMm:         45.0,
		BacksweepDeg:         16.0,
		UpsweepDeg:           4.0,
		EffectiveBendStartMm: model.SQlabEffectiveBendStartMm(45.0), // 95.5mm
		RiseMode:             model.RiseModeStandardKnuckle,
	},
	"sqlab3OX_12_15": {
		ID:                   "sqlab3OX_12_15",
		Name:                 "SQlab 3OX 12° (15mm Rise)",
		StockWidthMm:         780.0,
		CurrentWidthMm:       780.0,
		StatedRiseMm:         15.0,
		BacksweepDeg:         12.0,
		UpsweepDeg:           4.0,
		EffectiveBendStartMm: model.SQlabEffectiveBendStartMm(15.0), // 68.5mm
		RiseMode:             model.RiseModeStandardKnuckle,
	},
	"sqlab3OX_12_30": {
		ID:                   "sqlab3OX_12_30",
		Name:                 "SQlab 3OX 12° (30mm Rise)",
		StockWidthMm:         780.0,
		CurrentWidthMm:       780.0,
		StatedRiseMm:         30.0,
		BacksweepDeg:         12.0,
		UpsweepDeg:           4.0,
		EffectiveBendStartMm: model.SQlabEffectiveBendStartMm(30.0), // 82.0mm
		RiseMode:             model.RiseModeStandardKnuckle,
	},
	"sqlab3OX_12_45": {
		ID:                   "sqlab3OX_12_45",
		Name:                 "SQlab 3OX 12° (45mm Rise)",
		StockWidthMm:         780.0,
		CurrentWidthMm:       780.0,
		StatedRiseMm:         45.0,
		BacksweepDeg:         12.0,
		UpsweepDeg:           4.0,
		EffectiveBendStartMm: model.SQlabEffectiveBendStartMm(45.0), // 95.5mm
		RiseMode:             model.RiseModeStandardKnuckle,
	},
	"sqlab311_20_16_15": {
		ID:                   "sqlab311_20_16_15",
		Name:                 "SQlab 311 2.0 Alloy 16° (15mm Rise)",
		StockWidthMm:         740.0,
		CurrentWidthMm:       740.0,
		StatedRiseMm:         15.0,
		BacksweepDeg:         16.0,
		UpsweepDeg:           5.0,
		EffectiveBendStartMm: model.SQlabEffectiveBendStartMm(15.0), // 68.5mm
		RiseMode:             model.RiseModeStandardKnuckle,
	},
	"sqlab311_20_16_25": {
		ID:                   "sqlab311_20_16_25",
		Name:                 "SQlab 311 2.0 Alloy 16° (25mm Rise)",
		StockWidthMm:         740.0,
		CurrentWidthMm:       740.0,
		StatedRiseMm:         25.0,
		BacksweepDeg:         16.0,
		UpsweepDeg:           5.0,
		EffectiveBendStartMm: model.SQlabEffectiveBendStartMm(25.0), // 77.5mm
		RiseMode:             model.RiseModeStandardKnuckle,
	},
	"sqlab311_20_16_50": {
		ID:                   "sqlab311_20_16_50",
		Name:                 "SQlab 311 2.0 Alloy 16° (50mm Rise)",
		StockWidthMm:         740.0,
		CurrentWidthMm:       740.0,
		StatedRiseMm:         50.0,
		BacksweepDeg:         16.0,
		UpsweepDeg:           5.0,
		EffectiveBendStartMm: model.SQlabEffectiveBendStartMm(50.0), // 100.0mm
		RiseMode:             model.RiseModeStandardKnuckle,
	},
	"sqlab311_FLX_16_15": {
		ID:                   "sqlab311_FLX_16_15",
		Name:                 "SQlab 311 FL-X Carbon 16° (15mm Rise)",
		StockWidthMm:         740.0,
		CurrentWidthMm:       740.0,
		StatedRiseMm:         15.0,
		BacksweepDeg:         16.0,
		UpsweepDeg:           4.0,
		EffectiveBendStartMm: model.SQlabEffectiveBendStartMm(15.0), // 68.5mm
		RiseMode:             model.RiseModeStandardKnuckle,
	},
	"sqlab311_FLX_16_30": {
		ID:                   "sqlab311_FLX_16_30",
		Name:                 "SQlab 311 FL-X Carbon 16° (30mm Rise)",
		StockWidthMm:         740.0,
		CurrentWidthMm:       740.0,
		StatedRiseMm:         30.0,
		BacksweepDeg:         16.0,
		UpsweepDeg:           4.0,
		EffectiveBendStartMm: model.SQlabEffectiveBendStartMm(30.0), // 82.0mm
		RiseMode:             model.RiseModeStandardKnuckle,
	},
	"sqlab311_FLX_12_15": {
		ID:                   "sqlab311_FLX_12_15",
		Name:                 "SQlab 311 FL-X Carbon 12° (15mm Rise)",
		StockWidthMm:         740.0,
		CurrentWidthMm:       740.0,
		StatedRiseMm:         15.0,
		BacksweepDeg:         12.0,
		UpsweepDeg:           4.0,
		EffectiveBendStartMm: model.SQlabEffectiveBendStartMm(15.0), // 68.5mm
		RiseMode:             model.RiseModeStandardKnuckle,
	},
	"sqlab311_FLX_12_30": {
		ID:                   "sqlab311_FLX_12_30",
		Name:                 "SQlab 311 FL-X Carbon 12° (30mm Rise)",
		StockWidthMm:         740.0,
		CurrentWidthMm:       740.0,
		StatedRiseMm:         30.0,
		BacksweepDeg:         12.0,
		UpsweepDeg:           4.0,
		EffectiveBendStartMm: model.SQlabEffectiveBendStartMm(30.0), // 82.0mm
		RiseMode:             model.RiseModeStandardKnuckle,
	},
}

// CommonHandlebarCatalog holds the canonical ordered slice of all handlebars.
var CommonHandlebarCatalog = []model.HandlebarSpec{
	HandlebarPresets["persuader20_800"],
	HandlebarPresets["persuader20_840"],
	HandlebarPresets["sqlab3OX_16_15"],
	HandlebarPresets["sqlab3OX_16_30"],
	HandlebarPresets["sqlab3OX_16_45"],
	HandlebarPresets["sqlab3OX_12_15"],
	HandlebarPresets["sqlab3OX_12_30"],
	HandlebarPresets["sqlab3OX_12_45"],
	HandlebarPresets["sqlab311_20_16_15"],
	HandlebarPresets["sqlab311_20_16_25"],
	HandlebarPresets["sqlab311_20_16_50"],
	HandlebarPresets["sqlab311_FLX_16_15"],
	HandlebarPresets["sqlab311_FLX_16_30"],
	HandlebarPresets["sqlab311_FLX_12_15"],
	HandlebarPresets["sqlab311_FLX_12_30"],
}

// DefaultHandlebar returns the reference baseline handlebar (Tumbleweed Persuader 20° 800mm).
func DefaultHandlebar() model.HandlebarSpec {
	return HandlebarPresets["persuader20_800"]
}

// AllHandlebars returns a copy of all handlebars in the catalog.
func AllHandlebars() []model.HandlebarSpec {
	copied := make([]model.HandlebarSpec, len(CommonHandlebarCatalog))
	copy(copied, CommonHandlebarCatalog)
	return copied
}

// GetHandlebarPreset returns a HandlebarSpec for the given preset key.
func GetHandlebarPreset(presetKey string) (model.HandlebarSpec, bool) {
	norm := normalizeHandlebarKey(presetKey)
	for k, v := range HandlebarPresets {
		if normalizeHandlebarKey(k) == norm {
			return v, true
		}
	}
	return model.HandlebarSpec{}, false
}

// GetHandlebar looks up a handlebar by exact ID, preset key, or case-insensitive substring match.
func GetHandlebar(idOrName string) (model.HandlebarSpec, bool) {
	norm := normalizeHandlebarKey(idOrName)
	if norm == "" {
		return DefaultHandlebar(), true
	}

	// 1. Direct preset key match
	if h, ok := GetHandlebarPreset(idOrName); ok {
		return h, true
	}

	// 2. Normalized ID match in catalog
	for _, h := range CommonHandlebarCatalog {
		if normalizeHandlebarKey(h.ID) == norm {
			return h, true
		}
	}

	// 3. Substring match across ID or Name
	lowerQuery := strings.ToLower(strings.TrimSpace(idOrName))
	for _, h := range CommonHandlebarCatalog {
		if strings.Contains(strings.ToLower(h.ID), lowerQuery) || strings.Contains(strings.ToLower(h.Name), lowerQuery) {
			return h, true
		}
	}

	return model.HandlebarSpec{}, false
}

func normalizeHandlebarKey(k string) string {
	s := strings.ToLower(strings.TrimSpace(k))
	s = strings.ReplaceAll(s, "-", "_")
	s = strings.ReplaceAll(s, " ", "_")
	return s
}


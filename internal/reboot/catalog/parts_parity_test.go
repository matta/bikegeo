package catalog_test

import (
	"strings"
	"testing"

	legacycatalog "bikegeo/internal/catalog"
	"bikegeo/internal/model"
	"bikegeo/internal/reboot/catalog"
)

// TestHandlebarsMatchLegacy checks the exact handlebar port (Q20). The
// workbench supplies its own grip inset and roll separately.
func TestHandlebarsMatchLegacy(t *testing.T) {
	bars := catalog.Handlebars()
	if len(bars) != len(legacycatalog.CommonHandlebarCatalog) {
		t.Fatalf("catalog has %d handlebars, legacy catalog has %d", len(bars), len(legacycatalog.CommonHandlebarCatalog))
	}
	for i, opt := range bars {
		t.Run(opt.ID, func(t *testing.T) {
			if want := strings.ToLower(legacycatalog.CommonHandlebarCatalog[i].ID); opt.ID != want {
				t.Errorf("catalog bar %d = %q, want %q", i, opt.ID, want)
			}
			legacy, ok := legacycatalog.GetHandlebarPreset(opt.ID)
			if !ok {
				t.Fatalf("legacy preset %q missing", opt.ID)
			}
			got := opt.Setup
			for _, field := range []struct {
				name      string
				got, want float64
			}{
				{"StockWidthMm", got.StockWidthMm, legacy.StockWidthMm},
				{"CurrentWidthMm", got.CurrentWidthMm, legacy.CurrentWidthMm},
				{"StatedRiseMm", got.StatedRiseMm, legacy.StatedRiseMm},
				{"BacksweepDeg", float64(got.BacksweepDeg), float64(legacy.BacksweepDeg)},
				{"UpsweepDeg", float64(got.UpsweepDeg), float64(legacy.UpsweepDeg)},
				{"BendStartMm", got.BendStartMm, legacy.EffectiveBendStartMm},
			} {
				if field.got != field.want {
					t.Errorf("%s = %v, want %v", field.name, field.got, field.want)
				}
			}
			if string(got.RiseMode) != string(legacy.RiseMode) {
				t.Errorf("RiseMode = %q, want %q", got.RiseMode, legacy.RiseMode)
			}
		})
	}
}

// TestWheelRadiusMatchesLegacy checks the shared wheel against legacy.
func TestWheelRadiusMatchesLegacy(t *testing.T) {
	if catalog.WheelRadiusMm != model.DefaultWheelRadius29Mm {
		t.Errorf("WheelRadiusMm = %v, legacy DefaultWheelRadius29Mm = %v", catalog.WheelRadiusMm, model.DefaultWheelRadius29Mm)
	}
}

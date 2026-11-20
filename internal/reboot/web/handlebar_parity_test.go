package web

import (
	"strings"
	"testing"

	"bikegeo/internal/catalog"
)

// TestFixtureHandlebarsMatchLegacyCatalog checks the exact catalog port (Q20).
// The workbench supplies its own grip inset and roll separately.
func TestFixtureHandlebarsMatchLegacyCatalog(t *testing.T) {
	bars := otsoFenrirFixture().Handlebars
	if len(bars) != len(catalog.CommonHandlebarCatalog) {
		t.Fatalf("fixture has %d handlebars, legacy catalog has %d", len(bars), len(catalog.CommonHandlebarCatalog))
	}
	for i, opt := range bars {
		t.Run(opt.ID, func(t *testing.T) {
			if want := strings.ToLower(catalog.CommonHandlebarCatalog[i].ID); opt.ID != want {
				t.Errorf("fixture bar %d = %q, want %q", i, opt.ID, want)
			}
			legacy, ok := catalog.GetHandlebarPreset(opt.ID)
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

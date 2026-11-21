package catalog_test

import (
	"testing"

	"bikegeo/internal/reboot/assembly"
	"bikegeo/internal/reboot/catalog"
)

func TestDefaultScoreWeights(t *testing.T) {
	w := catalog.DefaultScoreWeights()
	want := assembly.ScoreWeights{
		HandGrip: 6, SaddleBrpY: 6, SaddleBrpX: 4, Stem: 5, HandlebarSweep: 3, ForkSag: 3,
		Spacers: 2, SaddleRailOffset: 2, RcWbRatio: 2, SeatpostInsertion: 1, LeveledBbDrop: 1, Trail: 1,
	}
	if w != want {
		t.Errorf("weights = %+v, want %+v", w, want)
	}
	if err := w.Validate(); err != nil {
		t.Fatal(err)
	}
	if got := w.ActiveTotal(true); got != 36 {
		t.Errorf("ActiveTotal(true) = %v", got)
	}
	if got := w.ActiveTotal(false); got != 23 {
		t.Errorf("ActiveTotal(false) = %v", got)
	}
}

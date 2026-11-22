package compare

import (
	"slices"
	"testing"

	"bikegeo/internal/assembly"
)

func TestCompareRowsTieBreaks(t *testing.T) {
	shared := assembly.ScoredBuild{Score: assembly.BuildScore{Tier: assembly.FitsAll, Total: 50}}
	better := assembly.ScoredBuild{Score: assembly.BuildScore{Tier: assembly.FitsAll, Total: 60}}

	// Build in expected sorted order, then reverse before sorting so every tie-break is exercised.
	expected := []Row{
		{FrameID: "z", sizeIndex: 9, forkIndex: 9, SagPct: 0, Build: better},
		{FrameID: "a", sizeIndex: 0, forkIndex: 0, SagPct: 20, Build: shared},
		{FrameID: "a", sizeIndex: 0, forkIndex: 0, SagPct: 15, Build: shared},
		{FrameID: "a", sizeIndex: 0, forkIndex: 0, SagPct: 25, Build: shared},
		{FrameID: "a", sizeIndex: 0, forkIndex: 1, SagPct: 20, Build: shared},
		{FrameID: "a", sizeIndex: 1, forkIndex: 0, SagPct: 20, Build: shared},
		{FrameID: "b", sizeIndex: 0, forkIndex: 0, SagPct: 20, Build: shared},
	}

	rows := slices.Clone(expected)
	slices.Reverse(rows)
	slices.SortFunc(rows, compareRows)

	for i := range expected {
		if rows[i].FrameID != expected[i].FrameID ||
			rows[i].sizeIndex != expected[i].sizeIndex ||
			rows[i].forkIndex != expected[i].forkIndex ||
			rows[i].SagPct != expected[i].SagPct ||
			rows[i].Build.Score.Total != expected[i].Build.Score.Total {
			t.Fatalf("row %d = %+v, want %+v", i, rows[i], expected[i])
		}
	}
}

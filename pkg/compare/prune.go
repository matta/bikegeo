package compare

import (
	"math"
	"sort"
	"strings"

	"bikegeo/pkg/canonicalizer"
	"bikegeo/pkg/ideal"
	"bikegeo/pkg/model"
	"bikegeo/pkg/scoring"
)

func PruneRows(
	allFrames []model.FrameModel,
	rows []model.CanonicalComparisonRow,
	prof *ideal.Profile,
	topPerFrame int,
	topLimit int,
) []model.CanonicalComparisonRow {
	if len(rows) <= 1 {
		return rows
	}

	frameMap := make(map[string]model.FrameModel, len(allFrames))
	for _, f := range allFrames {
		frameMap[strings.ToLower(f.ID)] = f
	}

	canonicalizer.ApplyIdealCockpitTargets(rows, prof)

	type scoredCandidate struct {
		row   model.CanonicalComparisonRow
		score scoring.Result
		index int
	}

	candidates := make([]scoredCandidate, len(rows))
	for i, r := range rows {
		candidates[i] = scoredCandidate{
			row:   r,
			score: scoring.ScoreRow(r, prof),
			index: i,
		}
	}

	// Stage 1: For AutoExpanded rows, keep the optimal sag per (FrameID, Size, ForkType)
	var stage1 []scoredCandidate
	type sagGroupKey struct {
		frameID  string
		size     string
		forkType string
	}
	sagGroups := make(map[sagGroupKey][]scoredCandidate)
	var sagGroupOrder []sagGroupKey

	for _, c := range candidates {
		if !c.row.Setup.AutoExpanded {
			stage1 = append(stage1, c)
			continue
		}
		ft := strings.ToLower(canonicalizer.FormatForkType(c.row))
		key := sagGroupKey{
			frameID:  strings.ToLower(c.row.Setup.FrameID),
			size:     strings.ToLower(c.row.Setup.Size),
			forkType: ft,
		}
		if _, exists := sagGroups[key]; !exists {
			sagGroupOrder = append(sagGroupOrder, key)
		}
		sagGroups[key] = append(sagGroups[key], c)
	}

	for _, k := range sagGroupOrder {
		group := sagGroups[k]
		if len(group) == 1 {
			stage1 = append(stage1, group[0])
			continue
		}
		best := group[0]
		for _, cand := range group[1:] {
			if cand.score.Total > best.score.Total+1e-4 {
				best = cand
			} else if math.Abs(cand.score.Total-best.score.Total) <= 1e-4 {
				// Score tie-break:
				// 1. Closest to 20% sag
				distCand := math.Abs(cand.row.Setup.SagPct - 20.0)
				distBest := math.Abs(best.row.Setup.SagPct - 20.0)
				if distCand < distBest-1e-4 {
					best = cand
				} else if math.Abs(distCand-distBest) <= 1e-4 {
					// 2. Lower sag (e.g. 20% before 25%)
					if cand.row.Setup.SagPct < best.row.Setup.SagPct {
						best = cand
					}
				}
			}
		}
		stage1 = append(stage1, best)
	}

	// Stage 2: Top N manifestations per frame
	stage2 := stage1
	if topPerFrame > 0 {
		frameGroups := make(map[string][]scoredCandidate)
		var frameOrder []string
		for _, c := range stage1 {
			fid := strings.ToLower(c.row.Setup.FrameID)
			if _, exists := frameGroups[fid]; !exists {
				frameOrder = append(frameOrder, fid)
			}
			frameGroups[fid] = append(frameGroups[fid], c)
		}

		var filtered []scoredCandidate
		for _, fid := range frameOrder {
			group := frameGroups[fid]
			f := frameMap[fid]
			defSize := f.GetDefaultSize()
			defFork := f.GetDefaultFork()

			sort.SliceStable(group, func(i, j int) bool {
				a, b := group[i], group[j]
				if math.Abs(a.score.Total-b.score.Total) > 1e-4 {
					return a.score.Total > b.score.Total
				}
				// Tie-breaking:
				// 1. Frame's default size
				aIsDefSize := strings.EqualFold(a.row.Setup.Size, defSize)
				bIsDefSize := strings.EqualFold(b.row.Setup.Size, defSize)
				if aIsDefSize != bIsDefSize {
					return aIsDefSize
				}
				// 2. Frame's default fork A2C (within 1mm)
				if defFork.A2cMm > 0 {
					aIsDefFork := math.Abs(a.row.EffectiveA2c-defFork.A2cMm) < 1.0
					bIsDefFork := math.Abs(b.row.EffectiveA2c-defFork.A2cMm) < 1.0
					if aIsDefFork != bIsDefFork {
						return aIsDefFork
					}
				}
				// 3. Cockpit fit: smaller BestR
				if a.row.Canonical.CockpitFitResult != nil && b.row.Canonical.CockpitFitResult != nil {
					diffR := a.row.Canonical.CockpitFitResult.BestR - b.row.Canonical.CockpitFitResult.BestR
					if math.Abs(diffR) > 1e-4 {
						return diffR < 0
					}
				}
				// 4. Rail offset: smaller absolute rail offset
				if a.row.Canonical.SelectedBRP != nil && b.row.Canonical.SelectedBRP != nil {
					diffRail := math.Abs(a.row.Canonical.SelectedBRP.RailOffsetMm) - math.Abs(b.row.Canonical.SelectedBRP.RailOffsetMm)
					if math.Abs(diffRail) > 1e-4 {
						return diffRail < 0
					}
				}
				return a.index < b.index
			})

			limit := min(topPerFrame, len(group))
			filtered = append(filtered, group[:limit]...)
		}
		stage2 = filtered
	}

	// Always sort surviving candidates globally descending by total score
	sort.SliceStable(stage2, func(i, j int) bool {
		if math.Abs(stage2[i].score.Total-stage2[j].score.Total) > 1e-4 {
			return stage2[i].score.Total > stage2[j].score.Total
		}
		return stage2[i].index < stage2[j].index
	})

	// Stage 3: Global Top Limit
	if topLimit > 0 && len(stage2) > topLimit {
		stage2 = stage2[:topLimit]
	}

	result := make([]model.CanonicalComparisonRow, len(stage2))
	for i, c := range stage2 {
		result[i] = c.row
	}
	return result
}

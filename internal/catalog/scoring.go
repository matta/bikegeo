package catalog

import "bikegeo/internal/assembly"

// DefaultScoreWeights returns the Q59a 12-attribute scoring weights (total 36;
// 23 when no BRP target is set).
func DefaultScoreWeights() assembly.ScoreWeights {
	return assembly.ScoreWeights{
		HandGrip:          6,
		SaddleBrpY:        6,
		SaddleBrpX:        4,
		Stem:              5,
		HandlebarSweep:    3,
		ForkSag:           3,
		Spacers:           2,
		SaddleRailOffset:  2,
		RcWbRatio:         2,
		SeatpostInsertion: 1,
		LeveledBbDrop:     1,
		Trail:             1,
	}
}

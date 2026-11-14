package ideal

import (
	"fmt"
	"strings"
)

// ValidateProfile strictly validates an idealized frame profile.
// Returns an error describing the exact attribute and problem if invalid.
func ValidateProfile(p *Profile) error {
	if strings.TrimSpace(p.Name) == "" {
		return fmt.Errorf("profile name must not be empty")
	}

	if sh, ok := p.SaddleHeightMm.Get(); ok && sh <= 0 {
		return fmt.Errorf("profile saddle_height_mm must be positive, got %.1f", sh)
	}

	if len(p.Attributes) == 0 {
		return fmt.Errorf("profile %q must define at least one attribute", p.Name)
	}

	seenIDs := make(map[string]bool)
	hasWeighted := false

	for idx, attr := range p.Attributes {
		id := strings.ToLower(strings.TrimSpace(attr.ID))
		if id == "" {
			return fmt.Errorf("profile %q attribute #%d is missing id", p.Name, idx+1)
		}
		if seenIDs[id] {
			return fmt.Errorf("profile %q contains duplicate attribute id %q", p.Name, attr.ID)
		}
		seenIDs[id] = true

		if _, ok := LookupAttribute(id); !ok {
			return fmt.Errorf("profile %q attribute %q is not a recognized attribute id", p.Name, attr.ID)
		}

		if attr.Weight < 0 {
			return fmt.Errorf("profile %q attribute %q weight must be non-negative, got %.1f", p.Name, attr.ID, attr.Weight)
		}

		if attr.Weight > 0 {
			hasWeighted = true
			if attr.Ideal == nil {
				return fmt.Errorf("profile %q weighted attribute %q must define an ideal range", p.Name, attr.ID)
			}
		}

		if attr.Ideal != nil {
			idealMin, hasIdealMin := attr.Ideal.Min.Get()
			idealMax, hasIdealMax := attr.Ideal.Max.Get()
			if !hasIdealMin && !hasIdealMax {
				return fmt.Errorf("profile %q attribute %q ideal range must define min, max, or both", p.Name, attr.ID)
			}
			if hasIdealMin && hasIdealMax && idealMin > idealMax {
				return fmt.Errorf("profile %q attribute %q ideal min (%.1f) exceeds max (%.1f)", p.Name, attr.ID, idealMin, idealMax)
			}

			// Validate Acceptable range if present
			isWider := false
			if attr.Acceptable != nil {
				accMin, hasAccMin := attr.Acceptable.Min.Get()
				accMax, hasAccMax := attr.Acceptable.Max.Get()
				if !hasAccMin && !hasAccMax {
					return fmt.Errorf("profile %q attribute %q acceptable range must define min, max, or both", p.Name, attr.ID)
				}
				if hasAccMin && hasAccMax && accMin > accMax {
					return fmt.Errorf("profile %q attribute %q acceptable min (%.1f) exceeds max (%.1f)", p.Name, attr.ID, accMin, accMax)
				}

				// One-sided consistency
				if hasIdealMin && !hasAccMin {
					return fmt.Errorf("profile %q attribute %q ideal has min bound but acceptable is unbounded below", p.Name, attr.ID)
				}
				if !hasIdealMin && hasAccMin {
					return fmt.Errorf("profile %q attribute %q ideal is unbounded below but acceptable has min bound", p.Name, attr.ID)
				}
				if hasIdealMax && !hasAccMax {
					return fmt.Errorf("profile %q attribute %q ideal has max bound but acceptable is unbounded above", p.Name, attr.ID)
				}
				if !hasIdealMax && hasAccMax {
					return fmt.Errorf("profile %q attribute %q ideal is unbounded above but acceptable has max bound", p.Name, attr.ID)
				}

				// Subset check
				if hasIdealMin && hasAccMin && accMin > idealMin {
					return fmt.Errorf("profile %q attribute %q acceptable min (%.1f) must be <= ideal min (%.1f)", p.Name, attr.ID, accMin, idealMin)
				}
				if hasIdealMax && hasAccMax && accMax < idealMax {
					return fmt.Errorf("profile %q attribute %q acceptable max (%.1f) must be >= ideal max (%.1f)", p.Name, attr.ID, accMax, idealMax)
				}

				// Check if acceptable is wider than ideal
				if hasIdealMin && hasAccMin && accMin < idealMin {
					isWider = true
				}
				if hasIdealMax && hasAccMax && accMax > idealMax {
					isWider = true
				}
			}

			edge, hasEdge := attr.EdgeScore.Get()
			if isWider {
				if !hasEdge {
					return fmt.Errorf("profile %q attribute %q acceptable range is wider than ideal, but edge_score is not defined", p.Name, attr.ID)
				}
				if edge <= 0 || edge > 100 {
					return fmt.Errorf("profile %q attribute %q edge_score (%.1f) must be in (0, 100]", p.Name, attr.ID, edge)
				}
			} else if hasEdge {
				if edge <= 0 || edge > 100 {
					return fmt.Errorf("profile %q attribute %q edge_score (%.1f) must be in (0, 100]", p.Name, attr.ID, edge)
				}
			}

			// Floor validation
			if fl, ok := attr.Floor.Get(); ok && fl >= 0 {
				return fmt.Errorf("profile %q attribute %q floor (%.1f) must be negative", p.Name, attr.ID, fl)
			}

			// Compile the score curves: rejects missing falloffs and any shape
			// that would be flat, rising or discontinuous.
			if _, err := attr.compileCurves(); err != nil {
				return fmt.Errorf("profile %q %w", p.Name, err)
			}
		}
	}

	if !hasWeighted {
		return fmt.Errorf("profile %q must define at least one weighted attribute", p.Name)
	}

	return nil
}

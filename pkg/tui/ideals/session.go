package ideals

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"bikegeo/pkg/canonicalizer"
	"bikegeo/pkg/ideal"
	"bikegeo/pkg/model"
	"bikegeo/pkg/storage"
	"bikegeo/pkg/validator"
)

// Session manages editing state, deep copies, dirty tracking, and disk persistence for an ideal profile.
type Session struct {
	IdealsDir   string
	DataDir     string
	StatePath   string
	Original    *ideal.Profile
	Working     *ideal.Profile
	OrigJSON    []byte
	FleetRows   []model.CanonicalComparisonRow
	CompareRows []model.CanonicalComparisonRow
}

// NewSession creates an editing session from a loaded Profile.
func NewSession(idealsDir string, p *ideal.Profile) (*Session, error) {
	origJSON, err := json.Marshal(p)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize profile: %w", err)
	}

	var working ideal.Profile
	if err := json.Unmarshal(origJSON, &working); err != nil {
		return nil, fmt.Errorf("failed to clone profile: %w", err)
	}

	return &Session{
		IdealsDir: idealsDir,
		Original:  p,
		Working:   &working,
		OrigJSON:  origJSON,
	}, nil
}

// NewBlankSession creates a new empty profile session.
func NewBlankSession(idealsDir string, name string) (*Session, error) {
	defaultAttr := ideal.AttributeSpec{
		ID:     "aett",
		Weight: 10,
		Ideal: &ideal.Range{
			Min: ptrFloat(600),
			Max: ptrFloat(640),
		},
		Acceptable: &ideal.Range{
			Min: ptrFloat(590),
			Max: ptrFloat(650),
		},
		EdgeScore: ptrFloat(75),
		Falloff: &ideal.Falloff{
			Below: ptrFloat(10),
			Above: ptrFloat(10),
		},
	}

	p := &ideal.Profile{
		Name:        name,
		Description: "Custom ideal geometry profile",
		Attributes:  []ideal.AttributeSpec{defaultAttr},
	}
	return NewSession(idealsDir, p)
}

func ptrFloat(v float64) *float64 {
	return &v
}

// IsDirty checks if the working profile differs from the original.
func (s *Session) IsDirty() bool {
	curJSON, err := json.Marshal(s.Working)
	if err != nil {
		return true
	}
	return !bytes.Equal(s.OrigJSON, curJSON)
}

// Validate checks the working profile against all domain rules.
func (s *Session) Validate() error {
	return ideal.ValidateProfile(s.Working)
}

// Save writes the working profile to disk in IdealsDir/<name>.json.
// Fails if validation fails.
func (s *Session) Save() error {
	if err := s.Validate(); err != nil {
		return fmt.Errorf("cannot save invalid profile: %w", err)
	}

	cleanName := strings.TrimSpace(s.Working.Name)
	if cleanName == "" {
		return fmt.Errorf("profile name cannot be empty")
	}

	if s.IdealsDir != "" {
		if err := os.MkdirAll(s.IdealsDir, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", s.IdealsDir, err)
		}
	}

	filePath := filepath.Join(s.IdealsDir, cleanName+".json")
	data, err := json.MarshalIndent(s.Working, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal profile: %w", err)
	}

	if err := os.WriteFile(filePath, append(data, '\n'), 0644); err != nil {
		return fmt.Errorf("failed to write %s: %w", filePath, err)
	}

	// Update original snapshot
	s.Original = s.Working
	s.OrigJSON, _ = json.Marshal(s.Working)
	return nil
}

// SaveAs saves the profile under a new name and switches the working name.
func (s *Session) SaveAs(newName string) error {
	newName = strings.TrimSpace(newName)
	if newName == "" {
		return fmt.Errorf("new profile name cannot be empty")
	}
	s.Working.Name = newName
	return s.Save()
}

// AddAttribute appends a new attribute to the working profile.
func (s *Session) AddAttribute(attr ideal.AttributeSpec) {
	s.Working.Attributes = append(s.Working.Attributes, attr)
}

// RemoveAttribute removes the attribute at the specified index.
func (s *Session) RemoveAttribute(index int) bool {
	if index < 0 || index >= len(s.Working.Attributes) {
		return false
	}
	s.Working.Attributes = append(s.Working.Attributes[:index], s.Working.Attributes[index+1:]...)
	return true
}

// MoveAttribute moves an attribute from oldIndex to newIndex.
func (s *Session) MoveAttribute(oldIndex, newIndex int) bool {
	n := len(s.Working.Attributes)
	if oldIndex < 0 || oldIndex >= n || newIndex < 0 || newIndex >= n || oldIndex == newIndex {
		return false
	}
	attr := s.Working.Attributes[oldIndex]
	s.Working.Attributes = append(s.Working.Attributes[:oldIndex], s.Working.Attributes[oldIndex+1:]...)
	s.Working.Attributes = append(s.Working.Attributes[:newIndex], append([]ideal.AttributeSpec{attr}, s.Working.Attributes[newIndex:]...)...)
	return true
}

// PopulateBikes loads fleet bikes from dataDir and active comparison bikes from statePath.
func (s *Session) PopulateBikes(bikesDir string, statePath string) {
	if bikesDir != "" {
		s.DataDir = bikesDir
	} else if s.DataDir != "" {
		bikesDir = s.DataDir
	} else {
		if info, err := os.Stat("data/bikes"); err == nil && info.IsDir() {
			bikesDir = "data/bikes"
			s.DataDir = bikesDir
		}
	}

	if statePath != "" {
		s.StatePath = statePath
	} else if s.StatePath != "" {
		statePath = s.StatePath
	} else {
		if _, err := os.Stat("data/last_compare.json"); err == nil {
			statePath = "data/last_compare.json"
			s.StatePath = statePath
		}
	}

	saddleHeight := 780.0
	if s.Working != nil && s.Working.SaddleHeightMm != nil && *s.Working.SaddleHeightMm > 0 {
		saddleHeight = *s.Working.SaddleHeightMm
	}
	rider := model.DefaultRiderProfile()
	rider.SaddleHeightMm = saddleHeight

	// Load fleet bikes across all sizes
	if bikesDir != "" {
		if store, err := storage.NewBikeStore(bikesDir); err == nil {
			if frames, err := store.GetAllFrames(); err == nil {
				s.FleetRows = nil
				for _, f := range frames {
					if !validator.ValidateFrame(f).IsValid {
						continue
					}
					for _, sz := range f.GetSizes() {
						defFork := f.GetDefaultFork()
						setup := model.ComparisonSetup{
							FrameID: f.ID,
							Size:    sz,
							A2cMm:   defFork.A2cMm,
							RakeMm:  defFork.RakeMm,
						}
						if row, err := canonicalizer.ResolveFrameForkSetup(f, setup, rider); err == nil {
							s.FleetRows = append(s.FleetRows, row)
						}
					}
				}
			}
		}
	}

	// Load comparison state if available
	if statePath != "" {
		if data, err := os.ReadFile(statePath); err == nil {
			var saved model.SavedComparisonState
			if err := json.Unmarshal(data, &saved); err == nil && len(saved.Columns) > 0 {
				saved.NormalizeRiderProfile()
				if s.Working != nil && s.Working.SaddleHeightMm != nil && *s.Working.SaddleHeightMm > 0 {
					saved.RiderProfile.SaddleHeightMm = *s.Working.SaddleHeightMm
				}
				if bikesDir != "" {
					if store, err := storage.NewBikeStore(bikesDir); err == nil {
						if frames, err := store.GetAllFrames(); err == nil {
							s.CompareRows = nil
							for _, col := range saved.Columns {
								for i := range frames {
									if frames[i].ID == col.FrameID {
										if row, err := canonicalizer.ResolveFrameForkSetup(frames[i], col, saved.RiderProfile); err == nil {
											s.CompareRows = append(s.CompareRows, row)
										}
										break
									}
								}
							}
						}
					}
				}
			}
		}
	}
}

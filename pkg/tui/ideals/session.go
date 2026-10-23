package ideals

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"bikegeo/pkg/ideal"
)

// Session manages editing state, deep copies, dirty tracking, and disk persistence for an ideal profile.
type Session struct {
	IdealsDir string
	Original  *ideal.Profile
	Working   *ideal.Profile
	OrigJSON  []byte
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

package storage

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"bikegeo/pkg/model"
)

var idSanitizer = regexp.MustCompile(`[^a-z0-9]+`)

type BikeStore struct {
	dataDir string
	mu      sync.RWMutex
}

func NewBikeStore(dataDir string) (*BikeStore, error) {
	if dataDir == "" {
		dataDir = "data/bikes"
	}
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create data dir: %w", err)
	}
	return &BikeStore{dataDir: dataDir}, nil
}

func (s *BikeStore) GetAllBikes() ([]model.BikeGeometry, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	entries, err := os.ReadDir(s.dataDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read data dir: %w", err)
	}

	var bikes []model.BikeGeometry
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		path := filepath.Join(s.dataDir, entry.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var bike model.BikeGeometry
		if err := json.Unmarshal(data, &bike); err != nil {
			continue
		}
		bikes = append(bikes, bike)
	}

	sort.Slice(bikes, func(i, j int) bool {
		strI := fmt.Sprintf("%s %s %s", bikes[i].Manufacturer, bikes[i].Model, bikes[i].Size)
		strJ := fmt.Sprintf("%s %s %s", bikes[j].Manufacturer, bikes[j].Model, bikes[j].Size)
		return strI < strJ
	})

	return bikes, nil
}

func (s *BikeStore) GetBikeByID(id string) (*model.BikeGeometry, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	path := filepath.Join(s.dataDir, fmt.Sprintf("%s.json", id))
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to read bike %s: %w", id, err)
	}

	var bike model.BikeGeometry
	if err := json.Unmarshal(data, &bike); err != nil {
		return nil, fmt.Errorf("failed to parse bike JSON: %w", err)
	}
	return &bike, nil
}

func (s *BikeStore) SaveBike(bike model.BikeGeometry) (*model.BikeGeometry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if bike.ID == "" {
		raw := strings.ToLower(fmt.Sprintf("%s-%s-%s", bike.Manufacturer, bike.Model, bike.Size))
		sanitized := idSanitizer.ReplaceAllString(raw, "-")
		sanitized = strings.Trim(sanitized, "-")
		if sanitized == "" {
			sanitized = fmt.Sprintf("bike-%d", time.Now().UnixMilli())
		}
		bike.ID = sanitized
	}

	path := filepath.Join(s.dataDir, fmt.Sprintf("%s.json", bike.ID))
	data, err := json.MarshalIndent(bike, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to serialize bike: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return nil, fmt.Errorf("failed to write bike file: %w", err)
	}

	return &bike, nil
}

func (s *BikeStore) DeleteBike(id string) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	path := filepath.Join(s.dataDir, fmt.Sprintf("%s.json", id))
	if err := os.Remove(path); err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, fmt.Errorf("failed to delete bike %s: %w", id, err)
	}
	return true, nil
}

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

	"bikegeo/pkg/canonicalizer"
	"bikegeo/pkg/geometry"
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

func parseFrameFromData(data []byte, fileID string) (*model.FrameModel, error) {
	var frame model.FrameModel
	if err := json.Unmarshal(data, &frame); err == nil {
		if len(frame.Configurations) > 0 || len(frame.Sizes) > 0 {
			if frame.ID == "" {
				frame.ID = fileID
			}
			for i := range frame.Configurations {
				frame.Configurations[i].Fork = canonicalizer.ResolveForkSpec(frame.Configurations[i].Fork)
			}
			return &frame, nil
		}
	}

	var bike model.BikeGeometry
	if err := json.Unmarshal(data, &bike); err == nil && (bike.Reach > 0 || bike.Manufacturer != "") {
		size := bike.Size
		if size == "" {
			size = "M"
		}
		frameID := bike.ID
		if frameID == "" {
			frameID = fileID
		}
		baseID := frameID
		if strings.HasSuffix(baseID, "-"+strings.ToLower(size)) {
			baseID = strings.TrimSuffix(baseID, "-"+strings.ToLower(size))
		}

		frame = model.FrameModel{
			ID:             baseID,
			Manufacturer:   bike.Manufacturer,
			Model:          bike.Model,
			Year:           bike.Year,
			Category:       bike.Category,
			Notes:          bike.Notes,
			WheelSize:      bike.WheelSize,
			HeadsetCupType: bike.HeadsetCupType,
			CupStack:       bike.CupStack,
			SeatHeight:     bike.SeatHeight,
			Sizes:          []string{size},
			DefaultSize:    size,
			Configurations: []model.FrameConfiguration{
				{
					ID:         "stock",
					Name:       "Stock Build",
					IsBaseline: true,
					Fork: func() model.ForkSpec {
						if bike.Fork.A2c > 0 || bike.Fork.Rake > 0 {
							return canonicalizer.ResolveForkSpec(bike.Fork)
						}
						return canonicalizer.ResolveForkSpec(model.ForkSpec{
							TravelMm: bike.ForkTravelMm,
							A2c:      bike.ForkA2c,
							Rake:     bike.ForkRake,
						})
					}(),
					SagPercentagePublished: bike.SagPercentagePublished,
					Geometries: map[string]model.SizeGeometry{
						size: {
							Reach:           bike.Reach,
							Stack:           bike.Stack,
							HtaDeg:          bike.HtaDeg,
							StaDeg:          bike.StaDeg,
							Htl:             bike.Htl,
							BbDrop:          bike.BbDrop,
							Wheelbase:       bike.Wheelbase,
							ChainstayLength: bike.ChainstayLength,
							FrontCenter:     bike.FrontCenter,
							SeatTubeLength:  bike.SeatTubeLength,
						},
					},
				},
			},
		}
		return &frame, nil
	}

	return nil, fmt.Errorf("unable to parse frame or bike JSON")
}

func (s *BikeStore) GetAllFrames() ([]model.FrameModel, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	entries, err := os.ReadDir(s.dataDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read data dir: %w", err)
	}

	var frames []model.FrameModel
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		path := filepath.Join(s.dataDir, entry.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		baseFileID := strings.TrimSuffix(entry.Name(), ".json")
		frame, err := parseFrameFromData(data, baseFileID)
		if err != nil {
			continue
		}
		frames = append(frames, *frame)
	}

	sort.Slice(frames, func(i, j int) bool {
		strI := fmt.Sprintf("%s %s", frames[i].Manufacturer, frames[i].Model)
		strJ := fmt.Sprintf("%s %s", frames[j].Manufacturer, frames[j].Model)
		return strI < strJ
	})

	return frames, nil
}

func (s *BikeStore) GetFrameByID(id string) (*model.FrameModel, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	path := filepath.Join(s.dataDir, fmt.Sprintf("%s.json", id))
	data, err := os.ReadFile(path)
	if err == nil {
		frame, parseErr := parseFrameFromData(data, id)
		if parseErr == nil {
			return frame, nil
		}
	}

	// Fallback: search all frames in memory for matching ID or legacy size prefix
	entries, readErr := os.ReadDir(s.dataDir)
	if readErr != nil {
		return nil, readErr
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		p := filepath.Join(s.dataDir, entry.Name())
		d, rErr := os.ReadFile(p)
		if rErr != nil {
			continue
		}
		f, pErr := parseFrameFromData(d, strings.TrimSuffix(entry.Name(), ".json"))
		if pErr != nil {
			continue
		}
		if f.ID == id {
			return f, nil
		}
		// Match against legacy ID or size pattern
		for _, sz := range f.GetSizes() {
			if fmt.Sprintf("%s-%s", f.ID, strings.ToLower(sz)) == id {
				return f, nil
			}
		}
	}

	return nil, nil
}

func (s *BikeStore) SaveFrame(frame model.FrameModel) (*model.FrameModel, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if frame.ID == "" {
		raw := strings.ToLower(fmt.Sprintf("%s-%s", frame.Manufacturer, frame.Model))
		sanitized := idSanitizer.ReplaceAllString(raw, "-")
		sanitized = strings.Trim(sanitized, "-")
		if sanitized == "" {
			sanitized = fmt.Sprintf("frame-%d", time.Now().UnixMilli())
		}
		frame.ID = sanitized
	}

	if len(frame.Sizes) == 0 {
		frame.Sizes = []string{"M"}
	}
	if frame.DefaultSize == "" {
		frame.DefaultSize = frame.Sizes[0]
	}

	path := filepath.Join(s.dataDir, fmt.Sprintf("%s.json", frame.ID))
	data, err := json.MarshalIndent(frame, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to serialize frame: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return nil, fmt.Errorf("failed to write frame file: %w", err)
	}

	return &frame, nil
}

func (s *BikeStore) DeleteFrame(id string) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	path := filepath.Join(s.dataDir, fmt.Sprintf("%s.json", id))
	if err := os.Remove(path); err != nil {
		if os.IsNotExist(err) {
			// Try finding if file had legacy suffix
			entries, _ := os.ReadDir(s.dataDir)
			for _, entry := range entries {
				if strings.TrimSuffix(entry.Name(), ".json") == id {
					_ = os.Remove(filepath.Join(s.dataDir, entry.Name()))
					return true, nil
				}
			}
			return false, nil
		}
		return false, fmt.Errorf("failed to delete frame %s: %w", id, err)
	}
	return true, nil
}

// GetAllBikes returns materialized BikeGeometry items for all frames and their sizes.
func (s *BikeStore) GetAllBikes() ([]model.BikeGeometry, error) {
	frames, err := s.GetAllFrames()
	if err != nil {
		return nil, err
	}

	var bikes []model.BikeGeometry
	for _, frame := range frames {
		sizes := frame.GetSizes()
		baseCfg := frame.GetBaselineConfiguration()
		var baseCfgID string
		if baseCfg != nil {
			baseCfgID = baseCfg.ID
		}
		for _, size := range sizes {
			b, err := geometry.MaterializeBikeGeometry(frame, size, baseCfgID)
			if err == nil {
				bikes = append(bikes, b)
			}
		}
	}

	sort.Slice(bikes, func(i, j int) bool {
		strI := fmt.Sprintf("%s %s %s", bikes[i].Manufacturer, bikes[i].Model, bikes[i].Size)
		strJ := fmt.Sprintf("%s %s %s", bikes[j].Manufacturer, bikes[j].Model, bikes[j].Size)
		return strI < strJ
	})

	return bikes, nil
}

// GetBikeByID resolves a bike geometry by full ID, or frame ID + size.
func (s *BikeStore) GetBikeByID(id string) (*model.BikeGeometry, error) {
	frames, err := s.GetAllFrames()
	if err != nil {
		return nil, err
	}

	for _, frame := range frames {
		baseCfg := frame.GetBaselineConfiguration()
		var baseCfgID string
		if baseCfg != nil {
			baseCfgID = baseCfg.ID
		}

		// Exact frame ID match -> return default size
		if frame.ID == id {
			b, err := geometry.MaterializeBikeGeometry(frame, frame.GetDefaultSize(), baseCfgID)
			if err == nil {
				return &b, nil
			}
		}

		for _, size := range frame.GetSizes() {
			candID := fmt.Sprintf("%s-%s", frame.ID, strings.ToLower(size))
			if candID == id {
				b, err := geometry.MaterializeBikeGeometry(frame, size, baseCfgID)
				if err == nil {
					return &b, nil
				}
			}
			// Check with config ID
			for _, cfg := range frame.Configurations {
				candConfigID := fmt.Sprintf("%s-%s-%s", frame.ID, strings.ToLower(size), cfg.ID)
				if candConfigID == id {
					b, err := geometry.MaterializeBikeGeometry(frame, size, cfg.ID)
					if err == nil {
						return &b, nil
					}
				}
			}
		}
	}

	return nil, nil
}

// SaveBike adapts a single BikeGeometry into a FrameModel persistence write.
func (s *BikeStore) SaveBike(bike model.BikeGeometry) (*model.BikeGeometry, error) {
	size := bike.Size
	if size == "" {
		size = "M"
	}
	if bike.ID == "" {
		raw := strings.ToLower(fmt.Sprintf("%s-%s-%s", bike.Manufacturer, bike.Model, size))
		sanitized := idSanitizer.ReplaceAllString(raw, "-")
		bike.ID = strings.Trim(sanitized, "-")
	}

	baseID := bike.ID
	if baseID != "" && strings.HasSuffix(baseID, "-"+strings.ToLower(size)) {
		baseID = strings.TrimSuffix(baseID, "-"+strings.ToLower(size))
	}

	existing, _ := s.GetFrameByID(baseID)
	var frame model.FrameModel
	if existing != nil {
		frame = *existing
	} else {
		frame = model.FrameModel{
			ID:             baseID,
			Manufacturer:   bike.Manufacturer,
			Model:          bike.Model,
			Year:           bike.Year,
			Category:       bike.Category,
			Notes:          bike.Notes,
			WheelSize:      bike.WheelSize,
			HeadsetCupType: bike.HeadsetCupType,
			CupStack:       bike.CupStack,
			SeatHeight:     bike.SeatHeight,
			Sizes:          []string{size},
			DefaultSize:    size,
		}
	}

	// Ensure size is in frame.Sizes
	hasSize := false
	for _, sz := range frame.Sizes {
		if sz == size {
			hasSize = true
			break
		}
	}
	if !hasSize {
		frame.Sizes = append(frame.Sizes, size)
	}

	sizeGeo := model.SizeGeometry{
		Reach:           bike.Reach,
		Stack:           bike.Stack,
		HtaDeg:          bike.HtaDeg,
		StaDeg:          bike.StaDeg,
		Htl:             bike.Htl,
		BbDrop:          bike.BbDrop,
		Wheelbase:       bike.Wheelbase,
		ChainstayLength: bike.ChainstayLength,
		FrontCenter:     bike.FrontCenter,
		SeatTubeLength:  bike.SeatTubeLength,
	}

	forkSpec := bike.Fork
	if forkSpec.A2c == 0 && bike.ForkA2c > 0 {
		forkSpec.A2c = bike.ForkA2c
		forkSpec.TravelMm = bike.ForkTravelMm
		forkSpec.Rake = bike.ForkRake
	}
	forkSpec = canonicalizer.ResolveForkSpec(forkSpec)

	if len(frame.Configurations) == 0 {
		frame.Configurations = []model.FrameConfiguration{
			{
				ID:                     "stock",
				Name:                   "Stock Build",
				IsBaseline:             true,
				Fork:                   forkSpec,
				SagPercentagePublished: bike.SagPercentagePublished,
				Geometries:             map[string]model.SizeGeometry{size: sizeGeo},
			},
		}
	} else {
		// Update baseline configuration geometry
		baseCfg := frame.GetBaselineConfiguration()
		if baseCfg != nil {
			if baseCfg.Geometries == nil {
				baseCfg.Geometries = make(map[string]model.SizeGeometry)
			}
			baseCfg.Geometries[size] = sizeGeo
		}
	}

	savedFrame, err := s.SaveFrame(frame)
	if err != nil {
		return nil, err
	}

	res, err := geometry.MaterializeBikeGeometry(*savedFrame, size, "")
	if err != nil {
		return nil, err
	}
	return &res, nil
}

// DeleteBike delegates deletion to frame or removes size from frame.
func (s *BikeStore) DeleteBike(id string) (bool, error) {
	// First check if id directly matches a frame
	f, _ := s.GetFrameByID(id)
	if f != nil && f.ID == id {
		return s.DeleteFrame(f.ID)
	}

	// If it's a specific size of a frame, remove that size if multiple exist
	if f != nil {
		for i, sz := range f.Sizes {
			if fmt.Sprintf("%s-%s", f.ID, strings.ToLower(sz)) == id {
				if len(f.Sizes) > 1 {
					f.Sizes = append(f.Sizes[:i], f.Sizes[i+1:]...)
					if f.DefaultSize == sz {
						f.DefaultSize = f.Sizes[0]
					}
					for cfgIdx := range f.Configurations {
						delete(f.Configurations[cfgIdx].Geometries, sz)
					}
					_, err := s.SaveFrame(*f)
					return err == nil, err
				} else {
					return s.DeleteFrame(f.ID)
				}
			}
		}
	}

	return s.DeleteFrame(id)
}

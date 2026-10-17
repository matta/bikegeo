package model

import (
	"encoding/json"
	"fmt"
	"math"
)


type HeadsetCupType string

const (
	CupEC44 HeadsetCupType = "EC44"
	CupZS44 HeadsetCupType = "ZS44"
	CupZS56 HeadsetCupType = "ZS56"
	CupIS42 HeadsetCupType = "IS42"
	CupIS52 HeadsetCupType = "IS52"
)

// CupStackMm returns the physical lower headset cup stack height in mm for a 1.5" tapered fork steerer.
func (c HeadsetCupType) CupStackMm() float64 {
	switch c {
	case CupEC44:
		return 12.0
	case CupZS44, CupZS56:
		return 2.0
	case CupIS42, CupIS52:
		return 0.0
	default:
		return 0.0
	}
}

// IsValid returns true if the HeadsetCupType is empty or a recognized standard.
func (c HeadsetCupType) IsValid() bool {
	switch c {
	case "", CupEC44, CupZS44, CupZS56, CupIS42, CupIS52:
		return true
	default:
		return false
	}
}

type BikeCategory string

const (
	CategoryHardtail       BikeCategory = "hardtail"
	CategoryFullSuspension BikeCategory = "full-suspension"
	CategoryGravel         BikeCategory = "gravel"
	CategoryRigid          BikeCategory = "rigid"
)

type Severity string

const (
	SeverityError   Severity = "error"
	SeverityWarning Severity = "warning"
	SeverityInfo    Severity = "info"
	SeveritySuccess Severity = "success"
)

// BikeGeometry represents bicycle frame geometry data from disk JSON.
type BikeGeometry struct {
	ID           string       `json:"id"`
	Manufacturer string       `json:"manufacturer"`
	Model        string       `json:"model"`
	Size         string       `json:"size"`
	Year         int          `json:"year,omitempty"`
	Category     BikeCategory `json:"category,omitempty"`
	Notes        string       `json:"notes,omitempty"`

	// Primary Frame Dimensions (mm / deg)
	Reach     float64 `json:"reach"`
	Stack     float64 `json:"stack"`
	HtaDeg    float64 `json:"hta_deg"`
	StaDeg    float64 `json:"sta_deg"`
	Htl       float64 `json:"htl"`
	BbDrop    float64 `json:"bb_drop"`
	ForkRake  float64 `json:"fork_rake"`
	Wheelbase float64 `json:"wheelbase"`

	// Optional Frame Dimensions
	ChainstayLength float64 `json:"chainstay_length,omitempty"`
	FrontCenter     float64 `json:"front_center,omitempty"`
	SeatTubeLength  float64 `json:"seat_tube_length,omitempty"`
	WheelSize       string  `json:"wheel_size,omitempty"`

	// Published CAD & Hardware Assumptions
	Fork                   ForkSpec       `json:"fork"`
	ForkTravelMm           float64        `json:"fork_travel_mm,omitempty"`
	ForkA2c                float64        `json:"fork_a2c,omitempty"`
	SagPercentagePublished float64        `json:"sag_percentage_published,omitempty"`
	HeadsetCupType         HeadsetCupType `json:"headset_cup_type,omitempty"`

	// Rider Fit Reference
	SeatHeight float64 `json:"seat_height,omitempty"`
}

// ForkSpec encapsulates physical and geometric properties of a fork.
type ForkSpec struct {
	ID                 string  `json:"id,omitempty"`
	Name               string  `json:"name,omitempty"`
	TravelMm           float64 `json:"travel_mm"`
	A2c                float64 `json:"a2c"`
	Rake               float64 `json:"rake"`
	StandardSagPercent float64 `json:"standard_sag_percent,omitempty"`
	Description        string  `json:"description,omitempty"`
	IsInferred         bool    `json:"is_inferred,omitempty"`
}

// SizeGeometry represents primary and optional frame dimensions for one size.
type SizeGeometry struct {
	Reach           float64 `json:"reach"`
	Stack           float64 `json:"stack"`
	HtaDeg          float64 `json:"hta_deg"`
	StaDeg          float64 `json:"sta_deg"`
	Htl             float64 `json:"htl"`
	BbDrop          float64 `json:"bb_drop"`
	Wheelbase       float64 `json:"wheelbase"`
	ChainstayLength float64 `json:"chainstay_length,omitempty"`
	FrontCenter     float64 `json:"front_center,omitempty"`
	SeatTubeLength  float64 `json:"seat_tube_length,omitempty"`
	StandoverHeight float64 `json:"standover_height,omitempty"`
}

// FrameConfiguration represents a specific fork or build configuration for the frame.
type FrameConfiguration struct {
	ID                     string                  `json:"id"`
	Name                   string                  `json:"name"`
	IsBaseline             bool                    `json:"is_baseline"`
	Fork                   ForkSpec                `json:"fork"`
	SagPercentagePublished float64                 `json:"sag_percentage_published"`
	Notes                  string                  `json:"notes,omitempty"`
	Geometries             map[string]SizeGeometry `json:"geometries,omitempty"`
}

// FrameModel represents a unified bike frame family on disk and in memory.
type FrameModel struct {
	ID                string               `json:"id"`
	Manufacturer      string               `json:"manufacturer"`
	Model             string               `json:"model"`
	Year              int                  `json:"year,omitempty"`
	Category          BikeCategory         `json:"category,omitempty"`
	Notes             string               `json:"notes,omitempty"`
	WheelSize         string               `json:"wheel_size,omitempty"`
	HeadsetCupType    HeadsetCupType       `json:"headset_cup_type,omitempty"`
	SeatHeight        float64              `json:"seat_height,omitempty"`
	SupportedTravelMm []float64            `json:"supported_travel_mm,omitempty"`
	RigidA2cMm        float64              `json:"rigid_a2c_mm,omitempty"`
	DefaultFork       string               `json:"default_fork,omitempty"`
	Sizes             []string             `json:"sizes"`
	DefaultSize       string               `json:"default_size,omitempty"`
	Configurations    []FrameConfiguration `json:"configurations"`
}

func (fm *FrameModel) GetSizes() []string {
	if len(fm.Sizes) > 0 {
		return fm.Sizes
	}
	return []string{"M"}
}

func (fm *FrameModel) GetDefaultSize() string {
	if fm.DefaultSize != "" {
		return fm.DefaultSize
	}
	if len(fm.Sizes) > 0 {
		return fm.Sizes[0]
	}
	return "M"
}

func (fm *FrameModel) GetBaselineConfiguration() *FrameConfiguration {
	for i := range fm.Configurations {
		if fm.Configurations[i].IsBaseline {
			return &fm.Configurations[i]
		}
	}
	if len(fm.Configurations) > 0 {
		return &fm.Configurations[0]
	}
	return nil
}

func (fm *FrameModel) GetConfiguration(configID string) *FrameConfiguration {
	for i := range fm.Configurations {
		if fm.Configurations[i].ID == configID {
			return &fm.Configurations[i]
		}
	}
	return fm.GetBaselineConfiguration()
}

// GetSupportedTravels returns the list of supported suspension travel sizes in mm.
// If SupportedTravelMm is explicitly defined on the frame, it is returned.
// Otherwise, it is inferred from Configurations with TravelMm > 0.
func (fm *FrameModel) GetSupportedTravels() []float64 {
	if len(fm.SupportedTravelMm) > 0 {
		out := make([]float64, len(fm.SupportedTravelMm))
		copy(out, fm.SupportedTravelMm)
		return out
	}
	var inferred []float64
	for _, cfg := range fm.Configurations {
		if cfg.Fork.TravelMm > 0 {
			exists := false
			for _, t := range inferred {
				if math.Abs(t-cfg.Fork.TravelMm) < 0.1 {
					exists = true
					break
				}
			}
			if !exists {
				inferred = append(inferred, cfg.Fork.TravelMm)
			}
		}
	}
	return inferred
}

// GetRigidA2c returns the supported rigid fork axle-to-crown in mm (or 0 if rigid is not supported).
func (fm *FrameModel) GetRigidA2c() float64 {
	if fm.RigidA2cMm > 0 {
		return fm.RigidA2cMm
	}
	for _, cfg := range fm.Configurations {
		if cfg.Fork.TravelMm == 0 && cfg.Fork.A2c > 0 {
			return cfg.Fork.A2c
		}
	}
	return 0
}

// SupportsRigid returns true if the frame supports a rigid fork.
func (fm *FrameModel) SupportsRigid() bool {
	return fm.GetRigidA2c() > 0
}

// SupportsTravel returns true if the frame supports the given suspension travel in mm.
func (fm *FrameModel) SupportsTravel(travelMm float64) bool {
	if travelMm <= 0 {
		return false
	}
	for _, t := range fm.GetSupportedTravels() {
		if math.Abs(t-travelMm) < 0.1 {
			return true
		}
	}
	return false
}

// GetSupportedForkOptions returns human-readable strings for all legal fork choices on this frame.
func (fm *FrameModel) GetSupportedForkOptions() []string {
	var opts []string
	for _, t := range fm.GetSupportedTravels() {
		opts = append(opts, fmt.Sprintf("%.0fmm", t))
	}
	if a2c := fm.GetRigidA2c(); a2c > 0 {
		opts = append(opts, fmt.Sprintf("rigid (%.0fmm)", a2c))
	}
	return opts
}

// GetDefaultForkSpecString returns the default fork token (e.g. "120mm" or "rigid") for this frame.
func (fm *FrameModel) GetDefaultForkSpecString() string {
	if fm.DefaultFork != "" {
		return fm.DefaultFork
	}
	base := fm.GetBaselineConfiguration()
	if base != nil {
		if base.Fork.TravelMm > 0 {
			return fmt.Sprintf("%.0fmm", base.Fork.TravelMm)
		}
		if base.Fork.A2c > 0 {
			return "rigid"
		}
	}
	travels := fm.GetSupportedTravels()
	if len(travels) > 0 {
		return fmt.Sprintf("%.0fmm", travels[0])
	}
	if fm.SupportsRigid() {
		return "rigid"
	}
	return "120mm"
}

// IsFixedFork returns true if the frame only supports a single fork capacity/type.
func (fm *FrameModel) IsFixedFork() bool {
	count := len(fm.GetSupportedTravels())
	if fm.SupportsRigid() {
		count++
	}
	return count <= 1
}

// IsFixedRigidOnly returns true if the frame only supports a rigid fork and no suspension forks.
func (fm *FrameModel) IsFixedRigidOnly() bool {
	return len(fm.GetSupportedTravels()) == 0 && fm.SupportsRigid()
}

type CrossConfigDiff struct {
	Metric        string  `json:"metric"`
	BaselineValue float64 `json:"baseline_value"`
	ConfigValue   float64 `json:"config_value"`
	PublishedDiff float64 `json:"published_diff"`
	KinematicDiff float64 `json:"kinematic_diff"`
	Discrepancy   float64 `json:"discrepancy"`
}

type CrossConfigValidation struct {
	FrameModelID     string            `json:"frame_model_id"`
	Size             string            `json:"size"`
	BaselineConfigID string            `json:"baseline_config_id"`
	ComparedConfigID string            `json:"compared_config_id"`
	BaselineA2c      float64           `json:"baseline_a2c"`
	ComparedA2c      float64           `json:"compared_a2c"`
	DeltaA2c         float64           `json:"delta_a2c"`
	IsCoherent       bool              `json:"is_coherent"`
	MaxDiscrepancy   float64           `json:"max_discrepancy"`
	Diffs            []CrossConfigDiff `json:"diffs"`
	Message          string            `json:"message"`
}


type ValidationIssue struct {
	ID         string   `json:"id"`
	Field      string   `json:"field,omitempty"`
	Severity   Severity `json:"severity"`
	Title      string   `json:"title"`
	Message    string   `json:"message"`
	Details    string   `json:"details,omitempty"`
	Expected   string   `json:"expected,omitempty"`
	Actual     string   `json:"actual,omitempty"`
	Difference float64  `json:"difference,omitempty"`
}

type PublishMode string

const (
	ModeStaticCad     PublishMode = "static_cad"
	ModeCupOmittedCad PublishMode = "cup_omitted_cad"
	ModeSaggedCad     PublishMode = "sagged_cad_15_25"
	ModeMismatchedCad PublishMode = "mismatched_cad"
	ModeUnspecified   PublishMode = "unspecified"
)

type ValidationReport struct {
	BikeID              string            `json:"bikeId"`
	IsValid             bool              `json:"isValid"`
	Issues              []ValidationIssue `json:"issues"`
	GeometricA2c        float64           `json:"geometricA2c"`
	StatedA2c           float64           `json:"statedA2c,omitempty"`
	ExpectedForkA2c     float64           `json:"expectedForkA2c,omitempty"`
	CupStack            float64           `json:"cupStack"`
	TotalStructuralA2c  float64           `json:"totalStructuralA2c"`
	A2cDifference       *float64          `json:"a2cDifference,omitempty"`
	InferredPublishMode PublishMode       `json:"inferredPublishMode"`
	FrontCenterCalc     float64           `json:"frontCenterCalc,omitempty"`
	FrontCenterDiff     *float64          `json:"frontCenterDiff,omitempty"`
}

// FrameValidationReport holds the complete validation result across all sizes and configurations of a FrameModel.
type FrameValidationReport struct {
	FrameID      string                      `json:"frame_id"`
	IsValid      bool                        `json:"is_valid"`
	Issues       []ValidationIssue           `json:"issues,omitempty"`
	SizeReports  map[string]ValidationReport `json:"size_reports,omitempty"`
	CrossConfigs []CrossConfigValidation     `json:"cross_configs,omitempty"`
}

// ErrorSummary returns a human-readable summary of all validation errors on the frame.
func (r FrameValidationReport) ErrorSummary() string {
	if r.IsValid {
		return ""
	}
	var msgs []string
	for _, iss := range r.Issues {
		if iss.Severity == SeverityError {
			msgs = append(msgs, fmt.Sprintf("%s: %s", iss.Title, iss.Message))
		}
	}
	for sz, rep := range r.SizeReports {
		for _, iss := range rep.Issues {
			if iss.Severity == SeverityError {
				msgs = append(msgs, fmt.Sprintf("[Size %s] %s: %s", sz, iss.Title, iss.Message))
			}
		}
	}
	for _, cc := range r.CrossConfigs {
		if !cc.IsCoherent {
			msgs = append(msgs, fmt.Sprintf("[Cross-Config %s vs %s (%s)] %s", cc.BaselineConfigID, cc.ComparedConfigID, cc.Size, cc.Message))
		}
	}
	if len(msgs) == 0 {
		return fmt.Sprintf("frame %q is in an invalid state", r.FrameID)
	}
	return fmt.Sprintf("frame %q failed validation: %s", r.FrameID, msgs[0])
}


type CanonicalConfig struct {
	TargetA2c  float64 `json:"targetA2c"`
	SeatHeight float64 `json:"seatHeight"`
	SagPercent float64 `json:"sagPercent"`
	Name       string  `json:"name,omitempty"`
}

// ComparisonSetup represents one column in a comparison matrix.
type ComparisonSetup struct {
	FrameID    string   `json:"frame_id"`
	Size       string   `json:"size"`
	Fork       string   `json:"fork"`                  // e.g. "120mm", "130mm", "rigid"
	SagPercent *float64 `json:"sag_percent,omitempty"` // nil -> defaults to 20% for suspension, 0% for rigid
}

// SavedComparisonState persists the user's most recent comparison in data/last_compare.json.
type SavedComparisonState struct {
	SeatHeight float64           `json:"seat_height"`
	Transpose  bool              `json:"transpose"`
	RefIndex   int               `json:"ref_index"`
	Columns    []ComparisonSetup `json:"columns"`
}

type DynamicGeometryResult struct {
	NewHta        float64 `json:"new_hta"`
	NewSta        float64 `json:"new_sta"`
	NewReach      float64 `json:"new_reach"`
	NewStack      float64 `json:"new_stack"`
	NewBbDrop     float64 `json:"new_bb_drop"`
	DeltaAngleDeg float64 `json:"delta_angle_deg"`
	DeltaReach    float64 `json:"delta_reach"`
	DeltaStack    float64 `json:"delta_stack"`
	DeltaBbDrop   float64 `json:"delta_bb_drop"`
	TotalNewA2c   float64 `json:"total_new_a2c"`
}

type BikeFitAnalysis struct {
	StructuralA2c       float64 `json:"structural_a2c"`
	ActiveReach         float64 `json:"active_reach"`
	ActiveStack         float64 `json:"active_stack"`
	ActiveHta           float64 `json:"active_hta"`
	ActiveSta           float64 `json:"active_sta"`
	ActiveBbDrop        float64 `json:"active_bb_drop"`
	Aett                float64 `json:"aett"`
	SpareVerticalStack  float64 `json:"spare_vertical_stack"`
	SpareAxialSteerer   float64 `json:"spare_axial_steerer"`
	DeltaAngleDeg       float64 `json:"delta_angle_deg"`
	DeltaReach          float64 `json:"delta_reach"`
	DeltaStack          float64 `json:"delta_stack"`
	DeltaBbDrop         float64 `json:"delta_bb_drop"`
	InferredPublishMode string  `json:"inferredPublishMode"`
}

type BaselineMetrics struct {
	Reach              float64 `json:"reach"`
	Stack              float64 `json:"stack"`
	HtaDeg             float64 `json:"hta_deg"`
	StaDeg             float64 `json:"sta_deg"`
	BbDrop             float64 `json:"bb_drop"`
	A2c                float64 `json:"a2c"`
	Aett               float64 `json:"aett"`
	SpareVerticalStack float64 `json:"spare_vertical_stack"`
	SpareAxialSteerer  float64 `json:"spare_axial_steerer"`
}

type CanonicalComparisonRow struct {
	Setup        ComparisonSetup  `json:"setup"`
	SetupFork    ForkSpec         `json:"setup_fork"`
	SagPercent   float64          `json:"sag_percent"`
	EffectiveA2c float64          `json:"effective_a2c"`
	Bike         BikeGeometry     `json:"bike"`
	Validation   ValidationReport `json:"validation"`
	Baseline     BaselineMetrics  `json:"baseline"`
	Canonical    BikeFitAnalysis  `json:"canonical"`
}

type ForkPreset struct {
	ID                 string  `json:"id"`
	Name               string  `json:"name"`
	TravelMm           float64 `json:"travel_mm"`
	StaticA2c          float64 `json:"a2c"`
	Rake               float64 `json:"rake"`
	StandardSagPercent float64 `json:"standard_sag_percent"`
	Description        string  `json:"description"`
}

func (p ForkPreset) ToForkSpec() ForkSpec {
	return ForkSpec{
		ID:                 p.ID,
		Name:               p.Name,
		TravelMm:           p.TravelMm,
		A2c:                p.StaticA2c,
		Rake:               p.Rake,
		StandardSagPercent: p.StandardSagPercent,
		Description:        p.Description,
	}
}

func (p *ForkPreset) UnmarshalJSON(data []byte) error {
	type Alias ForkPreset
	aux := struct {
		*Alias
		AltTravelMm   *float64 `json:"travelMm"`
		AltStaticA2c  *float64 `json:"staticA2c"`
		AltSagPercent *float64 `json:"standardSagPercent"`
	}{
		Alias: (*Alias)(p),
	}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	if p.TravelMm == 0 && aux.AltTravelMm != nil {
		p.TravelMm = *aux.AltTravelMm
	}
	if p.StaticA2c == 0 && aux.AltStaticA2c != nil {
		p.StaticA2c = *aux.AltStaticA2c
	}
	if p.StandardSagPercent == 0 && aux.AltSagPercent != nil {
		p.StandardSagPercent = *aux.AltSagPercent
	}
	return nil
}

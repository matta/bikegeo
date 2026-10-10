package model

type HeadsetCupType string

const (
	CupEC44   HeadsetCupType = "EC44"
	CupZS44   HeadsetCupType = "ZS44"
	CupZS56   HeadsetCupType = "ZS56"
	CupIS42   HeadsetCupType = "IS42"
	CupIS52   HeadsetCupType = "IS52"
	CupCustom HeadsetCupType = "Custom"
)

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
	ID           string        `json:"id"`
	Manufacturer string        `json:"manufacturer"`
	Model        string        `json:"model"`
	Size         string        `json:"size"`
	Year         int           `json:"year,omitempty"`
	Category     BikeCategory  `json:"category,omitempty"`
	Notes        string        `json:"notes,omitempty"`

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
	ForkTravelMm           float64        `json:"fork_travel_mm,omitempty"`
	ForkA2c                float64        `json:"fork_a2c,omitempty"`
	SagPercentagePublished float64        `json:"sag_percentage_published,omitempty"`
	HeadsetCupType         HeadsetCupType `json:"headset_cup_type,omitempty"`
	CupStack               float64        `json:"cup_stack,omitempty"`

	// Rider Fit Reference
	SeatHeight float64 `json:"seat_height,omitempty"`
}

// SizeGeometry represents primary and optional frame dimensions for one size.
type SizeGeometry struct {
	Reach           float64 `json:"reach"`
	Stack           float64 `json:"stack"`
	HtaDeg          float64 `json:"hta_deg"`
	StaDeg          float64 `json:"sta_deg"`
	Htl             float64 `json:"htl"`
	BbDrop          float64 `json:"bb_drop"`
	ForkRake        float64 `json:"fork_rake"`
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
	ForkTravelMm           float64                 `json:"fork_travel_mm"`
	ForkA2c                float64                 `json:"fork_a2c"`
	SagPercentagePublished float64                 `json:"sag_percentage_published"`
	CupStack               *float64                `json:"cup_stack,omitempty"`
	Notes                  string                  `json:"notes,omitempty"`
	Geometries             map[string]SizeGeometry `json:"geometries,omitempty"`
}

// FrameModel represents a unified bike frame family on disk and in memory.
type FrameModel struct {
	ID             string               `json:"id"`
	Manufacturer   string               `json:"manufacturer"`
	Model          string               `json:"model"`
	Year           int                  `json:"year,omitempty"`
	Category       BikeCategory         `json:"category,omitempty"`
	Notes          string               `json:"notes,omitempty"`
	WheelSize      string               `json:"wheel_size,omitempty"`
	HeadsetCupType HeadsetCupType       `json:"headset_cup_type,omitempty"`
	CupStack       float64              `json:"cup_stack,omitempty"`
	SeatHeight     float64              `json:"seat_height,omitempty"`
	Sizes          []string             `json:"sizes"`
	DefaultSize    string               `json:"default_size,omitempty"`
	Configurations []FrameConfiguration `json:"configurations"`
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
	CupStack            float64           `json:"cupStack"`
	TotalStructuralA2c  float64           `json:"totalStructuralA2c"`
	A2cDifference       *float64          `json:"a2cDifference,omitempty"`
	InferredPublishMode PublishMode       `json:"inferredPublishMode"`
	FrontCenterCalc     float64           `json:"frontCenterCalc,omitempty"`
	FrontCenterDiff     *float64          `json:"frontCenterDiff,omitempty"`
}

type CanonicalConfig struct {
	TargetA2c  float64 `json:"targetA2c"`
	CupStack   float64 `json:"cupStack"`
	SeatHeight float64 `json:"seatHeight"`
	SagPercent float64 `json:"sagPercent"`
	Name       string  `json:"name,omitempty"`
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
	Bike       BikeGeometry     `json:"bike"`
	Validation ValidationReport `json:"validation"`
	Baseline   BaselineMetrics  `json:"baseline"`
	Canonical  BikeFitAnalysis  `json:"canonical"`
}

type ForkPreset struct {
	ID                 string  `json:"id"`
	Name               string  `json:"name"`
	TravelMm           float64 `json:"travelMm"`
	StaticA2c          float64 `json:"staticA2c"`
	StandardSagPercent float64 `json:"standardSagPercent"`
	Description        string  `json:"description"`
}

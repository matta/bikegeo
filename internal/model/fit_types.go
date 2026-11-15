package model

// RiderProfile encapsulates anthropometric measurements and baseline cockpit reference parameters.
type RiderProfile struct {
	HeightIn                      float64
	InseamIn                      float64
	SaddleHeightMm                float64
	RiderWeightLbs                float64
	BikeWeightLbs                 float64
	BaselineStemMm                float64
	BaselineStemAngleDeg          Deg
	Handlebar                     HandlebarSpec
	BaselineSaddleSetbackOffsetMm float64
	TargetRef                     TargetRef
	ReferenceBar                  Opt[HandlebarSpec]
}

type CanonicalConfig struct {
	TargetA2c    float64
	SagPercent   float64
	Name         string
	Saddle       SaddleSpec
	Handlebar    HandlebarSpec
	RiderProfile RiderProfile
}

// ComparisonSetup represents one column in a comparison matrix.
type ComparisonSetup struct {
	FrameID      string
	Size         string
	A2cMm        float64
	RakeMm       float64
	TravelMm     float64
	IsRigid      bool
	DisplayLabel string
	SagPct       float64
	AutoExpanded bool
	// IsCadFork marks the frame's own CAD fork configuration among the auto-expanded
	// candidates; sag pruning never discards it.
	IsCadFork bool
}

// SavedComparisonState holds the active comparison configuration.
type SavedComparisonState struct {
	RiderProfile
	Transpose    bool
	RefIndex     int
	IdealProfile string
	SaddleID     string
	Saddle       SaddleSpec
	HandlebarID  string
	Handlebar    HandlebarSpec
	Columns      []ComparisonSetup
}

type DynamicGeometryResult struct {
	NewHta        Deg
	NewSta        Deg
	NewReach      float64
	NewStack      float64
	NewBbDrop     float64
	DeltaAngleDeg Deg
	DeltaReach    float64
	DeltaStack    float64
	DeltaBbDrop   float64
	TotalNewA2c   float64
}

// StemSpacerFit holds the exact stem length and spacer stack at a given stem
// angle that places StemClamp at RequiredStemClamp.
type StemSpacerFit struct {
	StemAngleDeg Deg
	StemLengthMm float64
	SpacerMm     float64
}

type BikeFitAnalysis struct {
	StructuralA2c       float64
	ActiveReach         float64
	ActiveStack         float64
	ActiveHta           Deg
	ActiveSta           Deg
	ActiveBbDrop        float64
	Aett                float64
	Ett                 float64
	ActualSetback       float64
	SpareVerticalStack  float64
	SpareAxialSteerer   float64
	SeatpostExtensionMm float64
	DeltaAngleDeg       Deg
	DeltaReach          float64
	DeltaStack          float64
	DeltaBbDrop         float64
	InferredPublishMode string

	// Axle Centering & Frame Proportions
	RearCenterWheelbaseRatio float64
	SeatToRearCenter         float64
	WheelbaseCalc            float64
	ActiveChainstay          float64
	ChainstayDesc            string
	TrailMm                  float64

	// Cockpit BRP & HB Points and Saddle-Handlebar Metrics
	Saddle              SaddleSpec
	Handlebar           HandlebarSpec
	SitBoneSetbackMm    float64
	IdealStaDeg         Deg
	AchievedClampOffset float64
	DeadCenterBRP       BRPPoint
	SelectedBRP         *BRPPoint
	PossibleBRPs        []BRPPoint
	SaddleHbMm          float64
	SaddleHbDropMm      float64
	SelectedHB          *HBPoint
	PossibleHBs         []HBPoint
	CockpitFitResult    *CockpitFitResult
	StemSpacerFit       *StemSpacerFit
}

type BaselineMetrics struct {
	Reach              float64
	Stack              float64
	HtaDeg             Deg
	StaDeg             Deg
	BbDrop             float64
	A2c                float64
	Aett               float64
	ActualSetback      float64
	SpareVerticalStack float64
	SpareAxialSteerer  float64
	SeatpostExtension  float64
	Chainstay          float64
	SeatToRearCenter   float64
	SaddleHbMm         float64
	TrailMm            float64
}

type CanonicalComparisonRow struct {
	Setup         ComparisonSetup
	DisplayLabel  string
	EffectiveA2c  float64
	EffectiveRake float64
	Bike          BikeGeometry
	Validation    ValidationReport
	Baseline      BaselineMetrics
	Canonical     BikeFitAnalysis
}

package storage_test

import (
	"os"
	"testing"

	"bikegeo/pkg/geometry"
	"bikegeo/pkg/model"
	"bikegeo/pkg/storage"
)

func TestStorageCRUD(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "bikegeo-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	store, err := storage.NewBikeStore(tempDir)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}

	// 1. Initial should be empty
	bikes, err := store.GetAllBikes()
	if err != nil {
		t.Fatalf("GetAllBikes error: %v", err)
	}
	if len(bikes) != 0 {
		t.Errorf("Expected 0 bikes, got %d", len(bikes))
	}

	// 2. Save new bike
	newBike := model.BikeGeometry{
		Manufacturer: "Kona",
		Model:        "Honzo",
		Size:         "M",
		Reach:        450,
		Stack:        630,
		HtaDeg:       66.0,
		StaDeg:       75.0,
		Htl:          110,
		BbDrop:       60,
		ForkRake:     44,
		Wheelbase:    1180,
	}

	saved, err := store.SaveBike(newBike)
	if err != nil {
		t.Fatalf("SaveBike error: %v", err)
	}
	if saved.ID != "kona-honzo-m" {
		t.Errorf("Expected auto-generated ID 'kona-honzo-m', got %s", saved.ID)
	}

	// 3. Retrieve bike by ID
	fetched, err := store.GetBikeByID("kona-honzo-m")
	if err != nil {
		t.Fatalf("GetBikeByID error: %v", err)
	}
	if fetched == nil || fetched.Reach != 450 {
		t.Fatalf("Expected fetched bike with reach 450, got %+v", fetched)
	}

	// 4. Update bike
	fetched.Reach = 455
	updated, err := store.SaveBike(*fetched)
	if err != nil {
		t.Fatalf("Update error: %v", err)
	}
	if updated.Reach != 455 {
		t.Errorf("Expected reach 455, got %.1f", updated.Reach)
	}

	// 5. Delete bike
	deleted, err := store.DeleteBike("kona-honzo-m")
	if err != nil {
		t.Fatalf("Delete error: %v", err)
	}
	if !deleted {
		t.Errorf("Expected deleted=true")
	}

	// 6. Verify deleted
	afterDel, err := store.GetBikeByID("kona-honzo-m")
	if err != nil {
		t.Fatalf("GetBikeByID after delete error: %v", err)
	}
	if afterDel != nil {
		t.Errorf("Expected nil after delete, got %+v", afterDel)
	}
}

func TestFrameModelStorage(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "bikegeo-frame-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	store, err := storage.NewBikeStore(tempDir)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}

	cadFork := model.ForkRef{Fork: "fox34_120", SagPct: model.Float64Ptr(20), A2cMm: 506, RakeMm: 44}
	rigidFork := &model.ForkRef{Fork: "rigid_483", A2cMm: 483, RakeMm: 44}
	frame := model.FrameModel{
		ID:                 "chumba-sendero",
		Manufacturer:       "Chumba",
		Model:              "Sendero",
		Year:               2024,
		Category:           model.CategoryHardtail,
		HeadsetCupType:     model.CupEC44,
		GeoHeadsetSpacerMm: model.Float64Ptr(12.0),
		Sizes:              []string{"S", "M", "L", "XL"},
		DefaultSize:        "L",
		CadFork:            cadFork,
		DefaultFork:        cadFork,
		RigidFork:          rigidFork,
		ForkRange: []model.ForkRef{
			{Fork: "rigid_483", A2cMm: 483, RakeMm: 44},
			{Fork: "fox34_130", A2cMm: 540, RakeMm: 44},
		},
		Geometries: map[string]model.SizeGeometry{
			"S":  {Reach: 425, Stack: 625, HtaDeg: 67.0, StaDeg: 74.5, Htl: 100, BbDrop: 60, Wheelbase: 1145},
			"M":  {Reach: 450, Stack: 635, HtaDeg: 67.0, StaDeg: 74.5, Htl: 110, BbDrop: 60, Wheelbase: 1172},
			"L":  {Reach: 475, Stack: 645, HtaDeg: 67.0, StaDeg: 74.5, Htl: 125, BbDrop: 60, Wheelbase: 1200},
			"XL": {Reach: 500, Stack: 655, HtaDeg: 67.0, StaDeg: 74.5, Htl: 140, BbDrop: 60, Wheelbase: 1230},
		},
	}

	saved, err := store.SaveFrame(frame)
	if err != nil {
		t.Fatalf("SaveFrame error: %v", err)
	}
	if saved.ID != "chumba-sendero" {
		t.Errorf("Expected ID chumba-sendero, got %s", saved.ID)
	}

	fetched, err := store.GetFrameByID("chumba-sendero")
	if err != nil {
		t.Fatalf("GetFrameByID error: %v", err)
	}
	if fetched == nil || len(fetched.Sizes) != 4 {
		t.Fatalf("Expected 4 sizes in fetched frame, got %+v", fetched)
	}

	// GetAllBikes should materialize 4 sizes
	bikes, err := store.GetAllBikes()
	if err != nil {
		t.Fatalf("GetAllBikes error: %v", err)
	}
	if len(bikes) != 4 {
		t.Errorf("Expected 4 materialized bikes, got %d", len(bikes))
	}

	// Retrieve by specific size ID
	largeBike, err := store.GetBikeByID("chumba-sendero-l")
	if err != nil {
		t.Fatalf("GetBikeByID error: %v", err)
	}
	if largeBike == nil || largeBike.Reach != 475 {
		t.Errorf("Expected reach 475 for L, got %+v", largeBike)
	}
	if largeBike.ForkA2c != 506 {
		t.Errorf("Expected fork A2C 506 for L, got %.1f", largeBike.ForkA2c)
	}

	// Kinematically derive geometry for rigid fork (483mm A2C vs 506mm CAD)
	derived := geometry.CalculateDynamicGeometry(
		largeBike.Reach, largeBike.Stack, largeBike.HtaDeg, largeBike.StaDeg, largeBike.BbDrop,
		506.0+largeBike.GetGeoHeadsetSpacerMm(), 483.0+largeBike.GetGeoHeadsetSpacerMm(),
		largeBike.Wheelbase, largeBike.ChainstayLength,
	)
	// Rigid fork (483 vs 506) pitches down front end, increasing reach and steepening HTA
	if derived.NewHta <= 67.0 {
		t.Errorf("Expected steeper HTA for rigid fork, got %.2f", derived.NewHta)
	}
	if derived.NewReach <= 475.0 {
		t.Errorf("Expected longer reach for rigid fork, got %.1f", derived.NewReach)
	}
}

func TestLoadRealBikesDataDir(t *testing.T) {
	store, err := storage.NewBikeStore("../../data/bikes")
	if err != nil {
		t.Fatalf("Failed to initialize store for ../../data/bikes: %v", err)
	}
	frames, err := store.GetAllFrames()
	if err != nil {
		t.Fatalf("Failed to get all frames: %v", err)
	}
	if len(frames) == 0 {
		t.Fatalf("Expected at least one frame in ../../data/bikes, got 0")
	}
	bikes, err := store.GetAllBikes()
	if err != nil {
		t.Fatalf("Failed to get all bikes: %v", err)
	}
	expectedBikes := 0
	for _, f := range frames {
		expectedBikes += len(f.Sizes)
	}
	if len(bikes) != expectedBikes {
		t.Errorf("Expected %d materialized bikes, got %d", expectedBikes, len(bikes))
	}
	for _, b := range bikes {
		if b.ForkA2c <= 0 {
			t.Errorf("Bike %s has invalid ForkA2c: %.1f", b.ID, b.ForkA2c)
		}
		if b.ForkRake <= 0 {
			t.Errorf("Bike %s has invalid ForkRake: %.1f", b.ID, b.ForkRake)
		}
	}
}

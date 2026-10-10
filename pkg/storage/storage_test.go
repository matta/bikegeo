package storage_test

import (
	"os"
	"testing"

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

	frame := model.FrameModel{
		ID:             "chumba-sendero",
		Manufacturer:   "Chumba",
		Model:          "Sendero",
		Year:           2024,
		Category:       model.CategoryHardtail,
		HeadsetCupType: model.CupEC44,
		CupStack:       12.0,
		Sizes:          []string{"S", "M", "L", "XL"},
		DefaultSize:    "L",
		Configurations: []model.FrameConfiguration{
			{
				ID:           "fork-120",
				Name:         "120mm Suspension",
				IsBaseline:   true,
				ForkTravelMm: 120,
				ForkA2c:      531,
				Geometries: map[string]model.SizeGeometry{
					"S":  {Reach: 425, Stack: 625, HtaDeg: 67.0, StaDeg: 74.5, Htl: 100, BbDrop: 60, ForkRake: 44, Wheelbase: 1145},
					"M":  {Reach: 450, Stack: 635, HtaDeg: 67.0, StaDeg: 74.5, Htl: 110, BbDrop: 60, ForkRake: 44, Wheelbase: 1172},
					"L":  {Reach: 475, Stack: 645, HtaDeg: 67.0, StaDeg: 74.5, Htl: 125, BbDrop: 60, ForkRake: 44, Wheelbase: 1200},
					"XL": {Reach: 500, Stack: 655, HtaDeg: 67.0, StaDeg: 74.5, Htl: 140, BbDrop: 60, ForkRake: 44, Wheelbase: 1230},
				},
			},
			{
				ID:           "rigid-483",
				Name:         "Rigid Fork",
				IsBaseline:   false,
				ForkTravelMm: 0,
				ForkA2c:      483,
				// Deliberately no geometries: should derive kinematically!
			},
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

	// Retrieve by alternate config ID (kinematically derived)
	rigidLarge, err := store.GetBikeByID("chumba-sendero-l-rigid-483")
	if err != nil {
		t.Fatalf("GetBikeByID rigid error: %v", err)
	}
	if rigidLarge == nil {
		t.Fatalf("Expected rigid large bike to be materialized")
	}
	// Rigid fork (483 vs 531) pitches down front end, increasing reach and steepening HTA
	if rigidLarge.HtaDeg <= 67.0 {
		t.Errorf("Expected steeper HTA for rigid fork, got %.2f", rigidLarge.HtaDeg)
	}
	if rigidLarge.Reach <= 475.0 {
		t.Errorf("Expected longer reach for rigid fork, got %.1f", rigidLarge.Reach)
	}
}


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

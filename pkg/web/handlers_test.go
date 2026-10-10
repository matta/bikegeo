package web_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"

	"bikegeo/pkg/model"
	"bikegeo/pkg/storage"
	"bikegeo/pkg/web"
)

func setupTestServer(t *testing.T) (*web.Server, *storage.BikeStore, func()) {
	t.Helper()
	tempDir, err := os.MkdirTemp("", "bikegeo-web-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}

	store, err := storage.NewBikeStore(tempDir)
	if err != nil {
		os.RemoveAll(tempDir)
		t.Fatalf("Failed to create store: %v", err)
	}

	// Seed test bike
	testBike := model.BikeGeometry{
		ID:           "test-bike-1",
		Manufacturer: "Binary",
		Model:        "Havok",
		Size:         "M",
		Category:     model.CategoryHardtail,
		Reach:        398.0,
		Stack:        656.0,
		HtaDeg:       69.0,
		StaDeg:       74.0,
		Htl:          150.0,
		BbDrop:       70.0,
		ForkRake:     44.0,
		Wheelbase:    1150.0,
		ForkA2c:      521.0,
		CupStack:     12.0,
	}
	if _, err := store.SaveBike(testBike); err != nil {
		os.RemoveAll(tempDir)
		t.Fatalf("Failed to seed test bike: %v", err)
	}

	srv := web.NewServer(store)
	cleanup := func() {
		os.RemoveAll(tempDir)
	}
	return srv, store, cleanup
}

func TestIndexPage(t *testing.T) {
	srv, _, cleanup := setupTestServer(t)
	defer cleanup()

	// 1. Regular browser GET /
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected 200 OK, got %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "BikeGeo Analyst") {
		t.Errorf("Expected page to contain 'BikeGeo Analyst'")
	}
	if !strings.Contains(body, "htmx.min.js") {
		t.Errorf("Expected page to include htmx.min.js")
	}

	// 2. HTMX GET /?tab=validator
	reqHtmx := httptest.NewRequest(http.MethodGet, "/?tab=validator", nil)
	reqHtmx.Header.Set("HX-Request", "true")
	recHtmx := httptest.NewRecorder()
	srv.ServeHTTP(recHtmx, reqHtmx)

	if recHtmx.Code != http.StatusOK {
		t.Errorf("Expected 200 OK for HTMX request, got %d", recHtmx.Code)
	}
	bodyHtmx := recHtmx.Body.String()
	if !strings.Contains(bodyHtmx, "Master Stack Equation Proof") {
		t.Errorf("Expected validator view to contain 'Master Stack Equation Proof'")
	}
	// Verify that Navbar is swapped out-of-band with Master Stack CAD Proof button highlighted
	if !strings.Contains(bodyHtmx, `id="main-navbar" hx-swap-oob="true"`) {
		t.Errorf("Expected out-of-band navbar swap with hx-swap-oob=\"true\"")
	}
	if !strings.Contains(bodyHtmx, "bg-amber-500") {
		t.Errorf("Expected highlighted button to contain amber background class")
	}

	// 3. HTMX GET /?tab=compare
	reqCompare := httptest.NewRequest(http.MethodGet, "/?tab=compare", nil)
	reqCompare.Header.Set("HX-Request", "true")
	recCompare := httptest.NewRecorder()
	srv.ServeHTTP(recCompare, reqCompare)

	if recCompare.Code != http.StatusOK {
		t.Errorf("Expected 200 OK for HTMX compare request, got %d", recCompare.Code)
	}
	bodyCompare := recCompare.Body.String()
	if !strings.Contains(bodyCompare, `id="main-navbar" hx-swap-oob="true"`) {
		t.Errorf("Expected out-of-band navbar swap on compare tab")
	}
	if !strings.Contains(bodyCompare, "comparison-matrix") {
		t.Errorf("Expected compare view to contain 'comparison-matrix'")
	}
}

// Test Target Pattern 1: Inline Editable Data Cell
func TestTargetPattern1_InlineEdit(t *testing.T) {
	srv, store, cleanup := setupTestServer(t)
	defer cleanup()

	// 1. GET form for inline editing Reach
	reqForm := httptest.NewRequest(http.MethodGet, "/bikes/test-bike-1/inline-edit?field=reach", nil)
	recForm := httptest.NewRecorder()
	srv.ServeHTTP(recForm, reqForm)

	if recForm.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK, got %d", recForm.Code)
	}
	formBody := recForm.Body.String()
	if !strings.Contains(formBody, "<form") || !strings.Contains(formBody, "name=\"value\"") {
		t.Errorf("Expected inline edit form, got %s", formBody)
	}

	// 2. POST new Reach value to save
	form := url.Values{}
	form.Set("value", "415.5")
	reqSave := httptest.NewRequest(http.MethodPost, "/bikes/test-bike-1/inline-edit?field=reach", strings.NewReader(form.Encode()))
	reqSave.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	recSave := httptest.NewRecorder()
	srv.ServeHTTP(recSave, reqSave)

	if recSave.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK, got %d", recSave.Code)
	}
	saveBody := recSave.Body.String()
	if !strings.Contains(saveBody, "415.5") {
		t.Errorf("Expected updated display cell with 415.5, got %s", saveBody)
	}

	// Verify persistence in store
	updatedBike, _ := store.GetBikeByID("test-bike-1")
	if updatedBike.Reach != 415.5 {
		t.Errorf("Expected persisted reach 415.5, got %.1f", updatedBike.Reach)
	}
}

// Test Target Pattern 2: Debounced Search Filtering
func TestTargetPattern2_DebouncedSearch(t *testing.T) {
	srv, _, cleanup := setupTestServer(t)
	defer cleanup()

	// 1. Search for matching bike
	reqMatch := httptest.NewRequest(http.MethodGet, "/compare/matrix?q=Havok&selected_ids=test-bike-1", nil)
	recMatch := httptest.NewRecorder()
	srv.ServeHTTP(recMatch, reqMatch)

	if recMatch.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK, got %d", recMatch.Code)
	}
	bodyMatch := recMatch.Body.String()
	if !strings.Contains(bodyMatch, "Binary Havok") {
		t.Errorf("Expected results to contain 'Binary Havok'")
	}

	// 2. Search for non-matching query
	reqNoMatch := httptest.NewRequest(http.MethodGet, "/compare/matrix?q=NonExistent&selected_ids=test-bike-1", nil)
	recNoMatch := httptest.NewRecorder()
	srv.ServeHTTP(recNoMatch, reqNoMatch)

	if recNoMatch.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK, got %d", recNoMatch.Code)
	}
	bodyNoMatch := recNoMatch.Body.String()
	if strings.Contains(bodyNoMatch, "Binary Havok") {
		t.Errorf("Expected 'Binary Havok' to be filtered out")
	}
}

// Test Target Pattern 3: Asynchronous Form Submission with Loading Indicator & Inline Validation Errors
func TestTargetPattern3_AsyncFormValidation(t *testing.T) {
	srv, _, cleanup := setupTestServer(t)
	defer cleanup()

	// 1. Submit invalid form with missing manufacturer and negative reach
	form := url.Values{}
	form.Set("manufacturer", "")
	form.Set("model", "TestModel")
	form.Set("size", "L")
	form.Set("reach", "-10") // Invalid negative reach

	reqInvalid := httptest.NewRequest(http.MethodPost, "/bikes/save?is_new=true", strings.NewReader(form.Encode()))
	reqInvalid.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	reqInvalid.Header.Set("HX-Request", "true")
	recInvalid := httptest.NewRecorder()
	srv.ServeHTTP(recInvalid, reqInvalid)

	invalidBody := recInvalid.Body.String()
	// Should render modal with validation errors inline
	if !strings.Contains(invalidBody, "Missing Manufacturer") {
		t.Errorf("Expected inline validation error for Missing Manufacturer")
	}
	if !strings.Contains(invalidBody, "id=\"save-indicator\"") {
		t.Errorf("Expected loading indicator save-indicator in form")
	}

	// 2. Submit valid form
	validForm := url.Values{}
	validForm.Set("manufacturer", "Santa Cruz")
	validForm.Set("model", "Chameleon")
	validForm.Set("size", "L")
	validForm.Set("category", "hardtail")
	validForm.Set("reach", "465")
	validForm.Set("stack", "635")
	validForm.Set("hta_deg", "65.5")
	validForm.Set("sta_deg", "74.5")
	validForm.Set("htl", "120")
	validForm.Set("bb_drop", "55")
	validForm.Set("fork_rake", "44")
	validForm.Set("wheelbase", "1200")
	validForm.Set("fork_a2c", "541")
	validForm.Set("fork_travel_mm", "130")

	reqValid := httptest.NewRequest(http.MethodPost, "/bikes/save?is_new=true", strings.NewReader(validForm.Encode()))
	reqValid.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	reqValid.Header.Set("HX-Request", "true")
	recValid := httptest.NewRecorder()
	srv.ServeHTTP(recValid, reqValid)

	if recValid.Code != http.StatusOK {
		t.Errorf("Expected 200 OK on valid save, got %d", recValid.Code)
	}
	loc := recValid.Header().Get("HX-Location")
	if !strings.Contains(loc, "santa-cruz-chameleon-l") {
		t.Errorf("Expected HX-Location redirect to new bike, got %s", loc)
	}
}

// Test REST API Compatibility
func TestRestAPICompatibility(t *testing.T) {
	srv, _, cleanup := setupTestServer(t)
	defer cleanup()

	// 1. GET /api/bikes
	reqGet := httptest.NewRequest(http.MethodGet, "/api/bikes", nil)
	recGet := httptest.NewRecorder()
	srv.ServeHTTP(recGet, reqGet)

	if recGet.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK, got %d", recGet.Code)
	}
	var bikes []model.BikeGeometry
	if err := json.Unmarshal(recGet.Body.Bytes(), &bikes); err != nil {
		t.Fatalf("Failed to parse API bikes JSON: %v", err)
	}
	if len(bikes) != 1 {
		t.Errorf("Expected 1 bike, got %d", len(bikes))
	}

	// 2. POST /api/validate
	valPayload, _ := json.Marshal(bikes[0])
	reqVal := httptest.NewRequest(http.MethodPost, "/api/validate", bytes.NewReader(valPayload))
	recVal := httptest.NewRecorder()
	srv.ServeHTTP(recVal, reqVal)

	if recVal.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK for /api/validate, got %d", recVal.Code)
	}
	var valReport model.ValidationReport
	if err := json.Unmarshal(recVal.Body.Bytes(), &valReport); err != nil {
		t.Fatalf("Failed to parse validation report: %v", err)
	}
	if !valReport.IsValid {
		t.Errorf("Expected valid report")
	}

	// 3. POST /api/canonicalize
	canonPayload, _ := json.Marshal(map[string]any{
		"bikes": bikes,
		"config": model.CanonicalConfig{
			TargetA2c:  515.0,
			CupStack:   12.0,
			SeatHeight: 780.0,
			SagPercent: 20.0,
		},
	})
	reqCanon := httptest.NewRequest(http.MethodPost, "/api/canonicalize", bytes.NewReader(canonPayload))
	recCanon := httptest.NewRecorder()
	srv.ServeHTTP(recCanon, reqCanon)

	if recCanon.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK for /api/canonicalize, got %d", recCanon.Code)
	}
	var canonRows []model.CanonicalComparisonRow
	if err := json.Unmarshal(recCanon.Body.Bytes(), &canonRows); err != nil {
		t.Fatalf("Failed to parse canonical rows: %v", err)
	}
	if len(canonRows) != 1 {
		t.Errorf("Expected 1 canonical row, got %d", len(canonRows))
	}
}

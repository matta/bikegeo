package web_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"

	"bikegeo/pkg/model"
	"bikegeo/pkg/storage"
	"bikegeo/pkg/web"

	"github.com/PuerkitoBio/goquery"
)

// Helpers for goquery DOM assertions
func parseHTML(t *testing.T, body string) *goquery.Document {
	t.Helper()
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(body))
	if err != nil {
		t.Fatalf("Failed to parse HTML document: %v", err)
	}
	return doc
}

func assertElement(t *testing.T, doc *goquery.Document, selector string) *goquery.Selection {
	t.Helper()
	sel := doc.Find(selector)
	if sel.Length() == 0 {
		t.Errorf("Expected element matching selector %q, found none", selector)
	}
	return sel
}

func assertAttr(t *testing.T, sel *goquery.Selection, attr, expected string) {
	t.Helper()
	val, exists := sel.Attr(attr)
	if !exists {
		t.Errorf("Expected attribute %q to exist on element", attr)
		return
	}
	if !strings.Contains(val, expected) {
		t.Errorf("Expected attribute %q to contain %q, got %q", attr, expected, val)
	}
}

func assertDoctype(t *testing.T, body string, expectFullLayout bool) {
	t.Helper()
	hasDoctype := strings.Contains(strings.ToLower(body), "<!doctype html>")
	if expectFullLayout && !hasDoctype {
		t.Errorf("Expected full layout with <!doctype html>, but got fragment")
	} else if !expectFullLayout && hasDoctype {
		t.Errorf("Expected partial fragment without <!doctype html>, but got full layout")
	}
}

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

	// 1. Regular browser GET / (full page layout)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected 200 OK, got %d", rec.Code)
	}
	assertDoctype(t, rec.Body.String(), true)
	doc := parseHTML(t, rec.Body.String())
	assertElement(t, doc, "head title")
	assertElement(t, doc, "script[src*='htmx.min.js']")
	assertElement(t, doc, "#navbar-container")
	assertElement(t, doc, "#app-content")

	// 2. HTMX GET /?tab=validator (returns only fragment + out-of-band navbar)
	reqHtmx := httptest.NewRequest(http.MethodGet, "/?tab=validator", nil)
	reqHtmx.Header.Set("HX-Request", "true")
	recHtmx := httptest.NewRecorder()
	srv.ServeHTTP(recHtmx, reqHtmx)

	if recHtmx.Code != http.StatusOK {
		t.Errorf("Expected 200 OK for HTMX request, got %d", recHtmx.Code)
	}
	assertDoctype(t, recHtmx.Body.String(), false)
	docHtmx := parseHTML(t, recHtmx.Body.String())
	navbarOob := assertElement(t, docHtmx, "#main-navbar")
	assertAttr(t, navbarOob, "hx-swap-oob", "true")
	assertElement(t, docHtmx, "#sag-simulator-results")

	// 3. HTMX GET /?tab=compare
	reqCompare := httptest.NewRequest(http.MethodGet, "/?tab=compare", nil)
	reqCompare.Header.Set("HX-Request", "true")
	recCompare := httptest.NewRecorder()
	srv.ServeHTTP(recCompare, reqCompare)

	if recCompare.Code != http.StatusOK {
		t.Errorf("Expected 200 OK for HTMX compare request, got %d", recCompare.Code)
	}
	assertDoctype(t, recCompare.Body.String(), false)
	docCompare := parseHTML(t, recCompare.Body.String())
	assertElement(t, docCompare, "#comparison-matrix")
	navCompare := assertElement(t, docCompare, "#main-navbar")
	assertAttr(t, navCompare, "hx-swap-oob", "true")
}

// Test Target Pattern 1: Inline Editable Data Cell
func TestTargetPattern1_InlineEdit(t *testing.T) {
	srv, store, cleanup := setupTestServer(t)
	defer cleanup()

	// 1. GET form for inline editing Reach
	reqForm := httptest.NewRequest(http.MethodGet, "/bikes/test-bike-1-m/inline-edit?field=reach", nil)
	recForm := httptest.NewRecorder()
	srv.ServeHTTP(recForm, reqForm)

	if recForm.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK, got %d", recForm.Code)
	}
	docForm := parseHTML(t, recForm.Body.String())
	input := assertElement(t, docForm, "form input[name='value']")
	assertAttr(t, input, "value", "398.0")
	formEl := assertElement(t, docForm, "form")
	assertAttr(t, formEl, "hx-post", "/bikes/test-bike-1-m/inline-edit?field=reach")
	assertAttr(t, formEl, "hx-swap", "outerHTML")

	// 2. Cancel / Display view
	reqDisp := httptest.NewRequest(http.MethodGet, "/bikes/test-bike-1-m/inline-display?field=reach", nil)
	recDisp := httptest.NewRecorder()
	srv.ServeHTTP(recDisp, reqDisp)
	if recDisp.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK for inline display, got %d", recDisp.Code)
	}
	docDisp := parseHTML(t, recDisp.Body.String())
	assertElement(t, docDisp, "[hx-get*='/bikes/test-bike-1-m/inline-edit?field=reach']")

	// 3. POST new Reach value to save
	form := url.Values{}
	form.Set("value", "415.5")
	reqSave := httptest.NewRequest(http.MethodPost, "/bikes/test-bike-1-m/inline-edit?field=reach", strings.NewReader(form.Encode()))
	reqSave.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	recSave := httptest.NewRecorder()
	srv.ServeHTTP(recSave, reqSave)

	if recSave.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK, got %d", recSave.Code)
	}
	if recSave.Header().Get("HX-Trigger") != "refreshMatrix" {
		t.Errorf("Expected HX-Trigger header 'refreshMatrix', got %s", recSave.Header().Get("HX-Trigger"))
	}
	docSave := parseHTML(t, recSave.Body.String())
	spanVal := assertElement(t, docSave, "span.font-mono")
	if !strings.Contains(spanVal.Text(), "415.5") {
		t.Errorf("Expected updated display cell with 415.5, got %s", spanVal.Text())
	}

	// 4. Invalid value returns 400
	formBad := url.Values{}
	formBad.Set("value", "-50")
	reqBad := httptest.NewRequest(http.MethodPost, "/bikes/test-bike-1-m/inline-edit?field=reach", strings.NewReader(formBad.Encode()))
	reqBad.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	recBad := httptest.NewRecorder()
	srv.ServeHTTP(recBad, reqBad)
	if recBad.Code != http.StatusBadRequest {
		t.Errorf("Expected 400 Bad Request on invalid inline value, got %d", recBad.Code)
	}

	// Verify persistence in store
	updatedBike, _ := store.GetBikeByID("test-bike-1-m")
	if updatedBike.Reach != 415.5 {
		t.Errorf("Expected persisted reach 415.5, got %.1f", updatedBike.Reach)
	}
}

// Test Target Pattern 2: Debounced Search Filtering
func TestTargetPattern2_DebouncedSearch(t *testing.T) {
	srv, _, cleanup := setupTestServer(t)
	defer cleanup()

	// 1. Search for matching bike
	reqMatch := httptest.NewRequest(http.MethodGet, "/compare/matrix?q=Havok&selected_ids=test-bike-1-m", nil)
	recMatch := httptest.NewRecorder()
	srv.ServeHTTP(recMatch, reqMatch)

	if recMatch.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK, got %d", recMatch.Code)
	}
	docMatch := parseHTML(t, recMatch.Body.String())
	assertElement(t, docMatch, "table")
	if !strings.Contains(docMatch.Text(), "Binary Havok") {
		t.Errorf("Expected results to contain 'Binary Havok'")
	}

	// 2. Search for non-matching query
	reqNoMatch := httptest.NewRequest(http.MethodGet, "/compare/matrix?q=NonExistent&selected_ids=test-bike-1-m", nil)
	recNoMatch := httptest.NewRecorder()
	srv.ServeHTTP(recNoMatch, reqNoMatch)

	if recNoMatch.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK, got %d", recNoMatch.Code)
	}
	docNoMatch := parseHTML(t, recNoMatch.Body.String())
	if strings.Contains(docNoMatch.Text(), "Binary Havok") {
		t.Errorf("Expected 'Binary Havok' to be filtered out")
	}
}

// Test Target Pattern 3: Asynchronous Form Submission with Loading Indicator & Inline Validation Errors (422)
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

	// Convention check: Validation failures must return 422 Unprocessable Entity
	if recInvalid.Code != http.StatusUnprocessableEntity {
		t.Errorf("Expected 422 Unprocessable Entity on validation failure, got %d", recInvalid.Code)
	}

	docInvalid := parseHTML(t, recInvalid.Body.String())
	assertElement(t, docInvalid, "#save-indicator")
	// Verify error message with goquery
	errorBanner := docInvalid.Find("div:contains('Missing Manufacturer')")
	if errorBanner.Length() == 0 {
		t.Errorf("Expected DOM to contain 'Missing Manufacturer' validation issue")
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
	if !strings.Contains(loc, "santa-cruz-chameleon") {
		t.Errorf("Expected HX-Location redirect to new bike, got %s", loc)
	}
}

// Test Compare Tab Controls & Parameters
func TestCompareTabControls(t *testing.T) {
	srv, _, cleanup := setupTestServer(t)
	defer cleanup()

	// 1. Preset change
	formPreset := url.Values{"preset": {"130mm"}, "selected_ids": {"test-bike-1-m"}}
	reqPreset := httptest.NewRequest(http.MethodPost, "/compare/preset", strings.NewReader(formPreset.Encode()))
	reqPreset.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	recPreset := httptest.NewRecorder()
	srv.ServeHTTP(recPreset, reqPreset)
	if recPreset.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK for /compare/preset, got %d", recPreset.Code)
	}
	docPreset := parseHTML(t, recPreset.Body.String())
	assertElement(t, docPreset, "#comparison-matrix")

	// 2. Cup Mode toggle
	formCup := url.Values{"cup_mode": {"preserve"}, "selected_ids": {"test-bike-1-m"}}
	reqCup := httptest.NewRequest(http.MethodPost, "/compare/cup-mode", strings.NewReader(formCup.Encode()))
	reqCup.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	recCup := httptest.NewRecorder()
	srv.ServeHTTP(recCup, reqCup)
	if recCup.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK for /compare/cup-mode, got %d", recCup.Code)
	}

	// 3. Toggle select
	formToggle := url.Values{"id": {"test-bike-1-m"}, "selected_ids": {"test-bike-1-m"}}
	reqToggle := httptest.NewRequest(http.MethodPost, "/compare/toggle-select", strings.NewReader(formToggle.Encode()))
	reqToggle.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	recToggle := httptest.NewRecorder()
	srv.ServeHTTP(recToggle, reqToggle)
	if recToggle.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK for /compare/toggle-select, got %d", recToggle.Code)
	}

	// 4. Select All & Clear All
	reqSelectAll := httptest.NewRequest(http.MethodPost, "/compare/select-all", nil)
	recSelectAll := httptest.NewRecorder()
	srv.ServeHTTP(recSelectAll, reqSelectAll)
	if recSelectAll.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK for /compare/select-all, got %d", recSelectAll.Code)
	}

	reqClearAll := httptest.NewRequest(http.MethodPost, "/compare/clear-all", nil)
	recClearAll := httptest.NewRecorder()
	srv.ServeHTTP(recClearAll, reqClearAll)
	if recClearAll.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK for /compare/clear-all, got %d", recClearAll.Code)
	}

	// 5. Diagram view
	formDiag := url.Values{"diagram_bike_id": {"test-bike-1-m"}, "selected_ids": {"test-bike-1-m"}}
	reqDiag := httptest.NewRequest(http.MethodPost, "/compare/diagram", strings.NewReader(formDiag.Encode()))
	reqDiag.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	recDiag := httptest.NewRecorder()
	srv.ServeHTTP(recDiag, reqDiag)
	if recDiag.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK for /compare/diagram, got %d", recDiag.Code)
	}
	docDiag := parseHTML(t, recDiag.Body.String())
	assertElement(t, docDiag, "svg")
}

// Test Validator Tab Sag Adjustment
func TestValidatorSagAdjustment(t *testing.T) {
	srv, _, cleanup := setupTestServer(t)
	defer cleanup()

	reqSag := httptest.NewRequest(http.MethodGet, "/validator/sag?sag=25&id=test-bike-1-m", nil)
	recSag := httptest.NewRecorder()
	srv.ServeHTTP(recSag, reqSag)
	if recSag.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK for /validator/sag, got %d", recSag.Code)
	}
	docSag := parseHTML(t, recSag.Body.String())
	assertElement(t, docSag, "div:contains('Head Angle')")
}

// Test Bike Editor Views: New and Edit (Dual Browser Layout vs HTMX fragment)
func TestBikeEditorViews(t *testing.T) {
	srv, _, cleanup := setupTestServer(t)
	defer cleanup()

	// 1. Browser GET /bikes/new (full layout)
	reqNewBrowser := httptest.NewRequest(http.MethodGet, "/bikes/new", nil)
	recNewBrowser := httptest.NewRecorder()
	srv.ServeHTTP(recNewBrowser, reqNewBrowser)
	if recNewBrowser.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK, got %d", recNewBrowser.Code)
	}
	assertDoctype(t, recNewBrowser.Body.String(), true)
	docNewBrowser := parseHTML(t, recNewBrowser.Body.String())
	assertElement(t, docNewBrowser, "#frame-editor-view")

	// 2. HTMX GET /bikes/new (fragment)
	reqNewHtmx := httptest.NewRequest(http.MethodGet, "/bikes/new", nil)
	reqNewHtmx.Header.Set("HX-Request", "true")
	recNewHtmx := httptest.NewRecorder()
	srv.ServeHTTP(recNewHtmx, reqNewHtmx)
	if recNewHtmx.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK, got %d", recNewHtmx.Code)
	}
	assertDoctype(t, recNewHtmx.Body.String(), false)
	docNewHtmx := parseHTML(t, recNewHtmx.Body.String())
	assertElement(t, docNewHtmx, "#frame-editor-view")

	// 3. Browser GET /bikes/test-bike-1-m/edit (full layout)
	reqEditBrowser := httptest.NewRequest(http.MethodGet, "/bikes/test-bike-1-m/edit", nil)
	recEditBrowser := httptest.NewRecorder()
	srv.ServeHTTP(recEditBrowser, reqEditBrowser)
	if recEditBrowser.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK, got %d", recEditBrowser.Code)
	}
	assertDoctype(t, recEditBrowser.Body.String(), true)
	docEditBrowser := parseHTML(t, recEditBrowser.Body.String())
	assertElement(t, docEditBrowser, "#frame-editor-view")

	// 4. HTMX GET /bikes/test-bike-1-m/edit (fragment)
	reqEditHtmx := httptest.NewRequest(http.MethodGet, "/bikes/test-bike-1-m/edit", nil)
	reqEditHtmx.Header.Set("HX-Request", "true")
	recEditHtmx := httptest.NewRecorder()
	srv.ServeHTTP(recEditHtmx, reqEditHtmx)
	if recEditHtmx.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK, got %d", recEditHtmx.Code)
	}
	assertDoctype(t, recEditHtmx.Body.String(), false)
	docEditHtmx := parseHTML(t, recEditHtmx.Body.String())
	assertElement(t, docEditHtmx, "#frame-editor-view")

	// 5. Toggle view in editor modal (table vs form)
	reqToggle := httptest.NewRequest(http.MethodGet, "/bikes/modal-toggle-view?view=table&is_new=true", nil)
	recToggle := httptest.NewRecorder()
	srv.ServeHTTP(recToggle, reqToggle)
	if recToggle.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK for toggle-view, got %d", recToggle.Code)
	}
	docToggle := parseHTML(t, recToggle.Body.String())
	assertElement(t, docToggle, "#frame-editor-view")
}

// Test Modal Action Matrix: Add/Remove sizes and configs
func TestModalActionsFullSuite(t *testing.T) {
	srv, _, cleanup := setupTestServer(t)
	defer cleanup()

	baseForm := url.Values{
		"manufacturer":          {"TestBrand"},
		"model":                 {"TestGeo"},
		"sizes":                 {"S", "M", "L"},
		"active_fork_travel_mm": {"140"},
		"reach_S":               {"430"},
		"reach_M":               {"450"},
		"reach_L":               {"475"},
	}

	actions := []struct {
		act          string
		query        string
		findSelector string
	}{
		{"add-size", "", "input[name='sizes'][value='XL']"},
		{"remove-size", "&size=S", ""},
		{"preset", "&preset_type=s1_s6", "input[name='sizes'][value='S6']"},
		{"add-config", "", "button:contains('Build')"},
		{"switch-config", "&config_id=config-1", ""},
		{"kinematic-fill", "", ""},
	}

	for _, a := range actions {
		t.Run("action_"+a.act, func(t *testing.T) {
			form := url.Values{}
			for k, v := range baseForm {
				form[k] = v
			}
			req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/bikes/modal/action?act=%s%s", a.act, a.query), strings.NewReader(form.Encode()))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			rec := httptest.NewRecorder()
			srv.ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("Expected 200 OK for action %s, got %d", a.act, rec.Code)
			}
			if a.findSelector != "" {
				doc := parseHTML(t, rec.Body.String())
				assertElement(t, doc, a.findSelector)
			}
		})
	}
}

// Test Bike Deletion & HX-Location Header
func TestBikeDeletion(t *testing.T) {
	srv, store, cleanup := setupTestServer(t)
	defer cleanup()

	// 1. Delete bike
	reqDel := httptest.NewRequest(http.MethodDelete, "/bikes/test-bike-1-m", nil)
	recDel := httptest.NewRecorder()
	srv.ServeHTTP(recDel, reqDel)

	if recDel.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK on delete, got %d", recDel.Code)
	}
	loc := recDel.Header().Get("HX-Location")
	if loc == "" || !strings.Contains(loc, "/?tab=validator") {
		t.Errorf("Expected HX-Location header to redirect to /?tab=validator, got %q", loc)
	}

	// 2. Verify bike is gone
	bike, _ := store.GetBikeByID("test-bike-1-m")
	if bike != nil {
		t.Errorf("Expected bike to be deleted from store")
	}
}

// Test REST API Compatibility & Validation
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

	// 2. GET /api/bikes/{id}
	reqGetByID := httptest.NewRequest(http.MethodGet, "/api/bikes/test-bike-1-m", nil)
	recGetByID := httptest.NewRecorder()
	srv.ServeHTTP(recGetByID, reqGetByID)
	if recGetByID.Code != http.StatusOK {
		t.Errorf("Expected 200 OK for /api/bikes/test-bike-1-m, got %d", recGetByID.Code)
	}

	reqGet404 := httptest.NewRequest(http.MethodGet, "/api/bikes/invalid-id", nil)
	recGet404 := httptest.NewRecorder()
	srv.ServeHTTP(recGet404, reqGet404)
	if recGet404.Code != http.StatusNotFound {
		t.Errorf("Expected 404 for invalid bike ID, got %d", recGet404.Code)
	}

	// 3. POST /api/validate
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

	// 4. POST /api/canonicalize
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

	// 5. POST /api/analyze
	reqAnalyze := httptest.NewRequest(http.MethodPost, "/api/analyze", bytes.NewReader(valPayload))
	recAnalyze := httptest.NewRecorder()
	srv.ServeHTTP(recAnalyze, reqAnalyze)
	if recAnalyze.Code != http.StatusOK {
		t.Errorf("Expected 200 OK for /api/analyze, got %d", recAnalyze.Code)
	}

	// 6. POST /api/bikes (create new)
	newBike := bikes[0]
	newBike.ID = "api-created-bike"
	newBike.Model = "CreatedViaAPI"
	newPayload, _ := json.Marshal(newBike)
	reqCreate := httptest.NewRequest(http.MethodPost, "/api/bikes", bytes.NewReader(newPayload))
	recCreate := httptest.NewRecorder()
	srv.ServeHTTP(recCreate, reqCreate)
	if recCreate.Code != http.StatusCreated {
		t.Errorf("Expected 201 Created for apiCreateBike, got %d", recCreate.Code)
	}

	// 7. PUT /api/bikes/{id} (update)
	newBike.Reach = 420.0
	upPayload, _ := json.Marshal(newBike)
	reqUpdate := httptest.NewRequest(http.MethodPut, "/api/bikes/api-created-bike", bytes.NewReader(upPayload))
	recUpdate := httptest.NewRecorder()
	srv.ServeHTTP(recUpdate, reqUpdate)
	if recUpdate.Code != http.StatusOK {
		t.Errorf("Expected 200 OK for apiUpdateBike, got %d", recUpdate.Code)
	}

	// 8. DELETE /api/bikes/{id}
	reqDel := httptest.NewRequest(http.MethodDelete, "/api/bikes/api-created-bike", nil)
	recDel := httptest.NewRecorder()
	srv.ServeHTTP(recDel, reqDel)
	if recDel.Code != http.StatusOK {
		t.Errorf("Expected 200 OK for apiDeleteBike, got %d", recDel.Code)
	}

	// 9. DELETE /api/bikes/{id} (non-existent) returns 404
	reqDel404 := httptest.NewRequest(http.MethodDelete, "/api/bikes/nonexistent-id", nil)
	recDel404 := httptest.NewRecorder()
	srv.ServeHTTP(recDel404, reqDel404)
	if recDel404.Code != http.StatusNotFound {
		t.Errorf("Expected 404 Not Found for non-existent bike, got %d", recDel404.Code)
	}
}

func TestMultiSizeGridModalActions(t *testing.T) {
	srv, _, cleanup := setupTestServer(t)
	defer cleanup()

	// 1. Initial modal for new bike contains multi-size grid
	reqNew := httptest.NewRequest(http.MethodGet, "/bikes/new", nil)
	recNew := httptest.NewRecorder()
	srv.ServeHTTP(recNew, reqNew)

	if recNew.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK for /bikes/new, got %d", recNew.Code)
	}
	docNew := parseHTML(t, recNew.Body.String())
	assertElement(t, docNew, "#frame-editor-view")

	// 2. Action: add size column
	formData := url.Values{
		"manufacturer": {"Chumba"},
		"model":        {"Sendero"},
		"sizes":        {"S", "M", "L", "XL"},
		"reach_L":      {"475.0"},
		"stack_L":      {"645.0"},
	}
	reqAddSize := httptest.NewRequest(http.MethodPost, "/bikes/modal/action?act=add-size", strings.NewReader(formData.Encode()))
	reqAddSize.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	recAddSize := httptest.NewRecorder()
	srv.ServeHTTP(recAddSize, reqAddSize)

	if recAddSize.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK for add-size action, got %d", recAddSize.Code)
	}
	docAddSize := parseHTML(t, recAddSize.Body.String())
	assertElement(t, docAddSize, "input[name='sizes'][value='XXL']")

	// 3. Action: add config
	reqAddConfig := httptest.NewRequest(http.MethodPost, "/bikes/modal/action?act=add-config", strings.NewReader(formData.Encode()))
	reqAddConfig.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	recAddConfig := httptest.NewRecorder()
	srv.ServeHTTP(recAddConfig, reqAddConfig)

	if recAddConfig.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK for add-config action, got %d", recAddConfig.Code)
	}
	docAddConfig := parseHTML(t, recAddConfig.Body.String())
	assertElement(t, docAddConfig, "button:contains('Build')")
}

func TestMultiSizeFrameSaveAndValidator(t *testing.T) {
	srv, store, cleanup := setupTestServer(t)
	defer cleanup()

	// 1. Save multi-size frame model
	formData := url.Values{
		"manufacturer":          {"Chumba"},
		"model":                 {"Sendero"},
		"year":                  {"2024"},
		"category":              {"hardtail"},
		"headset_cup_type":      {"EC44"},
		"cup_stack":             {"12"},
		"sizes":                 {"M", "L"},
		"active_config_name":    {"120mm Stock"},
		"active_fork_a2c":       {"531"},
		"active_fork_travel_mm": {"120"},
		"active_fork_rake":      {"44"},
		"reach_M":               {"450"},
		"stack_M":               {"635"},
		"hta_deg_M":             {"67.0"},
		"sta_deg_M":             {"74.5"},
		"htl_M":                 {"110"},
		"bb_drop_M":             {"60"},
		"wheelbase_M":           {"1172"},
		"reach_L":               {"475"},
		"stack_L":               {"645"},
		"hta_deg_L":             {"67.0"},
		"sta_deg_L":             {"74.5"},
		"htl_L":                 {"125"},
		"bb_drop_L":             {"60"},
		"wheelbase_L":           {"1200"},
	}

	reqSave := httptest.NewRequest(http.MethodPost, "/bikes/save?is_new=true", strings.NewReader(formData.Encode()))
	reqSave.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	recSave := httptest.NewRecorder()
	srv.ServeHTTP(recSave, reqSave)

	if recSave.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK for save, got %d", recSave.Code)
	}

	// Verify persistence in store
	frame, err := store.GetFrameByID("chumba-sendero")
	if err != nil || frame == nil {
		t.Fatalf("Expected saved frame in store, got error: %v", err)
	}
	if len(frame.Sizes) != 2 {
		t.Errorf("Expected 2 sizes in frame, got %d", len(frame.Sizes))
	}

	// 2. GET validator tab for this frame
	reqVal := httptest.NewRequest(http.MethodGet, "/?tab=validator&id=chumba-sendero&size=L", nil)
	recVal := httptest.NewRecorder()
	srv.ServeHTTP(recVal, reqVal)

	if recVal.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK for validator tab, got %d", recVal.Code)
	}
	docVal := parseHTML(t, recVal.Body.String())
	if !strings.Contains(docVal.Text(), "Chumba Sendero") {
		t.Errorf("Expected Chumba Sendero in validator tab")
	}
	assertElement(t, docVal, "button:contains('Edit Geometry Grid')")

	// 3. GET /bikes/{id}/edit with size and config params
	reqEditParams := httptest.NewRequest(http.MethodGet, "/bikes/chumba-sendero/edit?size=L", nil)
	recEditParams := httptest.NewRecorder()
	srv.ServeHTTP(recEditParams, reqEditParams)
	if recEditParams.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK for edit with params, got %d", recEditParams.Code)
	}
}

func TestCompareSwitchSize(t *testing.T) {
	srv, store, cleanup := setupTestServer(t)
	defer cleanup()

	// Seed frame with two sizes
	frame := model.FrameModel{
		ID:           "test-switcher",
		Manufacturer: "Switch",
		Model:        "Test",
		Category:     model.CategoryHardtail,
		Sizes:        []string{"M", "L"},
		DefaultSize:  "M",
		Configurations: []model.FrameConfiguration{
			{
				ID:         "stock",
				Name:       "Stock",
				IsBaseline: true,
				Fork:       model.ForkSpec{TravelMm: 120.0, A2c: 530.0, Rake: 44.0},
				Geometries: map[string]model.SizeGeometry{
					"M": {Reach: 440, Stack: 620, HtaDeg: 66, StaDeg: 75, Htl: 110, BbDrop: 60, Wheelbase: 1160, ChainstayLength: 430},
					"L": {Reach: 465, Stack: 630, HtaDeg: 66, StaDeg: 75, Htl: 120, BbDrop: 60, Wheelbase: 1190, ChainstayLength: 430},
				},
			},
		},
	}
	if _, err := store.SaveFrame(frame); err != nil {
		t.Fatalf("Failed to save frame: %v", err)
	}

	form := url.Values{
		"selected_ids": {"test-switcher-m"},
		"to":           {"test-switcher-l"},
	}
	req := httptest.NewRequest(http.MethodPost, "/compare/switch-size?from=test-switcher-m", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK, got %d", rec.Code)
	}
	doc := parseHTML(t, rec.Body.String())
	assertElement(t, doc, "#comparison-matrix")
}

func TestAdditionalHandlerCoverage(t *testing.T) {
	srv, _, cleanup := setupTestServer(t)
	defer cleanup()

	// 1. Index 404 for unknown path
	req404 := httptest.NewRequest(http.MethodGet, "/random-nonexistent-path", nil)
	rec404 := httptest.NewRecorder()
	srv.ServeHTTP(rec404, req404)
	if rec404.Code != http.StatusNotFound {
		t.Errorf("Expected 404 for random path, got %d", rec404.Code)
	}

	// 2. handleBikeEdit 404 for non-existent bike
	reqEdit404 := httptest.NewRequest(http.MethodGet, "/bikes/nonexistent-bike/edit", nil)
	recEdit404 := httptest.NewRecorder()
	srv.ServeHTTP(recEdit404, reqEdit404)
	if recEdit404.Code != http.StatusNotFound {
		t.Errorf("Expected 404 for editing nonexistent bike, got %d", recEdit404.Code)
	}

	// 3. handleInlineEditForm & Display for 'stack' field and 404
	reqFormStack := httptest.NewRequest(http.MethodGet, "/bikes/test-bike-1-m/inline-edit?field=stack", nil)
	recFormStack := httptest.NewRecorder()
	srv.ServeHTTP(recFormStack, reqFormStack)
	if recFormStack.Code != http.StatusOK {
		t.Errorf("Expected 200 OK for stack inline edit, got %d", recFormStack.Code)
	}

	reqForm404 := httptest.NewRequest(http.MethodGet, "/bikes/nonexistent-id/inline-edit?field=stack", nil)
	recForm404 := httptest.NewRecorder()
	srv.ServeHTTP(recForm404, reqForm404)
	if recForm404.Code != http.StatusNotFound {
		t.Errorf("Expected 404 for nonexistent inline form, got %d", recForm404.Code)
	}

	reqDispStack := httptest.NewRequest(http.MethodGet, "/bikes/test-bike-1-m/inline-display?field=stack", nil)
	recDispStack := httptest.NewRecorder()
	srv.ServeHTTP(recDispStack, reqDispStack)
	if recDispStack.Code != http.StatusOK {
		t.Errorf("Expected 200 OK for stack inline display, got %d", recDispStack.Code)
	}

	reqDisp404 := httptest.NewRequest(http.MethodGet, "/bikes/nonexistent-id/inline-display?field=stack", nil)
	recDisp404 := httptest.NewRecorder()
	srv.ServeHTTP(recDisp404, reqDisp404)
	if recDisp404.Code != http.StatusNotFound {
		t.Errorf("Expected 404 for nonexistent inline display, got %d", recDisp404.Code)
	}

	// 4. Save stack field & 404 on save
	stackForm := url.Values{"value": {"660.0"}}
	reqSaveStack := httptest.NewRequest(http.MethodPost, "/bikes/test-bike-1-m/inline-edit?field=stack", strings.NewReader(stackForm.Encode()))
	reqSaveStack.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	recSaveStack := httptest.NewRecorder()
	srv.ServeHTTP(recSaveStack, reqSaveStack)
	if recSaveStack.Code != http.StatusOK {
		t.Errorf("Expected 200 OK for saving stack, got %d", recSaveStack.Code)
	}

	reqSave404 := httptest.NewRequest(http.MethodPost, "/bikes/nonexistent-id/inline-edit?field=stack", strings.NewReader(stackForm.Encode()))
	reqSave404.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	recSave404 := httptest.NewRecorder()
	srv.ServeHTTP(recSave404, reqSave404)
	if recSave404.Code != http.StatusNotFound {
		t.Errorf("Expected 404 for nonexistent inline save, got %d", recSave404.Code)
	}

	// 5. Toggle bike selection on and off
	toggleOff := url.Values{"id": {"test-bike-1-m"}, "selected_ids": {"test-bike-1-m"}}
	reqToggleOff := httptest.NewRequest(http.MethodPost, "/compare/toggle-select", strings.NewReader(toggleOff.Encode()))
	reqToggleOff.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	recToggleOff := httptest.NewRecorder()
	srv.ServeHTTP(recToggleOff, reqToggleOff)
	if recToggleOff.Code != http.StatusOK {
		t.Errorf("Expected 200 OK for toggling off, got %d", recToggleOff.Code)
	}

	// 6. Compare with empty selection (tests fallback in getSelectedBikeIDs)
	reqDefaultSelect := httptest.NewRequest(http.MethodGet, "/?tab=compare", nil)
	recDefaultSelect := httptest.NewRecorder()
	srv.ServeHTTP(recDefaultSelect, reqDefaultSelect)
	if recDefaultSelect.Code != http.StatusOK {
		t.Errorf("Expected 200 OK for default compare selection, got %d", recDefaultSelect.Code)
	}

	// 7. REST API bad payload errors (400 Bad Request)
	badJSON := strings.NewReader("{invalid json")
	for _, endpoint := range []struct {
		method string
		path   string
	}{
		{http.MethodPost, "/api/validate"},
		{http.MethodPost, "/api/canonicalize"},
		{http.MethodPost, "/api/analyze"},
		{http.MethodPost, "/api/bikes"},
		{http.MethodPut, "/api/bikes/test-bike-1-m"},
	} {
		reqBad := httptest.NewRequest(endpoint.method, endpoint.path, badJSON)
		recBad := httptest.NewRecorder()
		srv.ServeHTTP(recBad, reqBad)
		if recBad.Code != http.StatusBadRequest {
			t.Errorf("Expected 400 for bad JSON on %s %s, got %d", endpoint.method, endpoint.path, recBad.Code)
		}
	}

	// 8. Save existing frame (is_new=false)
	updateForm := url.Values{
		"id":               {"test-bike-1"},
		"manufacturer":     {"Binary"},
		"model":            {"Havok Updated"},
		"category":         {"hardtail"},
		"headset_cup_type": {"EC44"},
		"cup_stack":        {"12"},
		"sizes":            {"M"},
		"reach_M":          {"410"},
		"stack_M":          {"650"},
		"hta_deg_M":        {"67.0"},
		"sta_deg_M":        {"74.0"},
		"htl_M":            {"120"},
		"bb_drop_M":        {"65"},
		"wheelbase_M":      {"1160"},
		"fork_a2c":         {"521"},
		"fork_travel_mm":   {"120"},
		"fork_rake":        {"44"},
	}
	reqSaveExisting := httptest.NewRequest(http.MethodPost, "/bikes/save?is_new=false", strings.NewReader(updateForm.Encode()))
	reqSaveExisting.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	recSaveExisting := httptest.NewRecorder()
	srv.ServeHTTP(recSaveExisting, reqSaveExisting)
	if recSaveExisting.Code != http.StatusOK {
		t.Errorf("Expected 200 OK for updating existing frame, got %d", recSaveExisting.Code)
	}

	// 9. Compare diagram toggle & preset variations
	formDiagClear := url.Values{"diagram_bike_id": {""}, "selected_ids": {"test-bike-1-m"}}
	reqDiagClear := httptest.NewRequest(http.MethodPost, "/compare/diagram", strings.NewReader(formDiagClear.Encode()))
	reqDiagClear.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	recDiagClear := httptest.NewRecorder()
	srv.ServeHTTP(recDiagClear, reqDiagClear)

	formPresetOther := url.Values{"preset": {"150mm"}, "selected_ids": {"test-bike-1-m"}}
	reqPresetOther := httptest.NewRequest(http.MethodPost, "/compare/preset", strings.NewReader(formPresetOther.Encode()))
	reqPresetOther.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	recPresetOther := httptest.NewRecorder()
	srv.ServeHTTP(recPresetOther, reqPresetOther)
}

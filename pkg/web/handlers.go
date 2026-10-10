package web

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/a-h/templ"

	"bikegeo/pkg/canonicalizer"
	"bikegeo/pkg/geometry"
	"bikegeo/pkg/model"
	"bikegeo/pkg/storage"
	"bikegeo/pkg/validator"
	"bikegeo/pkg/web/templates"
)

type Server struct {
	store *storage.BikeStore
	mux   *http.ServeMux
}

func NewServer(store *storage.BikeStore) *Server {
	s := &Server{
		store: store,
		mux:   http.NewServeMux(),
	}
	s.routes()
	return s
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}

func isHtmx(r *http.Request) bool {
	return r.Header.Get("HX-Request") == "true"
}

func (s *Server) routes() {
	// Static files
	s.mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))

	// Main web app pages and fragments
	s.mux.HandleFunc("GET /", s.handleIndex)

	// Compare routes
	s.mux.HandleFunc("POST /compare/preset", s.handleComparePreset)
	s.mux.HandleFunc("GET /compare/matrix", s.handleCompareMatrix)
	s.mux.HandleFunc("POST /compare/matrix", s.handleCompareMatrix)
	s.mux.HandleFunc("POST /compare/cup-mode", s.handleCompareCupMode)
	s.mux.HandleFunc("POST /compare/toggle-select", s.handleCompareToggleSelect)
	s.mux.HandleFunc("POST /compare/select-all", s.handleCompareSelectAll)
	s.mux.HandleFunc("POST /compare/clear-all", s.handleCompareClearAll)
	s.mux.HandleFunc("POST /compare/diagram", s.handleCompareDiagram)

	// Validator routes
	s.mux.HandleFunc("GET /validator/sag", s.handleValidatorSag)

	// Inline editing routes (Target Pattern 1)
	s.mux.HandleFunc("GET /bikes/{id}/inline-edit", s.handleInlineEditForm)
	s.mux.HandleFunc("GET /bikes/{id}/inline-display", s.handleInlineDisplay)
	s.mux.HandleFunc("POST /bikes/{id}/inline-edit", s.handleInlineEditSave)

	// Modal and Bike Save/Edit/Delete routes (Target Pattern 3)
	s.mux.HandleFunc("GET /bikes/new", s.handleBikeNew)
	s.mux.HandleFunc("GET /bikes/{id}/edit", s.handleBikeEdit)
	s.mux.HandleFunc("GET /bikes/modal-toggle-view", s.handleModalToggleView)
	s.mux.HandleFunc("POST /bikes/save", s.handleBikeSave)
	s.mux.HandleFunc("DELETE /bikes/{id}", s.handleBikeDelete)

	// Backward-compatible JSON REST API
	s.mux.HandleFunc("GET /api/bikes", s.apiGetBikes)
	s.mux.HandleFunc("GET /api/bikes/{id}", s.apiGetBikeByID)
	s.mux.HandleFunc("POST /api/bikes", s.apiCreateBike)
	s.mux.HandleFunc("PUT /api/bikes/{id}", s.apiUpdateBike)
	s.mux.HandleFunc("DELETE /api/bikes/{id}", s.apiDeleteBike)
	s.mux.HandleFunc("POST /api/validate", s.apiValidate)
	s.mux.HandleFunc("POST /api/canonicalize", s.apiCanonicalize)
	s.mux.HandleFunc("POST /api/analyze", s.apiAnalyze)
}

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	tab := r.URL.Query().Get("tab")
	if tab == "" {
		tab = "compare"
	}

	bikes, err := s.store.GetAllBikes()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var contentComponent templ.Component

	if tab == "validator" {
		inspectedID := r.URL.Query().Get("id")
		var inspectedBike model.BikeGeometry
		if len(bikes) > 0 {
			inspectedBike = bikes[0]
			if inspectedID != "" {
				for _, b := range bikes {
					if b.ID == inspectedID {
						inspectedBike = b
						break
					}
				}
			}
		}

		report := validator.ValidateBikeGeometry(inspectedBike)
		sagPercent := inspectedBike.SagPercentagePublished
		if sagPercent <= 0 {
			sagPercent = 20.0
		}
		forkTravel := inspectedBike.ForkTravelMm
		if forkTravel <= 0 {
			forkTravel = 120.0
		}
		staticForkA2c := inspectedBike.ForkA2c
		if staticForkA2c <= 0 {
			staticForkA2c = report.GeometricA2c
		}
		baseCup := report.CupStack
		simSagMm := (forkTravel * sagPercent) / 100.0
		simForkA2c := staticForkA2c - simSagMm
		simStructuralA2c := simForkA2c + baseCup

		wb := inspectedBike.Wheelbase
		if wb <= 0 {
			wb = 1150.0
		}
		cs := inspectedBike.ChainstayLength
		if cs <= 0 {
			cs = 430.0
		}

		dynPose := geometry.CalculateDynamicGeometry(
			inspectedBike.Reach,
			inspectedBike.Stack,
			inspectedBike.HtaDeg,
			inspectedBike.StaDeg,
			inspectedBike.BbDrop,
			report.TotalStructuralA2c,
			simStructuralA2c,
			wb,
			cs,
		)

		contentComponent = templates.ValidatorView(bikes, inspectedBike, report, sagPercent, dynPose)
	} else {
		// Compare tab
		presetID := "fox34_130"
		staticA2c := 541.0
		travelMm := 130.0
		sagPercent := 20.0
		cupMode := "native"
		seatHeight := 760.0

		var selectedIDs []string
		for i, b := range bikes {
			if i < 4 {
				selectedIDs = append(selectedIDs, b.ID)
			}
		}

		refBikeID := ""
		if len(selectedIDs) > 0 {
			refBikeID = selectedIDs[0]
		}
		diagramBikeID := refBikeID

		rows := s.computeComparisonRows(bikes, selectedIDs, staticA2c, travelMm, sagPercent, cupMode, seatHeight, "")

		contentComponent = templates.CompareView(
			bikes,
			selectedIDs,
			rows,
			presetID,
			staticA2c,
			travelMm,
			sagPercent,
			cupMode,
			seatHeight,
			refBikeID,
			diagramBikeID,
			"",
		)
	}

	if isHtmx(r) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_ = contentComponent.Render(r.Context(), w)
		return
	}

	page := templates.Layout("BikeGeo Analyst - Apples-to-Apples Frame Comparison", tab)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = page.Render(templ.WithChildren(r.Context(), templ.ComponentFunc(func(ctx context.Context, w io.Writer) error {
		if err := templates.Navbar(tab, len(bikes)).Render(ctx, w); err != nil {
			return err
		}
		_, _ = fmt.Fprint(w, `<main id="app-content" class="flex-1 max-w-7xl w-full mx-auto px-4 sm:px-6 lg:px-8 py-6">`)
		if err := contentComponent.Render(ctx, w); err != nil {
			return err
		}
		_, _ = fmt.Fprint(w, `</main>`)
		return templates.Footer().Render(ctx, w)
	})), w)
}

func (s *Server) computeComparisonRows(
	allBikes []model.BikeGeometry,
	selectedIDs []string,
	staticA2c, travelMm, sagPercent float64,
	cupMode string,
	seatHeight float64,
	query string,
) []model.CanonicalComparisonRow {
	var bikesToCompare []model.BikeGeometry
	q := strings.ToLower(strings.TrimSpace(query))

	for _, b := range allBikes {
		isSel := false
		for _, selID := range selectedIDs {
			if selID == b.ID {
				isSel = true
				break
			}
		}
		if !isSel {
			continue
		}

		if q != "" {
			name := strings.ToLower(fmt.Sprintf("%s %s %s", b.Manufacturer, b.Model, b.Size))
			if !strings.Contains(name, q) {
				continue
			}
		}

		bikesToCompare = append(bikesToCompare, b)
	}

	effectiveA2c := canonicalizer.CalculateSaggedA2c(staticA2c, travelMm, sagPercent)

	var rows []model.CanonicalComparisonRow
	for _, b := range bikesToCompare {
		cupStack := b.CupStack
		if cupStack == 0 && b.HeadsetCupType == model.CupEC44 {
			cupStack = 12.0
		}
		if cupMode == "fixed_ec44" {
			cupStack = 12.0
		} else if cupMode == "fixed_zero" {
			cupStack = 0.0
		}

		config := model.CanonicalConfig{
			TargetA2c:  effectiveA2c,
			CupStack:   cupStack,
			SeatHeight: seatHeight,
			SagPercent: sagPercent,
		}

		comparisonRows := canonicalizer.CanonicalizeBikes([]model.BikeGeometry{b}, config)
		if len(comparisonRows) > 0 {
			rows = append(rows, comparisonRows[0])
		}
	}

	return rows
}

func (s *Server) parseBenchmarkParams(r *http.Request) (staticA2c, travelMm, sagPercent float64, cupMode string, seatHeight float64, refBikeID, diagramBikeID, query string) {
	_ = r.ParseForm()

	staticA2c, _ = strconv.ParseFloat(r.FormValue("static_a2c"), 64)
	if staticA2c <= 0 {
		staticA2c = 541.0
	}
	travelMm, _ = strconv.ParseFloat(r.FormValue("travel_mm"), 64)
	if travelMm <= 0 {
		travelMm = 130.0
	}
	sagPercent, _ = strconv.ParseFloat(r.FormValue("sag_percent"), 64)
	if sagPercent < 0 {
		sagPercent = 20.0
	}
	seatHeight, _ = strconv.ParseFloat(r.FormValue("seat_height"), 64)
	if seatHeight <= 0 {
		seatHeight = 760.0
	}
	cupMode = r.FormValue("cup_mode")
	if cupMode == "" {
		cupMode = "native"
	}
	refBikeID = r.FormValue("ref_bike_id")
	diagramBikeID = r.FormValue("diagram_bike_id")
	query = r.FormValue("q")

	return
}

func (s *Server) getSelectedBikeIDs(allBikes []model.BikeGeometry, r *http.Request) []string {
	if r.Form.Has("selected_ids") {
		return r.Form["selected_ids"]
	}
	// Default to first 4 bikes
	var def []string
	for i, b := range allBikes {
		if i < 4 {
			def = append(def, b.ID)
		}
	}
	return def
}

func (s *Server) handleComparePreset(w http.ResponseWriter, r *http.Request) {
	_ = r.ParseForm()
	presetID := r.FormValue("preset_id")
	staticA2c, travelMm, sagPercent, cupMode, seatHeight, refBikeID, diagramBikeID, query := s.parseBenchmarkParams(r)

	for _, p := range canonicalizer.CommonForkPresets {
		if p.ID == presetID {
			staticA2c = p.StaticA2c
			travelMm = p.TravelMm
			sagPercent = p.StandardSagPercent
			break
		}
	}

	bikes, _ := s.store.GetAllBikes()
	selectedIDs := s.getSelectedBikeIDs(bikes, r)
	if refBikeID == "" && len(selectedIDs) > 0 {
		refBikeID = selectedIDs[0]
	}

	rows := s.computeComparisonRows(bikes, selectedIDs, staticA2c, travelMm, sagPercent, cupMode, seatHeight, query)

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = templates.CompareView(bikes, selectedIDs, rows, presetID, staticA2c, travelMm, sagPercent, cupMode, seatHeight, refBikeID, diagramBikeID, query).Render(r.Context(), w)
}

func (s *Server) handleCompareMatrix(w http.ResponseWriter, r *http.Request) {
	staticA2c, travelMm, sagPercent, cupMode, seatHeight, refBikeID, diagramBikeID, query := s.parseBenchmarkParams(r)
	bikes, _ := s.store.GetAllBikes()
	selectedIDs := s.getSelectedBikeIDs(bikes, r)
	if refBikeID == "" && len(selectedIDs) > 0 {
		refBikeID = selectedIDs[0]
	}

	rows := s.computeComparisonRows(bikes, selectedIDs, staticA2c, travelMm, sagPercent, cupMode, seatHeight, query)

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = templates.ComparisonMatrix(rows, refBikeID, diagramBikeID, seatHeight, staticA2c, travelMm, sagPercent).Render(r.Context(), w)
}

func (s *Server) handleCompareCupMode(w http.ResponseWriter, r *http.Request) {
	mode := r.URL.Query().Get("mode")
	if mode == "" {
		mode = "native"
	}
	staticA2c, travelMm, sagPercent, _, seatHeight, refBikeID, diagramBikeID, query := s.parseBenchmarkParams(r)
	presetID := r.FormValue("preset_id")
	if presetID == "" {
		presetID = "fox34_130"
	}

	bikes, _ := s.store.GetAllBikes()
	selectedIDs := s.getSelectedBikeIDs(bikes, r)
	rows := s.computeComparisonRows(bikes, selectedIDs, staticA2c, travelMm, sagPercent, mode, seatHeight, query)

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = templates.CompareView(bikes, selectedIDs, rows, presetID, staticA2c, travelMm, sagPercent, mode, seatHeight, refBikeID, diagramBikeID, query).Render(r.Context(), w)
}

func (s *Server) handleCompareToggleSelect(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	staticA2c, travelMm, sagPercent, cupMode, seatHeight, refBikeID, diagramBikeID, query := s.parseBenchmarkParams(r)
	presetID := r.FormValue("preset_id")
	if presetID == "" {
		presetID = "fox34_130"
	}

	bikes, _ := s.store.GetAllBikes()
	selectedIDs := s.getSelectedBikeIDs(bikes, r)

	found := false
	var updatedIDs []string
	for _, sid := range selectedIDs {
		if sid == id {
			found = true
		} else {
			updatedIDs = append(updatedIDs, sid)
		}
	}
	if !found && id != "" {
		updatedIDs = append(updatedIDs, id)
	}

	rows := s.computeComparisonRows(bikes, updatedIDs, staticA2c, travelMm, sagPercent, cupMode, seatHeight, query)

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = templates.CompareView(bikes, updatedIDs, rows, presetID, staticA2c, travelMm, sagPercent, cupMode, seatHeight, refBikeID, diagramBikeID, query).Render(r.Context(), w)
}

func (s *Server) handleCompareSelectAll(w http.ResponseWriter, r *http.Request) {
	staticA2c, travelMm, sagPercent, cupMode, seatHeight, refBikeID, diagramBikeID, query := s.parseBenchmarkParams(r)
	presetID := r.FormValue("preset_id")
	if presetID == "" {
		presetID = "fox34_130"
	}

	bikes, _ := s.store.GetAllBikes()
	var allIDs []string
	for _, b := range bikes {
		allIDs = append(allIDs, b.ID)
	}

	rows := s.computeComparisonRows(bikes, allIDs, staticA2c, travelMm, sagPercent, cupMode, seatHeight, query)

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = templates.CompareView(bikes, allIDs, rows, presetID, staticA2c, travelMm, sagPercent, cupMode, seatHeight, refBikeID, diagramBikeID, query).Render(r.Context(), w)
}

func (s *Server) handleCompareClearAll(w http.ResponseWriter, r *http.Request) {
	staticA2c, travelMm, sagPercent, cupMode, seatHeight, refBikeID, diagramBikeID, query := s.parseBenchmarkParams(r)
	presetID := r.FormValue("preset_id")
	if presetID == "" {
		presetID = "fox34_130"
	}

	bikes, _ := s.store.GetAllBikes()
	var keptIDs []string
	if refBikeID != "" {
		keptIDs = []string{refBikeID}
	} else if len(bikes) > 0 {
		keptIDs = []string{bikes[0].ID}
	}

	rows := s.computeComparisonRows(bikes, keptIDs, staticA2c, travelMm, sagPercent, cupMode, seatHeight, query)

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = templates.CompareView(bikes, keptIDs, rows, presetID, staticA2c, travelMm, sagPercent, cupMode, seatHeight, refBikeID, diagramBikeID, query).Render(r.Context(), w)
}

func (s *Server) handleCompareDiagram(w http.ResponseWriter, r *http.Request) {
	diagramBikeID := r.FormValue("diagram_bike_id")
	staticA2c, travelMm, sagPercent, cupMode, seatHeight, _, _, _ := s.parseBenchmarkParams(r)

	bike, err := s.store.GetBikeByID(diagramBikeID)
	if err != nil || bike == nil {
		http.Error(w, "Bike not found", http.StatusNotFound)
		return
	}

	effectiveA2c := canonicalizer.CalculateSaggedA2c(staticA2c, travelMm, sagPercent)
	cupStack := bike.CupStack
	if cupStack == 0 && bike.HeadsetCupType == model.CupEC44 {
		cupStack = 12.0
	}
	if cupMode == "fixed_ec44" {
		cupStack = 12.0
	} else if cupMode == "fixed_zero" {
		cupStack = 0.0
	}

	config := model.CanonicalConfig{
		TargetA2c:  effectiveA2c,
		CupStack:   cupStack,
		SeatHeight: seatHeight,
		SagPercent: sagPercent,
	}

	rows := canonicalizer.CanonicalizeBikes([]model.BikeGeometry{*bike}, config)
	if len(rows) == 0 {
		http.Error(w, "Analysis failed", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = templates.BikeDiagram(*bike, &rows[0].Canonical, seatHeight, true).Render(r.Context(), w)
}

func (s *Server) handleValidatorSag(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	sagStr := r.URL.Query().Get("sag")
	sagPercent, _ := strconv.ParseFloat(sagStr, 64)
	if sagPercent < 0 {
		sagPercent = 20.0
	}

	bike, err := s.store.GetBikeByID(id)
	if err != nil || bike == nil {
		http.Error(w, "Bike not found", http.StatusNotFound)
		return
	}

	report := validator.ValidateBikeGeometry(*bike)
	travel := bike.ForkTravelMm
	if travel <= 0 {
		travel = 120.0
	}
	staticForkA2c := bike.ForkA2c
	if staticForkA2c <= 0 {
		staticForkA2c = report.GeometricA2c
	}
	cup := report.CupStack
	simSagMm := (travel * sagPercent) / 100.0
	simForkA2c := staticForkA2c - simSagMm
	simStructuralA2c := simForkA2c + cup

	wb := bike.Wheelbase
	if wb <= 0 {
		wb = 1150.0
	}
	cs := bike.ChainstayLength
	if cs <= 0 {
		cs = 430.0
	}

	dynPose := geometry.CalculateDynamicGeometry(
		bike.Reach,
		bike.Stack,
		bike.HtaDeg,
		bike.StaDeg,
		bike.BbDrop,
		report.TotalStructuralA2c,
		simStructuralA2c,
		wb,
		cs,
	)

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = templates.SagSimulatorResults(*bike, sagPercent, dynPose).Render(r.Context(), w)
}

// Target Pattern 1: Inline Edit cell handlers
func (s *Server) handleInlineEditForm(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	field := r.URL.Query().Get("field")

	bike, err := s.store.GetBikeByID(id)
	if err != nil || bike == nil {
		http.Error(w, "Bike not found", http.StatusNotFound)
		return
	}

	valStr := ""
	colorClass := "text-white"
	switch field {
	case "reach":
		valStr = fmt.Sprintf("%.1f", bike.Reach)
		colorClass = "text-cyan-300"
	case "stack":
		valStr = fmt.Sprintf("%.1f", bike.Stack)
		colorClass = "text-amber-300"
	default:
		valStr = "0"
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = templates.InlineCellEdit(bike.ID, field, valStr, "mm", colorClass).Render(r.Context(), w)
}

func (s *Server) handleInlineDisplay(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	field := r.URL.Query().Get("field")

	bike, err := s.store.GetBikeByID(id)
	if err != nil || bike == nil {
		http.Error(w, "Bike not found", http.StatusNotFound)
		return
	}

	valStr := ""
	colorClass := "text-white"
	switch field {
	case "reach":
		valStr = fmt.Sprintf("%.1f", bike.Reach)
		colorClass = "text-cyan-300"
	case "stack":
		valStr = fmt.Sprintf("%.1f", bike.Stack)
		colorClass = "text-amber-300"
	default:
		valStr = "0"
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = templates.InlineCellDisplay(bike.ID, field, valStr, "mm", colorClass).Render(r.Context(), w)
}

func (s *Server) handleInlineEditSave(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	field := r.URL.Query().Get("field")
	_ = r.ParseForm()
	valStr := r.FormValue("value")

	newVal, err := strconv.ParseFloat(valStr, 64)
	if err != nil || newVal <= 0 {
		http.Error(w, "Invalid number", http.StatusBadRequest)
		return
	}

	bike, err := s.store.GetBikeByID(id)
	if err != nil || bike == nil {
		http.Error(w, "Bike not found", http.StatusNotFound)
		return
	}

	colorClass := "text-white"
	switch field {
	case "reach":
		bike.Reach = newVal
		colorClass = "text-cyan-300"
	case "stack":
		bike.Stack = newVal
		colorClass = "text-amber-300"
	}

	if _, err := s.store.SaveBike(*bike); err != nil {
		http.Error(w, "Save failed", http.StatusInternalServerError)
		return
	}

	// Trigger matrix refresh on other cells if needed
	w.Header().Set("HX-Trigger", "refreshMatrix")
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = templates.InlineCellDisplay(bike.ID, field, fmt.Sprintf("%.1f", newVal), "mm", colorClass).Render(r.Context(), w)
}

// Target Pattern 3: Bike Add/Edit Modal & Async Save
func (s *Server) handleBikeNew(w http.ResponseWriter, r *http.Request) {
	newBike := model.BikeGeometry{
		Size:           "M",
		Category:       model.CategoryHardtail,
		Reach:          450,
		Stack:          630,
		HtaDeg:         66.0,
		StaDeg:         75.0,
		Htl:            110,
		BbDrop:         60,
		ForkRake:       44,
		Wheelbase:      1180,
		ForkTravelMm:   130,
		ForkA2c:        541,
		HeadsetCupType: model.CupZS44,
		CupStack:       2,
	}

	report := validator.ValidateBikeGeometry(newBike)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = templates.EditorModal(newBike, true, &report, "", "form").Render(r.Context(), w)
}

func (s *Server) handleBikeEdit(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	bike, err := s.store.GetBikeByID(id)
	if err != nil || bike == nil {
		http.Error(w, "Bike not found", http.StatusNotFound)
		return
	}

	report := validator.ValidateBikeGeometry(*bike)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = templates.EditorModal(*bike, false, &report, "", "form").Render(r.Context(), w)
}

func (s *Server) handleModalToggleView(w http.ResponseWriter, r *http.Request) {
	isNew := r.URL.Query().Get("is_new") == "true"
	mode := r.URL.Query().Get("mode")
	if mode == "" {
		mode = "form"
	}

	bike := s.parseBikeFromForm(r)
	report := validator.ValidateBikeGeometry(bike)

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = templates.EditorModal(bike, isNew, &report, "", mode).Render(r.Context(), w)
}

func (s *Server) parseBikeFromForm(r *http.Request) model.BikeGeometry {
	_ = r.ParseForm()

	rawJSON := r.FormValue("raw_json")
	if rawJSON != "" {
		var parsed model.BikeGeometry
		if err := json.Unmarshal([]byte(rawJSON), &parsed); err == nil {
			return parsed
		}
	}

	reach, _ := strconv.ParseFloat(r.FormValue("reach"), 64)
	stack, _ := strconv.ParseFloat(r.FormValue("stack"), 64)
	hta, _ := strconv.ParseFloat(r.FormValue("hta_deg"), 64)
	sta, _ := strconv.ParseFloat(r.FormValue("sta_deg"), 64)
	htl, _ := strconv.ParseFloat(r.FormValue("htl"), 64)
	bbDrop, _ := strconv.ParseFloat(r.FormValue("bb_drop"), 64)
	rake, _ := strconv.ParseFloat(r.FormValue("fork_rake"), 64)
	wb, _ := strconv.ParseFloat(r.FormValue("wheelbase"), 64)
	a2c, _ := strconv.ParseFloat(r.FormValue("fork_a2c"), 64)
	travel, _ := strconv.ParseFloat(r.FormValue("fork_travel_mm"), 64)
	cup, _ := strconv.ParseFloat(r.FormValue("cup_stack"), 64)

	cupType := model.HeadsetCupType(r.FormValue("headset_cup_type"))
	if cupType == model.CupEC44 && cup == 0 {
		cup = 12.0
	}

	return model.BikeGeometry{
		ID:             r.FormValue("id"),
		Manufacturer:   strings.TrimSpace(r.FormValue("manufacturer")),
		Model:          strings.TrimSpace(r.FormValue("model")),
		Size:           strings.TrimSpace(r.FormValue("size")),
		Category:       model.BikeCategory(r.FormValue("category")),
		Notes:          strings.TrimSpace(r.FormValue("notes")),
		Reach:          reach,
		Stack:          stack,
		HtaDeg:         hta,
		StaDeg:         sta,
		Htl:            htl,
		BbDrop:         bbDrop,
		ForkRake:       rake,
		Wheelbase:      wb,
		ForkA2c:        a2c,
		ForkTravelMm:   travel,
		HeadsetCupType: cupType,
		CupStack:       cup,
	}
}

// Target Pattern 3: Asynchronous form submission with loading indicator and inline validation errors
func (s *Server) handleBikeSave(w http.ResponseWriter, r *http.Request) {
	isNew := r.URL.Query().Get("is_new") == "true"
	bike := s.parseBikeFromForm(r)

	// Validate bike geometry
	report := validator.ValidateBikeGeometry(bike)

	// If fatal error exists (e.g. missing essential dimensions or empty manufacturer/model/size)
	if !report.IsValid || bike.Manufacturer == "" || bike.Model == "" || bike.Size == "" {
		if bike.Manufacturer == "" {
			report.Issues = append(report.Issues, model.ValidationIssue{
				Field:    "manufacturer",
				Severity: model.SeverityError,
				Title:    "Missing Manufacturer",
				Message:  "Manufacturer name is required.",
			})
		}
		if bike.Model == "" {
			report.Issues = append(report.Issues, model.ValidationIssue{
				Field:    "model",
				Severity: model.SeverityError,
				Title:    "Missing Model",
				Message:  "Model name is required.",
			})
		}
		if bike.Size == "" {
			report.Issues = append(report.Issues, model.ValidationIssue{
				Field:    "size",
				Severity: model.SeverityError,
				Title:    "Missing Size",
				Message:  "Frame size is required.",
			})
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_ = templates.EditorModal(bike, isNew, &report, "Please resolve the validation errors highlighted below.", "form").Render(r.Context(), w)
		return
	}

	saved, err := s.store.SaveBike(bike)
	if err != nil {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_ = templates.EditorModal(bike, isNew, &report, fmt.Sprintf("Storage error: %v", err), "form").Render(r.Context(), w)
		return
	}

	// Successfully saved: close modal and trigger full reload or swap
	w.Header().Set("HX-Location", fmt.Sprintf("/?tab=validator&id=%s", saved.ID))
	w.WriteHeader(http.StatusOK)
}

func (s *Server) handleBikeDelete(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	_, _ = s.store.DeleteBike(id)

	w.Header().Set("HX-Location", "/?tab=validator")
	w.WriteHeader(http.StatusOK)
}

// REST API Handlers
func (s *Server) apiGetBikes(w http.ResponseWriter, r *http.Request) {
	bikes, err := s.store.GetAllBikes()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, bikes)
}

func (s *Server) apiGetBikeByID(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	bike, err := s.store.GetBikeByID(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if bike == nil {
		http.Error(w, "Bike not found", http.StatusNotFound)
		return
	}
	writeJSON(w, http.StatusOK, bike)
}

func (s *Server) apiCreateBike(w http.ResponseWriter, r *http.Request) {
	var bike model.BikeGeometry
	if err := json.NewDecoder(r.Body).Decode(&bike); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	saved, err := s.store.SaveBike(bike)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusCreated, saved)
}

func (s *Server) apiUpdateBike(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var bike model.BikeGeometry
	if err := json.NewDecoder(r.Body).Decode(&bike); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	bike.ID = id

	saved, err := s.store.SaveBike(bike)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, saved)
}

func (s *Server) apiDeleteBike(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	deleted, err := s.store.DeleteBike(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if !deleted {
		http.Error(w, "Bike not found", http.StatusNotFound)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"success": true, "id": id})
}

func (s *Server) apiValidate(w http.ResponseWriter, r *http.Request) {
	var bike model.BikeGeometry
	if err := json.NewDecoder(r.Body).Decode(&bike); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	report := validator.ValidateBikeGeometry(bike)
	writeJSON(w, http.StatusOK, report)
}

func (s *Server) apiCanonicalize(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Bikes  []model.BikeGeometry  `json:"bikes"`
		Config model.CanonicalConfig `json:"config"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	result := canonicalizer.CanonicalizeBikes(req.Bikes, req.Config)
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) apiAnalyze(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Bike           model.BikeGeometry `json:"bike"`
		TargetA2c      *float64           `json:"targetA2c"`
		TargetCupStack *float64           `json:"targetCupStack"`
		SeatHeight     *float64           `json:"seatHeight"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	result := geometry.AnalyzeBikeFit(req.Bike, req.TargetA2c, req.TargetCupStack, req.SeatHeight)
	writeJSON(w, http.StatusOK, result)
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

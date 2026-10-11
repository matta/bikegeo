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
	s.mux.HandleFunc("GET /favicon.ico", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusNoContent) })

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
	s.mux.HandleFunc("POST /compare/switch-size", s.handleCompareSwitchSize)
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
	s.mux.HandleFunc("POST /bikes/modal/action", s.handleModalAction)
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

	frames, err := s.store.GetAllFrames()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	bikes, _ := s.store.GetAllBikes()

	var contentComponent templ.Component

	if tab == "validator" {
		inspectedID := r.URL.Query().Get("id")
		sizeParam := r.URL.Query().Get("size")
		configParam := r.URL.Query().Get("config")

		var inspectedFrame *model.FrameModel
		if len(frames) > 0 {
			inspectedFrame = &frames[0]
			if inspectedID != "" {
				for i := range frames {
					if frames[i].ID == inspectedID || strings.HasPrefix(inspectedID, frames[i].ID+"-") {
						inspectedFrame = &frames[i]
						break
					}
				}
			}
		}

		activeSize := sizeParam
		if activeSize == "" && inspectedFrame != nil {
			activeSize = inspectedFrame.GetDefaultSize()
		}
		activeConfigID := configParam
		if activeConfigID == "" && inspectedFrame != nil {
			baseCfg := inspectedFrame.GetBaselineConfiguration()
			if baseCfg != nil {
				activeConfigID = baseCfg.ID
			}
		}

		var inspectedBike model.BikeGeometry
		if inspectedFrame != nil {
			inspectedBike, _ = geometry.MaterializeBikeGeometry(*inspectedFrame, activeSize, activeConfigID)
		} else if len(bikes) > 0 {
			inspectedBike = bikes[0]
		}

		var crossVal *model.CrossConfigValidation
		if inspectedFrame != nil && len(inspectedFrame.Configurations) > 1 {
			compID := activeConfigID
			baseCfg := inspectedFrame.GetBaselineConfiguration()
			if baseCfg != nil && compID == baseCfg.ID {
				for _, cfg := range inspectedFrame.Configurations {
					if cfg.ID != baseCfg.ID {
						compID = cfg.ID
						break
					}
				}
			}
			crossVal = validator.ValidateCrossConfig(*inspectedFrame, activeSize, compID)
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

		contentComponent = templates.ValidatorView(frames, inspectedFrame, inspectedBike, activeSize, activeConfigID, crossVal, report, sagPercent, dynPose)
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

	s.renderPageOrFragment(w, r, tab, contentComponent)
}

func (s *Server) renderPageOrFragment(w http.ResponseWriter, r *http.Request, tab string, contentComponent templ.Component) {
	bikes, _ := s.store.GetAllBikes()
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if isHtmx(r) {
		if err := templates.NavbarOOB(tab, len(bikes)).Render(r.Context(), w); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if err := contentComponent.Render(r.Context(), w); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		return
	}

	page := templates.Layout("BikeGeo Analyst - Apples-to-Apples Frame Comparison", tab)
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
			if selID == b.ID || strings.TrimSuffix(b.ID, "-"+strings.ToLower(b.Size)) == selID || strings.HasPrefix(b.ID, selID+"-") {
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
	_ = templates.ComparisonMatrix(rows, bikes, refBikeID, diagramBikeID, seatHeight, staticA2c, travelMm, sagPercent).Render(r.Context(), w)
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

func (s *Server) handleCompareSwitchSize(w http.ResponseWriter, r *http.Request) {
	_ = r.ParseForm()
	fromID := r.URL.Query().Get("from")
	toID := r.FormValue("to")
	if toID == "" {
		toID = r.URL.Query().Get("to")
	}

	staticA2c, travelMm, sagPercent, cupMode, seatHeight, refBikeID, diagramBikeID, query := s.parseBenchmarkParams(r)
	presetID := r.FormValue("preset_id")
	if presetID == "" {
		presetID = "fox34_130"
	}

	bikes, _ := s.store.GetAllBikes()
	selectedIDs := s.getSelectedBikeIDs(bikes, r)

	var updatedIDs []string
	for _, sid := range selectedIDs {
		if sid == fromID {
			if toID != "" {
				updatedIDs = append(updatedIDs, toID)
			}
		} else {
			updatedIDs = append(updatedIDs, sid)
		}
	}

	if refBikeID == fromID {
		refBikeID = toID
	}
	if diagramBikeID == fromID {
		diagramBikeID = toID
	}

	rows := s.computeComparisonRows(bikes, updatedIDs, staticA2c, travelMm, sagPercent, cupMode, seatHeight, query)

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = templates.CompareView(bikes, updatedIDs, rows, presetID, staticA2c, travelMm, sagPercent, cupMode, seatHeight, refBikeID, diagramBikeID, query).Render(r.Context(), w)
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
	newFrame := model.FrameModel{
		Year:           2024,
		Category:       model.CategoryHardtail,
		HeadsetCupType: model.CupEC44,
		CupStack:       12.0,
		Sizes:          []string{"S", "M", "L", "XL"},
		DefaultSize:    "L",
		Configurations: []model.FrameConfiguration{
			{
				ID:         "stock",
				Name:       "Stock Build",
				IsBaseline: true,
				Fork:       model.ForkSpec{ID: "fox34_130", Name: "Fox Float 34 130mm", TravelMm: 130, A2c: 541, Rake: 44},
				Geometries: map[string]model.SizeGeometry{
					"S":  {Reach: 425, Stack: 615, HtaDeg: 65.0, StaDeg: 74.5, Htl: 95, BbDrop: 55, Wheelbase: 1160},
					"M":  {Reach: 445, Stack: 625, HtaDeg: 65.0, StaDeg: 74.5, Htl: 105, BbDrop: 55, Wheelbase: 1185},
					"L":  {Reach: 465, Stack: 635, HtaDeg: 65.0, StaDeg: 74.5, Htl: 115, BbDrop: 55, Wheelbase: 1210},
					"XL": {Reach: 490, Stack: 645, HtaDeg: 65.0, StaDeg: 74.5, Htl: 130, BbDrop: 55, Wheelbase: 1240},
				},
			},
		},
	}

	matBike, _ := geometry.MaterializeBikeGeometry(newFrame, newFrame.GetDefaultSize(), "")
	report := validator.ValidateBikeGeometry(matBike)
	comp := templates.EditorModal(newFrame, true, &report, "", "form", "stock")
	s.renderPageOrFragment(w, r, "validator", comp)
}

func (s *Server) handleBikeEdit(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	frame, err := s.store.GetFrameByID(id)
	if err != nil || frame == nil {
		bike, bErr := s.store.GetBikeByID(id)
		if bErr != nil || bike == nil {
			http.Error(w, "Bike or frame not found", http.StatusNotFound)
			return
		}
		f := model.FrameModel{
			ID:             id,
			Manufacturer:   bike.Manufacturer,
			Model:          bike.Model,
			Year:           bike.Year,
			Category:       bike.Category,
			Notes:          bike.Notes,
			WheelSize:      bike.WheelSize,
			HeadsetCupType: bike.HeadsetCupType,
			CupStack:       bike.CupStack,
			Sizes:          []string{bike.Size},
			DefaultSize:    bike.Size,
			Configurations: []model.FrameConfiguration{
				{
					ID:         "stock",
					Name:       "Stock Build",
					IsBaseline: true,
					Fork: func() model.ForkSpec {
						if bike.Fork.A2c > 0 || bike.Fork.Rake > 0 {
							return bike.Fork
						}
						return model.ForkSpec{
							TravelMm: bike.ForkTravelMm,
							A2c:      bike.ForkA2c,
							Rake:     bike.ForkRake,
						}
					}(),
					Geometries: map[string]model.SizeGeometry{
						bike.Size: {
							Reach:           bike.Reach,
							Stack:           bike.Stack,
							HtaDeg:          bike.HtaDeg,
							StaDeg:          bike.StaDeg,
							Htl:             bike.Htl,
							BbDrop:          bike.BbDrop,
							Wheelbase:       bike.Wheelbase,
							ChainstayLength: bike.ChainstayLength,
							SeatTubeLength:  bike.SeatTubeLength,
						},
					},
				},
			},
		}
		frame = &f
	}

	matBike, _ := geometry.MaterializeBikeGeometry(*frame, frame.GetDefaultSize(), "")
	report := validator.ValidateBikeGeometry(matBike)
	comp := templates.EditorModal(*frame, false, &report, "", "form", frame.GetBaselineConfiguration().ID)
	s.renderPageOrFragment(w, r, "validator", comp)
}

func (s *Server) handleModalToggleView(w http.ResponseWriter, r *http.Request) {
	isNew := r.URL.Query().Get("is_new") == "true"
	mode := r.URL.Query().Get("mode")
	if mode == "" {
		mode = "form"
	}

	frame, activeConfigID := s.parseFrameFromForm(r)
	matBike, _ := geometry.MaterializeBikeGeometry(frame, frame.GetDefaultSize(), activeConfigID)
	report := validator.ValidateBikeGeometry(matBike)

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = templates.EditorModal(frame, isNew, &report, "", mode, activeConfigID).Render(r.Context(), w)
}

func (s *Server) handleModalAction(w http.ResponseWriter, r *http.Request) {
	isNew := r.URL.Query().Get("is_new") == "true"
	act := r.URL.Query().Get("act")
	frame, activeConfigID := s.parseFrameFromForm(r)

	switch act {
	case "add-size":
		newSizeName := fmt.Sprintf("S%d", len(frame.Sizes)+1)
		if len(frame.Sizes) == 0 {
			newSizeName = "M"
		} else {
			last := frame.Sizes[len(frame.Sizes)-1]
			switch last {
			case "S":
				newSizeName = "M"
			case "M":
				newSizeName = "L"
			case "L":
				newSizeName = "XL"
			case "XL":
				newSizeName = "XXL"
			}
		}
		frame.Sizes = append(frame.Sizes, newSizeName)
		for i := range frame.Configurations {
			if frame.Configurations[i].Geometries == nil {
				frame.Configurations[i].Geometries = make(map[string]model.SizeGeometry)
			}
			if len(frame.Sizes) > 1 {
				prevSz := frame.Sizes[len(frame.Sizes)-2]
				prevGeo := frame.Configurations[i].Geometries[prevSz]
				frame.Configurations[i].Geometries[newSizeName] = prevGeo
			}
		}

	case "remove-size":
		rmSz := r.URL.Query().Get("size")
		if len(frame.Sizes) > 1 {
			var updatedSizes []string
			for _, sz := range frame.Sizes {
				if sz != rmSz {
					updatedSizes = append(updatedSizes, sz)
				}
			}
			frame.Sizes = updatedSizes
			for i := range frame.Configurations {
				delete(frame.Configurations[i].Geometries, rmSz)
			}
		}

	case "preset":
		presetType := r.URL.Query().Get("preset_type")
		switch presetType {
		case "s_xl":
			frame.Sizes = []string{"S", "M", "L", "XL"}
		case "s1_s6":
			frame.Sizes = []string{"S1", "S2", "S3", "S4", "S5", "S6"}
		case "cm":
			frame.Sizes = []string{"49cm", "52cm", "54cm", "56cm", "58cm", "61cm"}
		}

	case "add-config":
		newConfigID := fmt.Sprintf("config-%d", len(frame.Configurations)+1)
		baseCfg := frame.GetBaselineConfiguration()
		travel := 140.0
		a2c := 551.0
		rake := 44.0
		if baseCfg != nil {
			travel = baseCfg.Fork.TravelMm + 10.0
			a2c = baseCfg.Fork.A2c + 10.0
			rake = baseCfg.Fork.Rake
		}
		frame.Configurations = append(frame.Configurations, model.FrameConfiguration{
			ID:         newConfigID,
			Name:       fmt.Sprintf("%.0fmm Build", travel),
			IsBaseline: false,
			Fork: model.ForkSpec{
				TravelMm: travel,
				A2c:      a2c,
				Rake:     rake,
			},
			Geometries: make(map[string]model.SizeGeometry),
		})
		activeConfigID = newConfigID

	case "switch-config":
		cfgID := r.URL.Query().Get("config_id")
		if cfgID != "" {
			activeConfigID = cfgID
		}

	case "kinematic-fill":
		cfg := frame.GetConfiguration(activeConfigID)
		baseCfg := frame.GetBaselineConfiguration()
		if cfg != nil && baseCfg != nil && !cfg.IsBaseline {
			if cfg.Geometries == nil {
				cfg.Geometries = make(map[string]model.SizeGeometry)
			}
			for _, sz := range frame.Sizes {
				b, err := geometry.MaterializeBikeGeometry(frame, sz, cfg.ID)
				if err == nil {
					cfg.Geometries[sz] = model.SizeGeometry{
						Reach:           b.Reach,
						Stack:           b.Stack,
						HtaDeg:          b.HtaDeg,
						StaDeg:          b.StaDeg,
						Htl:             b.Htl,
						BbDrop:          b.BbDrop,
						Wheelbase:       b.Wheelbase,
						ChainstayLength: b.ChainstayLength,
						SeatTubeLength:  b.SeatTubeLength,
					}
				}
			}
		}
	}

	matBike, _ := geometry.MaterializeBikeGeometry(frame, frame.GetDefaultSize(), activeConfigID)
	report := validator.ValidateBikeGeometry(matBike)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = templates.EditorModal(frame, isNew, &report, "", "form", activeConfigID).Render(r.Context(), w)
}

func (s *Server) parseFrameFromForm(r *http.Request) (model.FrameModel, string) {
	_ = r.ParseForm()

	rawJSON := r.FormValue("raw_json")
	if rawJSON != "" {
		var parsed model.FrameModel
		if err := json.Unmarshal([]byte(rawJSON), &parsed); err == nil && len(parsed.Configurations) > 0 {
			activeID := "stock"
			if base := parsed.GetBaselineConfiguration(); base != nil {
				activeID = base.ID
			}
			return parsed, activeID
		}
		var parsedBike model.BikeGeometry
		if err := json.Unmarshal([]byte(rawJSON), &parsedBike); err == nil && parsedBike.Manufacturer != "" {
			sz := parsedBike.Size
			if sz == "" {
				sz = "M"
			}
			fm := model.FrameModel{
				ID:             parsedBike.ID,
				Manufacturer:   parsedBike.Manufacturer,
				Model:          parsedBike.Model,
				Year:           parsedBike.Year,
				Category:       parsedBike.Category,
				Notes:          parsedBike.Notes,
				WheelSize:      parsedBike.WheelSize,
				HeadsetCupType: parsedBike.HeadsetCupType,
				CupStack:       parsedBike.CupStack,
				Sizes:          []string{sz},
				DefaultSize:    sz,
				Configurations: []model.FrameConfiguration{
					{
						ID:         "stock",
						Name:       "Stock Build",
						IsBaseline: true,
						Fork: func() model.ForkSpec {
							if parsedBike.Fork.A2c > 0 || parsedBike.Fork.Rake > 0 {
								return parsedBike.Fork
							}
							return model.ForkSpec{
								TravelMm: parsedBike.ForkTravelMm,
								A2c:      parsedBike.ForkA2c,
								Rake:     parsedBike.ForkRake,
							}
						}(),
						Geometries: map[string]model.SizeGeometry{
							sz: {
								Reach:           parsedBike.Reach,
								Stack:           parsedBike.Stack,
								HtaDeg:          parsedBike.HtaDeg,
								StaDeg:          parsedBike.StaDeg,
								Htl:             parsedBike.Htl,
								BbDrop:          parsedBike.BbDrop,
								Wheelbase:       parsedBike.Wheelbase,
								ChainstayLength: parsedBike.ChainstayLength,
								SeatTubeLength:  parsedBike.SeatTubeLength,
							},
						},
					},
				},
			}
			return fm, "stock"
		}
	}

	frameID := r.FormValue("id")
	manufacturer := strings.TrimSpace(r.FormValue("manufacturer"))
	modelName := strings.TrimSpace(r.FormValue("model"))
	year, _ := strconv.Atoi(r.FormValue("year"))
	category := model.BikeCategory(r.FormValue("category"))
	wheelSize := strings.TrimSpace(r.FormValue("wheel_size"))
	cupType := model.HeadsetCupType(r.FormValue("headset_cup_type"))
	cupStack, _ := strconv.ParseFloat(r.FormValue("cup_stack"), 64)
	if cupType == model.CupEC44 && cupStack == 0 {
		cupStack = 12.0
	}
	notes := strings.TrimSpace(r.FormValue("notes"))

	sizes := r.Form["sizes"]
	if len(sizes) == 0 {
		singleSize := r.FormValue("size")
		if singleSize != "" {
			sizes = []string{singleSize}
		} else {
			sizes = []string{"M"}
		}
	}

	activeConfigID := r.FormValue("active_config_id")
	if activeConfigID == "" {
		activeConfigID = "stock"
	}

	existing, _ := s.store.GetFrameByID(frameID)
	var configs []model.FrameConfiguration
	if existing != nil && len(existing.Configurations) > 0 {
		configs = existing.Configurations
	}

	activeName := r.FormValue("active_config_name")
	if activeName == "" {
		activeName = "Stock Build"
	}
	activePreset := strings.TrimSpace(r.FormValue("active_fork_preset"))
	activeForkName := strings.TrimSpace(r.FormValue("active_fork_name"))
	activeA2c, _ := strconv.ParseFloat(r.FormValue("active_fork_a2c"), 64)
	if activeA2c <= 0 {
		activeA2c, _ = strconv.ParseFloat(r.FormValue("fork_a2c"), 64)
	}
	if activeA2c <= 0 {
		activeA2c = 541.0
	}
	activeTravel, _ := strconv.ParseFloat(r.FormValue("active_fork_travel_mm"), 64)
	if activeTravel <= 0 {
		activeTravel, _ = strconv.ParseFloat(r.FormValue("fork_travel_mm"), 64)
	}
	activeRake, _ := strconv.ParseFloat(r.FormValue("active_fork_rake"), 64)
	if activeRake <= 0 {
		activeRake, _ = strconv.ParseFloat(r.FormValue("fork_rake"), 64)
	}
	if activeRake <= 0 {
		activeRake = 44.0
	}
	activeSag, _ := strconv.ParseFloat(r.FormValue("active_sag_published"), 64)

	forkSpec := model.ForkSpec{
		ID:       activePreset,
		Name:     activeForkName,
		TravelMm: activeTravel,
		A2c:      activeA2c,
		Rake:     activeRake,
	}
	forkSpec = canonicalizer.ResolveForkSpec(forkSpec)

	activeGeos := make(map[string]model.SizeGeometry)
	for _, sz := range sizes {
		reach, _ := strconv.ParseFloat(r.FormValue("reach_"+sz), 64)
		if reach == 0 {
			reach, _ = strconv.ParseFloat(r.FormValue("reach"), 64)
		}
		stack, _ := strconv.ParseFloat(r.FormValue("stack_"+sz), 64)
		if stack == 0 {
			stack, _ = strconv.ParseFloat(r.FormValue("stack"), 64)
		}
		hta, _ := strconv.ParseFloat(r.FormValue("hta_deg_"+sz), 64)
		if hta == 0 {
			hta, _ = strconv.ParseFloat(r.FormValue("hta_deg"), 64)
		}
		sta, _ := strconv.ParseFloat(r.FormValue("sta_deg_"+sz), 64)
		if sta == 0 {
			sta, _ = strconv.ParseFloat(r.FormValue("sta_deg"), 64)
		}
		htl, _ := strconv.ParseFloat(r.FormValue("htl_"+sz), 64)
		if htl == 0 {
			htl, _ = strconv.ParseFloat(r.FormValue("htl"), 64)
		}
		bbDrop, _ := strconv.ParseFloat(r.FormValue("bb_drop_"+sz), 64)
		if bbDrop == 0 {
			bbDrop, _ = strconv.ParseFloat(r.FormValue("bb_drop"), 64)
		}
		wb, _ := strconv.ParseFloat(r.FormValue("wheelbase_"+sz), 64)
		if wb == 0 {
			wb, _ = strconv.ParseFloat(r.FormValue("wheelbase"), 64)
		}
		cs, _ := strconv.ParseFloat(r.FormValue("chainstay_"+sz), 64)
		st, _ := strconv.ParseFloat(r.FormValue("seat_tube_"+sz), 64)

		activeGeos[sz] = model.SizeGeometry{
			Reach:           reach,
			Stack:           stack,
			HtaDeg:          hta,
			StaDeg:          sta,
			Htl:             htl,
			BbDrop:          bbDrop,
			Wheelbase:       wb,
			ChainstayLength: cs,
			SeatTubeLength:  st,
		}
	}

	found := false
	for i := range configs {
		if configs[i].ID == activeConfigID {
			configs[i].Name = activeName
			configs[i].Fork = forkSpec
			configs[i].SagPercentagePublished = activeSag
			configs[i].Geometries = activeGeos
			found = true
			break
		}
	}
	if !found {
		configs = append(configs, model.FrameConfiguration{
			ID:                     activeConfigID,
			Name:                   activeName,
			IsBaseline:             len(configs) == 0,
			Fork:                   forkSpec,
			SagPercentagePublished: activeSag,
			Geometries:             activeGeos,
		})
	}

	fm := model.FrameModel{
		ID:             frameID,
		Manufacturer:   manufacturer,
		Model:          modelName,
		Year:           year,
		Category:       category,
		Notes:          notes,
		WheelSize:      wheelSize,
		HeadsetCupType: cupType,
		CupStack:       cupStack,
		Sizes:          sizes,
		DefaultSize:    sizes[0],
		Configurations: configs,
	}

	return fm, activeConfigID
}

func (s *Server) handleBikeSave(w http.ResponseWriter, r *http.Request) {
	isNew := r.URL.Query().Get("is_new") == "true"
	frame, activeConfigID := s.parseFrameFromForm(r)

	// Validate baseline geometry for default size
	matBike, _ := geometry.MaterializeBikeGeometry(frame, frame.GetDefaultSize(), "")
	report := validator.ValidateBikeGeometry(matBike)

	if !report.IsValid || frame.Manufacturer == "" || frame.Model == "" || len(frame.Sizes) == 0 {
		if frame.Manufacturer == "" {
			report.Issues = append(report.Issues, model.ValidationIssue{
				Field:    "manufacturer",
				Severity: model.SeverityError,
				Title:    "Missing Manufacturer",
				Message:  "Manufacturer name is required.",
			})
		}
		if frame.Model == "" {
			report.Issues = append(report.Issues, model.ValidationIssue{
				Field:    "model",
				Severity: model.SeverityError,
				Title:    "Missing Model",
				Message:  "Model name is required.",
			})
		}
		if len(frame.Sizes) == 0 {
			report.Issues = append(report.Issues, model.ValidationIssue{
				Field:    "sizes",
				Severity: model.SeverityError,
				Title:    "Missing Sizes",
				Message:  "At least one frame size is required.",
			})
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusUnprocessableEntity)
		_ = templates.EditorModal(frame, isNew, &report, "Please resolve the validation errors highlighted below.", "form", activeConfigID).Render(r.Context(), w)
		return
	}

	savedFrame, err := s.store.SaveFrame(frame)
	if err != nil {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_ = templates.EditorModal(frame, isNew, &report, fmt.Sprintf("Storage error: %v", err), "form", activeConfigID).Render(r.Context(), w)
		return
	}

	defaultBike, _ := geometry.MaterializeBikeGeometry(*savedFrame, savedFrame.GetDefaultSize(), "")
	locSave, _ := json.Marshal(map[string]string{
		"path":   fmt.Sprintf("/?tab=validator&id=%s", defaultBike.ID),
		"target": "#app-content",
	})
	w.Header().Set("HX-Location", string(locSave))
	w.WriteHeader(http.StatusOK)
}

func (s *Server) handleBikeDelete(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	_, _ = s.store.DeleteBike(id)

	locDel, _ := json.Marshal(map[string]string{
		"path":   "/?tab=validator",
		"target": "#app-content",
	})
	w.Header().Set("HX-Location", string(locDel))
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

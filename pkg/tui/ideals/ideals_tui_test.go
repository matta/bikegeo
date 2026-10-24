package ideals

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gdamore/tcell/v3"

	"bikegeo/pkg/ideal"
	"bikegeo/pkg/model"
	"bikegeo/pkg/tui"
)

func TestSessionStateManagement(t *testing.T) {
	tempDir := t.TempDir()

	// Load default embedded profile
	p, err := ideal.LoadProfile("", "default")
	if err != nil {
		t.Fatalf("failed to load default profile: %v", err)
	}

	session, err := NewSession(tempDir, p)
	if err != nil {
		t.Fatalf("failed to create session: %v", err)
	}

	if session.IsDirty() {
		t.Fatalf("expected initial session to be clean")
	}

	// Mutate description
	session.Working.Description = "Modified Description"
	if !session.IsDirty() {
		t.Fatalf("expected session to be dirty after mutation")
	}

	if err := session.Validate(); err != nil {
		t.Fatalf("expected valid profile, got: %v", err)
	}

	// Save to disk
	if err := session.Save(); err != nil {
		t.Fatalf("failed to save session: %v", err)
	}
	if session.IsDirty() {
		t.Fatalf("expected session to be clean after save")
	}

	// Verify file on disk
	savedPath := filepath.Join(tempDir, "default.json")
	if _, err := os.Stat(savedPath); err != nil {
		t.Fatalf("expected saved file at %s", savedPath)
	}

	// Test SaveAs
	if err := session.SaveAs("custom-fit"); err != nil {
		t.Fatalf("failed to save as: %v", err)
	}
	customPath := filepath.Join(tempDir, "custom-fit.json")
	if _, err := os.Stat(customPath); err != nil {
		t.Fatalf("expected saved file at %s", customPath)
	}

	// Test attribute manipulation
	initCount := len(session.Working.Attributes)
	session.AddAttribute(ideal.AttributeSpec{
		ID:      "standover",
		Weight:  2,
		Ideal:   &ideal.Range{Min: ptrFloat(700), Max: ptrFloat(750)},
		Falloff: &ideal.Falloff{Below: ptrFloat(20), Above: ptrFloat(20)},
	})
	if len(session.Working.Attributes) != initCount+1 {
		t.Fatalf("expected %d attributes after add", initCount+1)
	}

	session.MoveAttribute(initCount, 0)
	if session.Working.Attributes[0].ID != "standover" {
		t.Fatalf("expected standover to be moved to index 0")
	}

	session.RemoveAttribute(0)
	if len(session.Working.Attributes) != initCount {
		t.Fatalf("expected %d attributes after remove", initCount)
	}

	// Test invalid profile rejected on save
	session.Working.Attributes[0].Weight = -5
	if err := session.Save(); err == nil {
		t.Fatalf("expected save to fail with negative weight")
	}
}

func TestProfileChooserScreen(t *testing.T) {
	tempDir := t.TempDir()

	// Create two sample profiles in tempDir
	p1, _ := ideal.LoadProfile("", "default")
	s1, _ := NewSession(tempDir, p1)
	_ = s1.Save()

	s2, _ := NewBlankSession(tempDir, "trail-enduro")
	_ = s2.Save()

	_, scr := tui.NewMockScreen(t, 100, 30)
	defer scr.Fini()

	app := tui.NewApp(scr)
	chooser := NewProfileChooserScreen(app, tempDir)
	app.PushView(chooser)

	if !tui.ScreenContains(scr, "trail-enduro") {
		t.Fatalf("expected Screen to display 'trail-enduro', got:\n%s", tui.ScreenDump(scr))
	}
	if !tui.ScreenContains(scr, "default") {
		t.Fatalf("expected Screen to display 'default', got:\n%s", tui.ScreenDump(scr))
	}

	// Open selected with Enter
	enterEv := tcell.NewEventKey(tcell.KeyEnter, "", tcell.ModNone)
	chooser.HandleEvent(enterEv)

	// App should now have ProfileEditScreen on top
	if !tui.ScreenContains(scr, "IDEAL PROFILE:") {
		t.Fatalf("expected Screen to transition to ProfileEditScreen, got:\n%s", tui.ScreenDump(scr))
	}
}

func TestProfileEditScreenAttributeModalFlow(t *testing.T) {
	tempDir := t.TempDir()
	p, _ := ideal.LoadProfile("", "default")
	session, _ := NewSession(tempDir, p)

	_, scr := tui.NewMockScreen(t, 100, 30)
	defer scr.Fini()

	app := tui.NewApp(scr)
	editScreen := NewProfileEditScreen(app, session)
	app.PushView(editScreen)

	if !tui.ScreenContains(scr, "Setback (setback)") {
		t.Fatalf("expected Screen to display 'Setback (setback)', got:\n%s", tui.ScreenDump(scr))
	}

	// Press Enter to edit the selected attribute (Setback)
	enterEv := tcell.NewEventKey(tcell.KeyEnter, "", tcell.ModNone)
	editScreen.HandleEvent(enterEv)

	if !tui.ScreenContains(scr, "Edit Attribute: setback") {
		t.Fatalf("expected AttributeModal to appear on screen, got:\n%s", tui.ScreenDump(scr))
	}

	curView := app.CurrentView()
	attrModal, ok := curView.(*AttributeModal)
	if !ok {
		t.Fatalf("expected CurrentView to be AttributeModal, got %T", curView)
	}

	// Change weight in modal
	attrModal.weightInput.SetText("25")
	attrModal.validateLive()

	// Trigger Save action on modal
	attrModal.Modal.Buttons[0].Action()

	// Modal should be popped, returning to editScreen
	if app.CurrentView() != editScreen {
		t.Fatalf("expected active view to be editScreen after saving attribute")
	}

	// Verify setback weight was updated in working profile
	if session.Working.Attributes[0].Weight != 25 {
		t.Fatalf("expected updated weight 25, got %.0f", session.Working.Attributes[0].Weight)
	}

	// Press 's' to save profile
	sEv := tcell.NewEventKey(tcell.KeyRune, "s", tcell.ModNone)
	editScreen.HandleEvent(sEv)
	app.Redraw()

	if session.IsDirty() {
		t.Fatalf("expected session to be clean after save")
	}
	if !tui.ScreenContains(scr, "Saved to default.json") {
		t.Fatalf("expected save confirmation status on screen, got:\n%s", tui.ScreenDump(scr))
	}
}

func TestUnsavedChangesPromptOnExit(t *testing.T) {
	tempDir := t.TempDir()
	p, _ := ideal.LoadProfile("", "default")
	session, _ := NewSession(tempDir, p)

	_, scr := tui.NewMockScreen(t, 100, 30)
	defer scr.Fini()

	app := tui.NewApp(scr)
	editScreen := NewProfileEditScreen(app, session)
	app.PushView(editScreen)

	// Make dirty
	session.Working.Description = "New Unsaved Changes"

	// Press 'q'
	qEv := tcell.NewEventKey(tcell.KeyRune, "q", tcell.ModNone)
	editScreen.HandleEvent(qEv)

	if !tui.ScreenContains(scr, "Unsaved Changes") {
		t.Fatalf("expected Unsaved Changes modal on screen, got:\n%s", tui.ScreenDump(scr))
	}

	// Discard action
	modal, ok := app.CurrentView().(*tui.ModalDialog)
	if !ok {
		t.Fatalf("expected active view to be ModalDialog")
	}

	// Click Discard button (index 1)
	modal.Buttons[1].Action()

	if app.CurrentView() != nil && app.CurrentView() == modal {
		t.Fatalf("expected modal to be closed after discard")
	}
}

func TestTableColumnAlignment(t *testing.T) {
	p, err := ideal.LoadProfile("", "default")
	if err != nil {
		t.Fatalf("failed to load default profile: %v", err)
	}

	header := formatTableHeader(100)
	headerRunes := []rune(header)

	weightColIdx := strings.Index(string(headerRunes), "Weight")
	idealColIdx := strings.Index(string(headerRunes), "Ideal Range")
	accColIdx := strings.Index(string(headerRunes), "Acceptable")
	edgeColIdx := strings.Index(string(headerRunes), "Edge")
	fallColIdx := strings.Index(string(headerRunes), "Falloff")
	floorColIdx := strings.Index(string(headerRunes), "Floor")

	// Each row prefix is "  " (2 chars)
	for _, attr := range p.Attributes {
		row := "  " + formatAttributeRow(attr, 100)
		rowRunes := []rune(row)

		// Verify row has at least as many runes as header
		if len(rowRunes) < floorColIdx {
			t.Fatalf("row too short for attr %s: %d runes, expected at least %d", attr.ID, len(rowRunes), floorColIdx)
		}

		// At the character right before each column, there must be a space separator
		if rowRunes[weightColIdx-1] != ' ' {
			t.Errorf("Attr %s: misalignment before Weight column, got %q at idx %d in:\n%s", attr.ID, string(rowRunes[weightColIdx-1]), weightColIdx-1, row)
		}
		if rowRunes[idealColIdx-1] != ' ' {
			t.Errorf("Attr %s: misalignment before Ideal column, got %q at idx %d in:\n%s", attr.ID, string(rowRunes[idealColIdx-1]), idealColIdx-1, row)
		}
		if rowRunes[accColIdx-1] != ' ' {
			t.Errorf("Attr %s: misalignment before Acceptable column, got %q at idx %d in:\n%s", attr.ID, string(rowRunes[accColIdx-1]), accColIdx-1, row)
		}
		if rowRunes[edgeColIdx-1] != ' ' {
			t.Errorf("Attr %s: misalignment before Edge column, got %q at idx %d in:\n%s", attr.ID, string(rowRunes[edgeColIdx-1]), edgeColIdx-1, row)
		}
		if rowRunes[fallColIdx-1] != ' ' {
			t.Errorf("Attr %s: misalignment before Falloff column, got %q at idx %d in:\n%s", attr.ID, string(rowRunes[fallColIdx-1]), fallColIdx-1, row)
		}
		if rowRunes[floorColIdx-1] != ' ' {
			t.Errorf("Attr %s: misalignment before Floor column, got %q at idx %d in:\n%s", attr.ID, string(rowRunes[floorColIdx-1]), floorColIdx-1, row)
		}
	}
}

func TestCurveInspectorZoneBarAndEnglishSummary(t *testing.T) {
	meta, _ := ideal.LookupAttribute("aett")

	// 1. Two-Sided Bounded Spec
	spec := ideal.AttributeSpec{
		ID:     "aett",
		Weight: 10,
		Ideal: &ideal.Range{
			Min: ptrFloat(625),
			Max: ptrFloat(636),
		},
		Acceptable: &ideal.Range{
			Min: ptrFloat(605),
			Max: ptrFloat(648),
		},
		EdgeScore: ptrFloat(75),
		Falloff: &ideal.Falloff{
			Below: ptrFloat(20),
			Above: ptrFloat(12),
		},
		Floor: ptrFloat(-200),
	}

	inspector := NewCurveInspector(spec, meta, tui.DefaultTheme())

	// Verify Zone Segments
	segs := inspector.buildZoneSegments()
	if len(segs) != 7 {
		t.Fatalf("expected 7 segments for two-sided curve, got %d", len(segs))
	}
	if segs[0].Name != "[FLOOR]" || segs[0].ScoreStr != "-200" {
		t.Errorf("expected left floor segment, got %+v", segs[0])
	}
	if segs[1].Name != "[PENALTY]" || segs[1].ScoreStr != "0–75" {
		t.Errorf("expected left penalty segment, got %+v", segs[1])
	}
	if segs[2].Name != "[ACCEPT]" || segs[2].ScoreStr != "75–100" {
		t.Errorf("expected left accept segment, got %+v", segs[2])
	}
	if segs[3].Name != "[== IDEAL ==]" || segs[3].RangeStr != "625–636" || segs[3].ScoreStr != "100" {
		t.Errorf("expected ideal segment, got %+v", segs[3])
	}
	if segs[4].Name != "[ACCEPT]" || segs[4].ScoreStr != "100–75" {
		t.Errorf("expected right accept segment, got %+v", segs[4])
	}
	if segs[5].Name != "[PENALTY]" || segs[5].ScoreStr != "75–0" {
		t.Errorf("expected right penalty segment, got %+v", segs[5])
	}
	if segs[6].Name != "[FLOOR]" || segs[6].ScoreStr != "-200" {
		t.Errorf("expected right floor segment, got %+v", segs[6])
	}

	// Verify English Summary
	bullets := inspector.generateEnglishSummary()
	if len(bullets) < 4 {
		t.Fatalf("expected at least 4 summary bullets, got %d", len(bullets))
	}
	if !strings.Contains(bullets[0], "625–636 mm receives full 100 pts") {
		t.Errorf("unexpected sweet spot bullet: %q", bullets[0])
	}
	if !strings.Contains(bullets[1], "605–648 mm tapers from 100 down to 75 pts") {
		t.Errorf("unexpected acceptable bullet: %q", bullets[1])
	}
	if !strings.Contains(bullets[2], "below 605") || !strings.Contains(bullets[2], "above 648") {
		t.Errorf("unexpected penalty bullet: %q", bullets[2])
	}

	// 2. One-Sided Spec (Chainstay >= 455)
	csMeta, _ := ideal.LookupAttribute("chainstay")
	csSpec := ideal.AttributeSpec{
		ID:     "chainstay",
		Weight: 4,
		Ideal: &ideal.Range{
			Min: ptrFloat(455),
		},
		Acceptable: &ideal.Range{
			Min: ptrFloat(445),
		},
		EdgeScore: ptrFloat(70),
		Falloff: &ideal.Falloff{
			Below: ptrFloat(25),
		},
		Floor: ptrFloat(-100),
	}
	csInspector := NewCurveInspector(csSpec, csMeta, tui.DefaultTheme())
	csSegs := csInspector.buildZoneSegments()
	if len(csSegs) != 4 {
		t.Fatalf("expected 4 segments for lower-only curve, got %d", len(csSegs))
	}
	if csSegs[3].Name != "[==== IDEAL (UNBOUNDED) ====]" || csSegs[3].RangeStr != "≥ 455" {
		t.Errorf("expected unbounded ideal segment, got %+v", csSegs[3])
	}

	csBullets := csInspector.generateEnglishSummary()
	if !strings.Contains(csBullets[0], "≥ 455 mm receives full 100 pts") {
		t.Errorf("unexpected chainstay sweet spot bullet: %q", csBullets[0])
	}

	// 3. Prober & Database Ranking
	fleetRows := []model.CanonicalComparisonRow{
		{
			Bike:      model.BikeGeometry{Manufacturer: "Surly", Model: "ECR"},
			Setup:     model.ComparisonSetup{Size: "L"},
			Canonical: model.BikeFitAnalysis{Aett: 630},
		},
		{
			Bike:      model.BikeGeometry{Manufacturer: "Ritchey", Model: "Ultra"},
			Setup:     model.ComparisonSetup{Size: "L"},
			Canonical: model.BikeFitAnalysis{Aett: 647},
		},
		{
			Bike:      model.BikeGeometry{Manufacturer: "Esker", Model: "Hayduke"},
			Setup:     model.ComparisonSetup{Size: "M"},
			Canonical: model.BikeFitAnalysis{Aett: 595},
		},
	}
	compareRows := []model.CanonicalComparisonRow{
		fleetRows[0],
	}

	inspector.SetBikeRows(fleetRows, compareRows)
	if len(inspector.ActiveRows()) != 3 {
		t.Fatalf("expected 3 fleet rows, got %d", len(inspector.ActiveRows()))
	}

	// Toggle Dataset
	inspector.ToggleDataset()
	if len(inspector.ActiveRows()) != 1 {
		t.Fatalf("expected 1 compare row after toggle, got %d", len(inspector.ActiveRows()))
	}
	inspector.ToggleDataset()
	if len(inspector.ActiveRows()) != 3 {
		t.Fatalf("expected 3 fleet rows after second toggle, got %d", len(inspector.ActiveRows()))
	}

	// Value Prober
	testVal := 630.0
	inspector.SetProbeVal(&testVal)
	if inspector.ProbeVal == nil || *inspector.ProbeVal != 630.0 {
		t.Fatalf("expected ProbeVal 630.0, got %v", inspector.ProbeVal)
	}
}

func TestAttributeModalTwoPaneLiveUpdatesAndHotkeys(t *testing.T) {
	spec := ideal.AttributeSpec{
		ID:         "aett",
		Weight:     10,
		Ideal:      &ideal.Range{Min: ptrFloat(625), Max: ptrFloat(636)},
		Acceptable: &ideal.Range{Min: ptrFloat(605), Max: ptrFloat(648)},
		EdgeScore:  ptrFloat(75),
		Falloff:    &ideal.Falloff{Below: ptrFloat(20), Above: ptrFloat(12)},
		Floor:      ptrFloat(-200),
	}

	_, scr := tui.NewMockScreen(t, 100, 30)
	defer scr.Fini()

	app := tui.NewApp(scr)
	modal := NewAttributeModal(spec, false, tui.DefaultTheme())

	fleetRows := []model.CanonicalComparisonRow{
		{
			Bike:      model.BikeGeometry{Manufacturer: "Surly", Model: "ECR"},
			Setup:     model.ComparisonSetup{Size: "L"},
			Canonical: model.BikeFitAnalysis{Aett: 630},
		},
		{
			Bike:      model.BikeGeometry{Manufacturer: "Ritchey", Model: "Ultra"},
			Setup:     model.ComparisonSetup{Size: "L"},
			Canonical: model.BikeFitAnalysis{Aett: 647},
		},
	}
	compareRows := []model.CanonicalComparisonRow{
		fleetRows[0],
	}
	modal.SetBikeRows(fleetRows, compareRows)

	app.PushView(modal)
	app.Redraw()

	dump := tui.ScreenDump(scr)

	// Check for left pane elements
	if !tui.ScreenContains(scr, "Probe Value") {
		t.Fatalf("expected 'Probe Value' input on left form pane, got:\n%s", dump)
	}

	// Check for right pane inspector elements
	if !tui.ScreenContains(scr, "Analysis: AETT") {
		t.Fatalf("expected 'Analysis: AETT' header in inspector pane, got:\n%s", dump)
	}
	if !tui.ScreenContains(scr, "IDEAL") {
		t.Fatalf("expected 'IDEAL' segment on zone bar, got:\n%s", dump)
	}
	if !tui.ScreenContains(scr, "Sweet spot: 625–636 mm receives full 100 pts") {
		t.Fatalf("expected English summary on right pane, got:\n%s", dump)
	}
	if !tui.ScreenContains(scr, "Surly ECR (L") {
		t.Fatalf("expected representative real frame on right pane, got:\n%s", dump)
	}

	// Test live probe entry
	modal.probeInput.SetText("632")
	modal.validateLive()
	app.Redraw()

	if !tui.ScreenContains(scr, "Probe: 632mm ── Score: 100 pts") {
		t.Fatalf("expected live probe calculation on screen, got:\n%s", tui.ScreenDump(scr))
	}

	// Test Dataset Toggle via F2 key
	if !tui.ScreenContains(scr, "Fleet: 2 sizes") {
		t.Fatalf("expected Fleet dataset indicator on screen, got:\n%s", tui.ScreenDump(scr))
	}
	f2Ev := tcell.NewEventKey(tcell.KeyF2, "", tcell.ModNone)
	modal.HandleEvent(f2Ev)
	app.Redraw()

	if !tui.ScreenContains(scr, "Compare: 1 sizes") {
		t.Fatalf("expected Compare dataset indicator on screen after F2 toggle, got:\n%s", tui.ScreenDump(scr))
	}
}

package ideals

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/gdamore/tcell/v3"

	"bikegeo/pkg/ideal"
	"bikegeo/pkg/tui"
)

// ProfileChooserScreen allows browsing, selecting, creating, cloning, or deleting ideal profiles.
type ProfileChooserScreen struct {
	App       *tui.App
	IdealsDir string
	Theme     tui.Theme
	ListView  *tui.ListView
	StatusBar *tui.StatusBar

	summaries []ideal.ProfileSummary
	statusMsg string
	isErr     bool
}

// NewProfileChooserScreen creates an initialized ProfileChooserScreen.
func NewProfileChooserScreen(app *tui.App, idealsDir string) *ProfileChooserScreen {
	pcs := &ProfileChooserScreen{
		App:       app,
		IdealsDir: idealsDir,
		Theme:     tui.DefaultTheme(),
		ListView:  tui.NewListView(),
		StatusBar: tui.NewStatusBar([]tui.KeyHint{
			{Key: "[Enter]", Desc: "Open"},
			{Key: "[n]", Desc: "New"},
			{Key: "[c]", Desc: "Clone"},
			{Key: "[d]", Desc: "Delete"},
			{Key: "[q]", Desc: "Quit"},
		}),
	}
	pcs.ListView.Focused = true
	pcs.ListView.OnSelect = func(idx int, item tui.ListItem) {
		pcs.openProfile(item.ID)
	}
	pcs.refreshList()
	return pcs
}

func (pcs *ProfileChooserScreen) refreshList() {
	summaries, err := ideal.ListProfiles(pcs.IdealsDir)
	if err != nil {
		pcs.statusMsg = err.Error()
		pcs.isErr = true
		return
	}
	pcs.summaries = summaries

	var items []tui.ListItem
	for _, s := range summaries {
		shStr := "—"
		if s.SaddleHeightMm != nil {
			shStr = fmt.Sprintf("%.0fmm", *s.SaddleHeightMm)
		}
		secondary := fmt.Sprintf("Saddle: %-6s | %d attrs (%d scored)", shStr, s.AttributeCount, s.ScoredCount)
		mainText := fmt.Sprintf("%-16s %s", s.Name, s.Description)
		items = append(items, tui.ListItem{
			ID:        s.Name,
			Text:      mainText,
			Secondary: secondary,
		})
	}
	pcs.ListView.SetItems(items)
	pcs.statusMsg = fmt.Sprintf("Found %d ideal profile(s) in %s", len(summaries), pcs.IdealsDir)
	pcs.isErr = false
}

// Init view.
func (pcs *ProfileChooserScreen) Init(theme tui.Theme) {
	pcs.Theme = theme
	pcs.ListView.Theme = theme
	pcs.StatusBar.Theme = theme
}

// Draw renders chooser screen.
func (pcs *ProfileChooserScreen) Draw(s tcell.Screen, bounds tui.Rect) {
	if bounds.W <= 0 || bounds.H <= 0 {
		return
	}

	// 0. Completely clear background
	tui.FillRect(s, bounds, pcs.Theme.Default)

	// Title bar
	title := " BIKEGEO IDEAL PROFILES "
	tui.FillRect(s, tui.Rect{X: bounds.X, Y: bounds.Y, W: bounds.W, H: 1}, pcs.Theme.Selected)
	tui.DrawText(s, bounds.X+1, bounds.Y, title, pcs.Theme.Selected)

	// List Box
	boxH := bounds.H - 2
	boxRect := tui.Rect{X: bounds.X, Y: bounds.Y + 1, W: bounds.W, H: boxH}
	tui.FillRect(s, boxRect, pcs.Theme.Default)
	tui.DrawBox(s, boxRect, "Select Ideal Profile to Edit", pcs.Theme.Border)

	// Header line
	headerY := bounds.Y + 2
	tui.DrawText(s, bounds.X+4, headerY, fmt.Sprintf("%-16s %-30s %s", "Name", "Description", "Details"), pcs.Theme.Header)
	tui.DrawText(s, bounds.X+2, headerY+1, strings.Repeat("─", bounds.W-4), pcs.Theme.Border)

	// List View
	listRect := tui.Rect{X: bounds.X + 2, Y: headerY + 2, W: bounds.W - 4, H: boxH - 4}
	pcs.ListView.Draw(s, listRect)

	// Status Bar
	pcs.StatusBar.Message = pcs.statusMsg
	pcs.StatusBar.IsError = pcs.isErr
	pcs.StatusBar.Draw(s, bounds)
}

// HandleEvent handles chooser actions.
func (pcs *ProfileChooserScreen) HandleEvent(ev tcell.Event) tui.EventResult {
	kev, ok := ev.(*tcell.EventKey)
	if !ok {
		return tui.EventIgnored
	}

	switch kev.Key() {
	case tcell.KeyEscape:
		pcs.App.Quit()
		return tui.EventQuit
	case tcell.KeyRune:
		switch kev.Str() {
		case "q":
			pcs.App.Quit()
			return tui.EventQuit
		case "n":
			pcs.promptNewProfile()
			return tui.EventHandled
		case "c":
			pcs.promptCloneProfile()
			return tui.EventHandled
		case "d":
			pcs.promptDeleteProfile()
			return tui.EventHandled
		}
	}

	return pcs.ListView.HandleEvent(ev)
}

func (pcs *ProfileChooserScreen) openProfile(name string) {
	p, err := ideal.LoadProfile(pcs.IdealsDir, name)
	if err != nil {
		pcs.statusMsg = err.Error()
		pcs.isErr = true
		return
	}

	session, err := NewSession(pcs.IdealsDir, p)
	if err != nil {
		pcs.statusMsg = err.Error()
		pcs.isErr = true
		return
	}

	editScreen := NewProfileEditScreen(pcs.App, session)
	editScreen.OnBack = func() {
		pcs.App.PopView()
		pcs.refreshList()
	}
	pcs.App.PushView(editScreen)
}

func (pcs *ProfileChooserScreen) promptNewProfile() {
	modal := tui.NewModalDialog("Create New Profile", 48, 8)
	nameInput := tui.NewTextInput("custom")
	nameInput.Focused = true

	form := tui.NewForm()
	form.AddField(tui.FormField{ID: "name", Label: "Profile Name", Widget: nameInput})
	modal.Content = form

	modal.Buttons = []tui.ModalButton{
		{
			Label: "Create",
			Action: func() {
				name := strings.TrimSpace(nameInput.Text)
				if name == "" {
					modal.StatusMsg = "Name cannot be empty"
					modal.StatusIsWarn = true
					return
				}
				session, err := NewBlankSession(pcs.IdealsDir, name)
				if err != nil {
					modal.StatusMsg = err.Error()
					modal.StatusIsWarn = true
					return
				}
				pcs.App.PopView()
				editScreen := NewProfileEditScreen(pcs.App, session)
				editScreen.OnBack = func() {
					pcs.App.PopView()
					pcs.refreshList()
				}
				pcs.App.PushView(editScreen)
			},
		},
		{
			Label: "Cancel",
			Action: func() {
				pcs.App.PopView()
			},
		},
	}
	pcs.App.PushView(modal)
}

func (pcs *ProfileChooserScreen) promptCloneProfile() {
	sel, ok := pcs.ListView.SelectedItem()
	if !ok {
		return
	}

	modal := tui.NewModalDialog("Clone Profile: "+sel.ID, 48, 8)
	nameInput := tui.NewTextInput(sel.ID + "-copy")
	nameInput.Focused = true

	form := tui.NewForm()
	form.AddField(tui.FormField{ID: "name", Label: "Cloned Name", Widget: nameInput})
	modal.Content = form

	modal.Buttons = []tui.ModalButton{
		{
			Label: "Clone",
			Action: func() {
				name := strings.TrimSpace(nameInput.Text)
				if name == "" {
					modal.StatusMsg = "Name cannot be empty"
					modal.StatusIsWarn = true
					return
				}
				p, err := ideal.LoadProfile(pcs.IdealsDir, sel.ID)
				if err != nil {
					modal.StatusMsg = err.Error()
					modal.StatusIsWarn = true
					return
				}
				session, err := NewSession(pcs.IdealsDir, p)
				if err != nil {
					modal.StatusMsg = err.Error()
					modal.StatusIsWarn = true
					return
				}
				session.Working.Name = name
				pcs.App.PopView()
				editScreen := NewProfileEditScreen(pcs.App, session)
				editScreen.OnBack = func() {
					pcs.App.PopView()
					pcs.refreshList()
				}
				pcs.App.PushView(editScreen)
			},
		},
		{
			Label: "Cancel",
			Action: func() {
				pcs.App.PopView()
			},
		},
	}
	pcs.App.PushView(modal)
}

func (pcs *ProfileChooserScreen) promptDeleteProfile() {
	sel, ok := pcs.ListView.SelectedItem()
	if !ok {
		return
	}
	if sel.ID == "default" {
		pcs.statusMsg = "Cannot delete the default profile"
		pcs.isErr = true
		return
	}

	modal := tui.NewModalDialog("Delete Profile?", 48, 8)
	modal.Message = fmt.Sprintf("Delete profile %q permanently?", sel.ID)
	modal.Buttons = []tui.ModalButton{
		{
			Label: "Delete",
			Action: func() {
				path := filepath.Join(pcs.IdealsDir, sel.ID+".json")
				_ = os.Remove(path)
				pcs.App.PopView()
				pcs.refreshList()
			},
		},
		{
			Label: "Cancel",
			Action: func() {
				pcs.App.PopView()
			},
		},
	}
	pcs.App.PushView(modal)
}

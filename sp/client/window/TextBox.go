package window

import (
	"sync"

	rg "github.com/gen2brain/raylib-go/raygui"
	rl "github.com/gen2brain/raylib-go/raylib"
)

type TextBoxComponent struct {
	bounds    rl.Rectangle
	ID        string
	Text      *string // Pointer to the model string
	TextMutex *sync.RWMutex
	maxChars  int
	editMode  bool
}

func NewTextBoxComponent(id string, text *string, mutex *sync.RWMutex, maxChars int) *TextBoxComponent {
	return &TextBoxComponent{
		ID:        id,
		Text:      text,
		TextMutex: mutex,
		maxChars:  maxChars,
	}
}

func (t *TextBoxComponent) Calculate(bounds rl.Rectangle) {
	t.bounds = bounds
}

func (t *TextBoxComponent) Draw(eventChannel chan<- UIEvent) {
	// Handle click-to-activate
	if rl.IsMouseButtonPressed(rl.MouseLeftButton) {
		t.editMode = rl.CheckCollisionPointRec(rl.GetMousePosition(), t.bounds)
	}

	// Lock mutex for reading/writing the string
	t.TextMutex.Lock()
	changed := rg.TextBox(t.bounds, t.Text, t.maxChars, t.editMode)
	t.TextMutex.Unlock()

	if changed {
		eventChannel <- UIEvent{SourceID: t.ID, Type: EventValueChange}
	}
}

func (t *TextBoxComponent) GetBounds() rl.Rectangle {
	return t.bounds
}

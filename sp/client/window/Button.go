package window

import (
	rg "github.com/gen2brain/raylib-go/raygui"
	rl "github.com/gen2brain/raylib-go/raylib"
)

type ButtonComponent struct {
	bounds rl.Rectangle
	ID     string
	Text   string
}

func NewButtonComponent(id string, text string) *ButtonComponent {
	return &ButtonComponent{ID: id, Text: text}
}

func (b *ButtonComponent) Calculate(bounds rl.Rectangle) {
	b.bounds = bounds
}

func (b *ButtonComponent) Draw(eventChannel chan<- UIEvent) {
	if rg.Button(b.bounds, b.Text) {
		eventChannel <- UIEvent{SourceID: b.ID, Type: EventClick}
	}
}

func (b *ButtonComponent) GetBounds() rl.Rectangle {
	return b.bounds
}

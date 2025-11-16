package window

import (
	rl "github.com/gen2brain/raylib-go/raylib"
)

type EventType int

const (
	EventClick EventType = iota
	EventValueChange
)

type UIEvent struct {
	SourceID string
	Type     EventType
}

type RGComponent interface {
	Draw(eventChannel chan<- UIEvent)
	Calculate(bounds rl.Rectangle)
	GetBounds() rl.Rectangle
}

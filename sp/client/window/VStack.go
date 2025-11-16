package window

import (
	rl "github.com/gen2brain/raylib-go/raylib"
)

type VStack struct {
	bounds   rl.Rectangle
	children []RGComponent
	padding  float32
}

func NewVStack(padding float32) *VStack {
	return &VStack{
		padding:  padding,
		children: make([]RGComponent, 0),
	}
}

func (s *VStack) AddChild(child RGComponent) {
	s.children = append(s.children, child)
}

func (s *VStack) GetBounds() rl.Rectangle {
	return s.bounds
}

func (s *VStack) Calculate(bounds rl.Rectangle) {
	s.bounds = bounds

	childCount := float32(len(s.children))
	if childCount == 0 {
		return
	}

	innerX := s.bounds.X + s.padding
	innerY := s.bounds.Y + s.padding
	innerWidth := s.bounds.Width - (s.padding * 2)
	innerHeight := s.bounds.Height - (s.padding * 2)

	totalPadding := s.padding * (childCount - 1)
	availableHeight := innerHeight - totalPadding
	childHeight := availableHeight / childCount
	childWidth := innerWidth

	currentY := innerY
	for _, child := range s.children {
		childBounds := rl.Rectangle{
			X:      innerX,
			Y:      currentY,
			Width:  childWidth,
			Height: childHeight,
		}
		child.Calculate(childBounds)
		currentY += childHeight + s.padding
	}
}

func (s *VStack) Draw(eventChannel chan<- UIEvent) {
	// debug output
	// rl.DrawRectangleLinesEx(s.bounds, 1, rl.Red)

	for _, child := range s.children {
		child.Draw(eventChannel)
	}
}

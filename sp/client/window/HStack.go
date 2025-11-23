package window

import (
	rl "github.com/gen2brain/raylib-go/raylib"
)

type HStack struct {
	bounds   rl.Rectangle
	children []RGComponent
	padding  float32
}

func NewHStack(padding float32) *HStack {
	return &HStack{
		padding:  padding,
		children: make([]RGComponent, 0),
	}
}

func (s *HStack) AddChild(child RGComponent) {
	s.children = append(s.children, child)
}

func (s *HStack) GetBounds() rl.Rectangle {
	return s.bounds
}

func (s *HStack) Calculate(bounds rl.Rectangle) {
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
	availableWidth := innerWidth - totalPadding
	childWidth := availableWidth / childCount
	childHeight := innerHeight

	currentX := innerX
	for _, child := range s.children {
		childBounds := rl.Rectangle{
			X:      currentX,
			Y:      innerY,
			Width:  childWidth,
			Height: childHeight,
		}

		child.Calculate(childBounds)
		currentX += childWidth + s.padding
	}
}

func (s *HStack) Draw(eventChannel chan<- UIEvent) {
	// Debug draw
	// rl.DrawRectangleLinesEx(s.bounds, 1, rl.Blue)

	for _, child := range s.children {
		child.Draw(eventChannel)
	}
}

func (s *HStack) Rebuild(old RGComponent) {
	if old == nil {
		return
	}

	if oldS, ok := old.(*HStack); ok {
		for i, child := range s.children {
			if i < len(oldS.children) {
				child.Rebuild(oldS.children[i])
			}
		}
	}
}

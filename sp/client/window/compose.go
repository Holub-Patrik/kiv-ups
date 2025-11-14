package window

import (
	"sync"

	rg "github.com/gen2brain/raylib-go/raygui"
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

type VStack struct {
	bounds   rl.Rectangle
	children []RGComponent
	padding  float32
}

type HStack struct {
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
	// rl.DrawRectangleLinesEx(s.bounds, 1, rl.Blue) // Debug draw
	for _, child := range s.children {
		child.Draw(eventChannel)
	}
}

type GameScreen struct {
	// GameScreen is a VStack.
	*VStack

	playerBar *HStack
	actionBar *HStack
}

func NewGameScreen(padding float32) *GameScreen {
	mainLayout := NewVStack(padding)

	playerBar := NewHStack(padding)
	actionBar := NewHStack(padding)

	return &GameScreen{
		VStack:    mainLayout,
		playerBar: playerBar,
		actionBar: actionBar,
	}
}

func (gs *GameScreen) Calculate(bounds rl.Rectangle) {
	gs.bounds = bounds

	padding := gs.padding
	const actionBarRatio float32 = 0.2

	actionBarHeight := (bounds.Height - padding*3) * actionBarRatio
	playerBarHeight := (bounds.Height - padding*3) * (1.0 - actionBarRatio)

	playerBounds := rl.Rectangle{
		X:      bounds.X + padding,
		Y:      bounds.Y + padding,
		Width:  bounds.Width - padding*2,
		Height: playerBarHeight,
	}

	actionBounds := rl.Rectangle{
		X:      bounds.X + padding,
		Y:      bounds.Y + padding + playerBarHeight + padding,
		Width:  bounds.Width - padding*2,
		Height: actionBarHeight,
	}

	gs.playerBar.Calculate(playerBounds)
	gs.actionBar.Calculate(actionBounds)
}

func (gs *GameScreen) Draw(eventChannel chan<- UIEvent) {
	gs.playerBar.Draw(eventChannel)
	gs.actionBar.Draw(eventChannel)
}

func (gs *GameScreen) AddPlayerCard(card RGComponent) {
	gs.playerBar.AddChild(card)
}

func (gs *GameScreen) AddActionButton(button RGComponent) {
	gs.actionBar.AddChild(button)
}

type PanelComponent struct {
	bounds rl.Rectangle
	Color  rl.Color
	child  RGComponent
}

func NewPanelComponent(color rl.Color) *PanelComponent {
	return &PanelComponent{Color: color}
}

func (p *PanelComponent) Calculate(bounds rl.Rectangle) {
	p.bounds = bounds
	p.child.Calculate(bounds)
}

func (p *PanelComponent) Draw(eventChannel chan<- UIEvent) {
	rl.DrawRectangleRec(p.bounds, p.Color)
	rl.DrawRectangleLinesEx(p.bounds, 1, rl.White) // Debug border

	p.child.Draw(eventChannel)
}

func (p *PanelComponent) GetBounds() rl.Rectangle {
	return p.bounds
}

func (p *PanelComponent) AddChild(child RGComponent) {
	p.child = child
}

type LabelComponent struct {
	bounds   rl.Rectangle
	Text     string
	FontSize int32
	Color    rl.Color
}

func NewLabelComponent(text string, fontSize int32, color rl.Color) *LabelComponent {
	return &LabelComponent{
		Text:     text,
		FontSize: fontSize,
		Color:    color,
	}
}
func (l *LabelComponent) Calculate(bounds rl.Rectangle) {
	l.bounds = bounds
}
func (l *LabelComponent) Draw(eventChannel chan<- UIEvent) {
	// Simple centered text for now
	textWidth := rl.MeasureText(l.Text, l.FontSize)
	x := l.bounds.X + (l.bounds.Width/2 - float32(textWidth)/2)
	y := l.bounds.Y + (l.bounds.Height/2 - float32(l.FontSize)/2)
	rl.DrawText(l.Text, int32(x), int32(y), l.FontSize, l.Color)
}
func (l *LabelComponent) GetBounds() rl.Rectangle {
	return l.bounds
}

// --- ButtonComponent ---
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

// --- TextBoxComponent ---
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

package window

import (
	"time"

	rl "github.com/gen2brain/raylib-go/raylib"
)

type PopupComponent interface {
	RGComponent
	Update() bool // Returns true if the popup is still-alive
}

type TimedPopup struct {
	bounds    rl.Rectangle
	text      string
	color     rl.Color
	duration  time.Duration
	startTime time.Time
}

func NewTimedPopup(text string, duration time.Duration) *TimedPopup {
	return &TimedPopup{
		text:      text,
		color:     rl.NewColor(50, 50, 50, 240),
		duration:  duration,
		startTime: time.Now(),
	}
}

func (p *TimedPopup) Update() bool {
	elapsed := time.Since(p.startTime)
	isAlive := elapsed < p.duration
	return isAlive
}

// Calculate positions the popup in the center of the screen.
func (p *TimedPopup) Calculate(screenBounds rl.Rectangle) {
	const widthRatio float32 = 0.4   // 40% of screen width
	const heightRatio float32 = 0.15 // 15% of screen height

	const maxHeight float32 = 50
	const maxWidth float32 = 150

	p.bounds.Width = screenBounds.Width * widthRatio
	p.bounds.Height = screenBounds.Height * heightRatio

	if p.bounds.Width > maxWidth {
		p.bounds.Width = maxWidth
	}

	if p.bounds.Height > maxHeight {
		p.bounds.Height = maxHeight
	}

	p.bounds.X = screenBounds.X
	p.bounds.Y = screenBounds.Y
}

func (p *TimedPopup) Draw(eventChannel chan<- UIEvent) {
	rl.DrawRectangleRec(p.bounds, p.color)
	rl.DrawRectangleLinesEx(p.bounds, 3, rl.White)

	rl.DrawText(p.text, int32(p.bounds.X+20), int32(p.bounds.Y+20), 20, rl.White)
}

func (p *TimedPopup) GetBounds() rl.Rectangle {
	return p.bounds
}

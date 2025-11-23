package window

import (
	"time"

	rl "github.com/gen2brain/raylib-go/raylib"
)

const margin float32 = 5

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

func (p *TimedPopup) Calculate(screenBounds rl.Rectangle) {
	const widthRatio float32 = 0.4   // 40% of screen width
	const heightRatio float32 = 0.15 // 15% of screen height

	textSize := rl.MeasureTextEx(rl.GetFontDefault(), p.text, 20, 1)

	p.bounds.Width = textSize.X + 2*margin
	p.bounds.Height = textSize.Y + 2*margin

	p.bounds.X = screenBounds.X
	p.bounds.Y = screenBounds.Y

}

func (p *TimedPopup) Draw(eventChannel chan<- UIEvent) {
	rl.DrawRectangleRec(p.bounds, p.color)
	rl.DrawRectangleLinesEx(p.bounds, 3, rl.White)

	rl.DrawTextEx(
		rl.GetFontDefault(),
		p.text,
		rl.Vector2{
			X: p.bounds.X + margin,
			Y: p.bounds.Y + margin,
		},
		20,
		1,
		rl.White,
	)
}

func (p *TimedPopup) GetBounds() rl.Rectangle {
	return p.bounds
}

func (p *TimedPopup) Rebuild(old RGComponent) {
	/* noop popups don't hold persistant data */
}

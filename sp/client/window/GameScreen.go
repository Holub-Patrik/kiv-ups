package window

import (
	rl "github.com/gen2brain/raylib-go/raylib"
)

type GameScreen struct {
	// GameScreen is a VStack.
	*VStack

	riverBar  *HStack
	playerBar *HStack
	actionBar *HStack
}

func NewGameScreen(padding float32) *GameScreen {
	mainLayout := NewVStack(padding)

	riverBar := NewHStack(padding)
	playerBar := NewHStack(padding)
	actionBar := NewHStack(padding)

	return &GameScreen{
		VStack:    mainLayout,
		riverBar:  riverBar,
		playerBar: playerBar,
		actionBar: actionBar,
	}
}

func (gs *GameScreen) Calculate(bounds rl.Rectangle) {
	gs.bounds = bounds

	padding := gs.padding
	const actionBarRatio float32 = 0.1
	const riverBarRatio float32 = 0.3

	riverBarHeight := (bounds.Height - padding*4) * riverBarRatio
	actionBarHeight := (bounds.Height - padding*4) * actionBarRatio
	playerBarHeight := (bounds.Height - padding*4) * (1.0 - actionBarRatio - riverBarRatio)

	riverBounds := rl.Rectangle{
		X:      bounds.X + padding,
		Y:      bounds.Y + padding,
		Width:  bounds.Width - padding*2,
		Height: riverBarHeight,
	}

	playerBounds := rl.Rectangle{
		X:      bounds.X + padding,
		Y:      bounds.Y + padding + actionBarHeight + padding,
		Width:  bounds.Width - padding*2,
		Height: playerBarHeight,
	}

	actionBounds := rl.Rectangle{
		X:      bounds.X + padding,
		Y:      bounds.Y + padding + riverBarHeight + padding + playerBarHeight + padding,
		Width:  bounds.Width - padding*2,
		Height: actionBarHeight,
	}

	gs.riverBar.Calculate(riverBounds)
	gs.playerBar.Calculate(playerBounds)
	gs.actionBar.Calculate(actionBounds)
}

func (gs *GameScreen) Draw(eventChannel chan<- UIEvent) {
	gs.playerBar.Draw(eventChannel)
	gs.actionBar.Draw(eventChannel)
}

func (gs *GameScreen) ResetRiver() {
	gs.riverBar = NewHStack(gs.padding)
}

func (gs *GameScreen) AddRiverCard(card RGComponent) {
	gs.riverBar.AddChild(card)
}

func (gs *GameScreen) AddPlayerCard(card RGComponent) {
	gs.playerBar.AddChild(card)
}

func (gs *GameScreen) AddActionButton(button RGComponent) {
	gs.actionBar.AddChild(button)
}

package window

import (
	rl "github.com/gen2brain/raylib-go/raylib"
)

type GameScreen struct {
	*VStack
	riverBar        *HStack
	playerBar       *HStack
	actionBar       *HStack
	otherPlayersBar *HStack
	potDisplay      RGComponent
}

func NewGameScreen(padding float32) *GameScreen {
	return &GameScreen{
		VStack:          NewVStack(padding),
		riverBar:        NewHStack(padding),
		playerBar:       NewHStack(padding),
		actionBar:       NewHStack(padding),
		otherPlayersBar: NewHStack(padding),
	}
}

// helpers for gamescreen building
func (gs *GameScreen) AddRiverCard(card RGComponent)     { gs.riverBar.AddChild(card) }
func (gs *GameScreen) ResetRiver()                       { gs.riverBar = NewHStack(gs.padding) }
func (gs *GameScreen) AddPlayerCard(card RGComponent)    { gs.playerBar.AddChild(card) } // Your hand
func (gs *GameScreen) AddActionButton(btn RGComponent)   { gs.actionBar.AddChild(btn) }
func (gs *GameScreen) ResetOtherPlayers()                { gs.otherPlayersBar = NewHStack(gs.padding) }
func (gs *GameScreen) AddOtherPlayer(player RGComponent) { gs.otherPlayersBar.AddChild(player) }
func (gs *GameScreen) SetPotDisplay(pot RGComponent)     { gs.potDisplay = pot }

func (gs *GameScreen) Calculate(bounds rl.Rectangle) {
	gs.bounds = bounds
	padding := gs.padding

	const potH = 0.08
	const riverH = 0.15
	const oppH = 0.35
	const handH = 0.15
	const actionH = 0.12

	totalPad := padding * 6
	availH := bounds.Height - totalPad

	y := bounds.Y + padding

	if gs.potDisplay != nil {
		h := availH * potH
		gs.potDisplay.Calculate(rl.Rectangle{X: bounds.X + padding, Y: y, Width: bounds.Width - padding*2, Height: h})
		y += h + padding
	}

	actualRiverH := availH * riverH
	gs.riverBar.Calculate(rl.Rectangle{X: bounds.X + padding, Y: y, Width: bounds.Width - padding*2, Height: actualRiverH})
	y += actualRiverH + padding

	actualOtherPlayersH := availH * oppH
	gs.otherPlayersBar.Calculate(rl.Rectangle{X: bounds.X + padding, Y: y, Width: bounds.Width - padding*2, Height: actualOtherPlayersH})
	y += actualOtherPlayersH + padding

	actualHandH := availH * handH
	gs.playerBar.Calculate(rl.Rectangle{X: bounds.X + padding, Y: y, Width: bounds.Width - padding*2, Height: actualHandH})
	y += actualHandH + padding

	actualActionH := availH * actionH
	gs.actionBar.Calculate(rl.Rectangle{X: bounds.X + padding, Y: y, Width: bounds.Width - padding*2, Height: actualActionH})
}

func (gs *GameScreen) Draw(eventChannel chan<- UIEvent) {
	if gs.potDisplay != nil {
		gs.potDisplay.Draw(eventChannel)
	}
	gs.riverBar.Draw(eventChannel)
	gs.otherPlayersBar.Draw(eventChannel)
	gs.playerBar.Draw(eventChannel)
	gs.actionBar.Draw(eventChannel)
}

func (gs *GameScreen) GetBounds() rl.Rectangle { return gs.bounds }

func (gs *GameScreen) Rebuild(old RGComponent) {
	if old == nil {
		return
	}

	if oldGS, ok := old.(*GameScreen); ok {
		gs.actionBar.Rebuild(oldGS.actionBar)
	}
}

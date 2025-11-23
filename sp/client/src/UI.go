package main

import (
	"fmt"
	"sort"
	"strconv"

	w "poker-client/window"

	rl "github.com/gen2brain/raylib-go/raylib"
)

func buildUI(ctx *ProgCtx) {
	ctx.UI.MainMenu = buildMainMenu(ctx)
	ctx.UI.ServerSelect = buildServerConnectMenu(ctx)
	ctx.UI.Connecting = buildConnectingScreen(ctx)
	ctx.UI.Game = buildGameScreen(ctx)
}

func buildMainMenu(ctx *ProgCtx) UIElement {
	_ = ctx

	mainMenu := w.NewVStack(10)
	connect_btn := w.NewCenterComponent(w.NewButtonComponent("MainMenu_ConnectBtn", "Connect", 150, 50))
	close_btn := w.NewCenterComponent(w.NewButtonComponent("MainMenu_CloseBtn", "Close", 150, 50))
	mainMenu.AddChild(connect_btn)
	mainMenu.AddChild(close_btn)

	mainMenuPanel := w.NewPanelComponent(rl.DarkGray, mainMenu)
	mainMenuBounds := w.NewBoundsBox(0.6, 0.8, mainMenuPanel)

	return UIElement{dirty: true, component: mainMenuBounds}
}

func buildServerConnectMenu(ctx *ProgCtx) UIElement {
	serverMenu := w.NewHStack(10)

	ipTextBox := w.NewTextBoxComponent("Server_IPBox", &ctx.State.ServerIP, 16)
	ipTextBoxPanel := w.NewPanelComponent(rl.RayWhite, ipTextBox)
	ipTextBoxCentered := w.NewCenterComponent(ipTextBoxPanel)
	ipTextBoxBounded := w.NewBoundsBox(1, 0.3, ipTextBoxCentered)
	serverMenu.AddChild(ipTextBoxBounded)

	portTextBox := w.NewTextBoxComponent("Server_PortBox", &ctx.State.ServerPort, 6)
	portTextBoxPanel := w.NewPanelComponent(rl.RayWhite, portTextBox)
	portTextBoxCentered := w.NewCenterComponent(portTextBoxPanel)
	portTextBoxBounded := w.NewBoundsBox(1, 0.3, portTextBoxCentered)
	serverMenu.AddChild(portTextBoxBounded)

	confirmBtn := w.NewCenterComponent(w.NewButtonComponent("Server_ConfirmBtn", "Confirm", 150, 50))
	serverMenu.AddChild(confirmBtn)

	serverMenuPanel := w.NewPanelComponent(rl.Gray, serverMenu)
	serverMenuBounds := w.NewBoundsBox(0.9, 0.9, serverMenuPanel)

	return UIElement{dirty: true, component: serverMenuBounds}
}

func buildConnectingScreen(ctx *ProgCtx) UIElement {
	_ = ctx

	connecting := w.NewVStack(10)
	label := w.NewCenterComponent(w.NewLabelComponent("Connecting...", 20, rl.White))
	cancel_btn := w.NewCenterComponent(w.NewButtonComponent("Connecting_CancelBtn", "Cancel", 150, 50))
	connecting.AddChild(label)
	connecting.AddChild(cancel_btn)

	connectingPanel := w.NewPanelComponent(rl.Gray, connecting)
	connectingBounds := w.NewBoundsBox(0.4, 0.4, connectingPanel)

	return UIElement{dirty: true, component: connectingBounds}
}

func buildGameScreen(ctx *ProgCtx) UIElement {
	ctx.StateMutex.RLock()
	defer ctx.StateMutex.RUnlock()

	screen := w.NewGameScreen(10)
	screenPanel := w.NewPanelComponent(rl.Color{R: 20, G: 80, B: 40, A: 255}, screen) // Felt Green

	// 1. Render Community Cards (River)
	screen.ResetRiver()
	for _, card := range ctx.State.Table.CommunityCards {
		screen.AddRiverCard(buildCardComponent(card.Symbol, rl.RayWhite))
	}

	// 2. Render My Hand
	// Note: GameScreen struct in your code only had AddPlayerCard (singular).
	// You might need to loop this or wrap them in an HStack if GameScreen supports it.
	// Assuming we add them to the player bar.

	for _, card := range ctx.State.Table.MyHand {
		screen.AddPlayerCard(buildCardComponent(card.Symbol, rl.Gold))
	}

	readyBtn := w.NewButtonComponent("Game_Ready", "Ready", 100, 50)
	foldBtn := w.NewButtonComponent("Game_Fold", "Fold", 100, 50)
	checkBtn := w.NewButtonComponent("Game_Check", "Check", 100, 50)
	callBtn := w.NewButtonComponent("Game_Call", "Call", 100, 50)

	// Bet needs a way to input amount, for now just a generic button
	betBtn := w.NewButtonComponent("Game_Bet", "Bet 100", 100, 50)
	leaveBtn := w.NewButtonComponent("Game_Leave", "Leave", 100, 50)

	screen.AddActionButton(readyBtn)
	screen.AddActionButton(foldBtn)
	screen.AddActionButton(checkBtn)
	screen.AddActionButton(callBtn)
	screen.AddActionButton(betBtn)
	screen.AddActionButton(leaveBtn)

	return UIElement{dirty: true, component: screenPanel}
}

func buildCardComponent(text string, color rl.Color) w.RGComponent {
	lbl := w.NewCenterComponent(w.NewLabelComponent(text, 20, rl.Black))
	cardPanel := w.NewPanelComponent(color, lbl)

	return w.NewBoundsBox(0.8, 0.8, cardPanel)
}

func buildRoomSelectUI(ctx *ProgCtx) UIElement {
	roomList := w.NewVStack(5)
	roomList.AddChild(w.NewLabelComponent("Select a Room", 24, rl.White))

	ctx.StateMutex.RLock()

	sorted_keys := make([]int, 0, len(ctx.State.Rooms))
	for k := range ctx.State.Rooms {
		sorted_keys = append(sorted_keys, k)
	}

	sort.Ints(sorted_keys)

	rooms := make([]Room, 0, len(ctx.State.Rooms))
	for _, id := range sorted_keys {
		rooms = append(rooms, ctx.State.Rooms[id])
	}

	ctx.StateMutex.RUnlock()

	if len(rooms) == 0 {
		centered_label := w.NewCenterComponent(w.NewLabelComponent("No rooms available.", 18, rl.Gray))
		roomList.AddChild(centered_label)
	}

	for _, room := range rooms {
		roomText := fmt.Sprintf("%s (%d/%d)", room.Name, room.CurrentPlayers, room.MaxPlayers)
		centered_btn := w.NewCenterComponent(w.NewButtonComponent("join_"+strconv.Itoa(room.ID), roomText, 150, 50))
		roomList.AddChild(centered_btn)
	}

	back_btn := w.NewCenterComponent(w.NewButtonComponent("RoomSelect_BackBtn", "Back", 150, 50))
	roomList.AddChild(back_btn)

	roomListPanel := w.NewPanelComponent(rl.DarkBlue, roomList)
	roomBoundsBox := w.NewBoundsBox(0.4, 0.6, roomListPanel)

	return UIElement{dirty: true, component: roomBoundsBox}
}

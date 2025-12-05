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
	ctx.UI.Connecting = buildConnectingScreen(ctx)
	ctx.UI.Reconnecting = buildReconnectingMenu(ctx)
	ctx.UI.Game = buildGameScreen(ctx)
}

func buildMainMenu(ctx *ProgCtx) UIElement {
	_ = ctx

	mainMenu := w.NewVStack(10)

	playerStack := w.NewVStack(10)
	playerLabel := w.NewLabelComponent("Player:", 20, rl.White)

	nickField := w.NewHStack(5)
	nickLabel := w.NewLabelComponent("Nick:", 20, rl.White)
	nickTextBox := buildCenteredTextBox("MainMenu_NickBox", &ctx.State.Nickname, 10000)
	nickField.AddChild(nickLabel)
	nickField.AddChild(nickTextBox)

	chipsField := w.NewHStack(5)
	chipsLabel := w.NewLabelComponent("Chips:", 20, rl.White)
	chipsTextBox := buildCenteredTextBox("MainMenu_NickBox", &ctx.State.ChipsStr, 100)
	chipsField.AddChild(chipsLabel)
	chipsField.AddChild(chipsTextBox)

	playerStack.AddChild(playerLabel)
	playerStack.AddChild(nickField)
	playerStack.AddChild(chipsField)

	serverStack := w.NewVStack(10)
	serverLabel := w.NewLabelComponent("Server:", 20, rl.White)

	ipField := w.NewHStack(5)
	ipLabel := w.NewLabelComponent("IP:", 20, rl.White)
	ipBox := buildCenteredTextBox("Server_IPBox", &ctx.State.ServerIP, 16)
	ipField.AddChild(ipLabel)
	ipField.AddChild(ipBox)

	portField := w.NewHStack(5)
	portLabel := w.NewLabelComponent("Port:", 20, rl.White)
	portBox := buildCenteredTextBox("Server_PortBox", &ctx.State.ServerPort, 6)
	portField.AddChild(portLabel)
	portField.AddChild(portBox)

	serverStack.AddChild(serverLabel)
	serverStack.AddChild(ipField)
	serverStack.AddChild(portField)

	horPS := w.NewHStack(20)
	horPS.AddChild(playerStack)
	horPS.AddChild(serverStack)
	horPSCentered := w.NewCenterComponent(horPS)

	connect_btn := w.NewCenterComponent(w.NewButtonComponent("MainMenu_ConnectBtn", "Connect", 150, 50))
	close_btn := w.NewCenterComponent(w.NewButtonComponent("MainMenu_CloseBtn", "Close", 150, 50))

	mainMenu.AddChild(horPSCentered)
	mainMenu.AddChild(connect_btn)
	mainMenu.AddChild(close_btn)

	mainMenuPanel := w.NewPanelComponent(rl.DarkGray, mainMenu)
	mainMenuBounds := w.NewBoundsBox(0.6, 0.8, mainMenuPanel)

	return UIElement{dirty: true, component: mainMenuBounds}
}

func buildCenteredTextBox(id string, ref *string, maxChars int) w.RGComponent {
	textBox := w.NewTextBoxComponent(id, ref, maxChars)
	textBoxPanel := w.NewPanelComponent(rl.RayWhite, textBox)
	textBoxCentered := w.NewCenterComponent(textBoxPanel)
	textBoxBounded := w.NewBoundsBox(1, 0.5, textBoxCentered)
	return textBoxBounded
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

func buildReconnectingMenu(ctx *ProgCtx) UIElement {
	_ = ctx

	connecting := w.NewVStack(10)
	label := w.NewCenterComponent(w.NewLabelComponent("Reconnect?", 20, rl.White))
	buttons := w.NewHStack(20)
	accept_btn := w.NewCenterComponent(w.NewButtonComponent("Reconnect_Accept", "Accept", 150, 50))
	decline_btn := w.NewCenterComponent(w.NewButtonComponent("Reconnect_Decline", "Decline", 150, 50))

	buttons.AddChild(accept_btn)
	buttons.AddChild(decline_btn)

	connecting.AddChild(label)
	connecting.AddChild(buttons)

	connectingPanel := w.NewPanelComponent(rl.Gray, connecting)
	connectingBounds := w.NewBoundsBox(0.4, 0.4, connectingPanel)

	return UIElement{dirty: true, component: connectingBounds}
}

func buildGameScreen(ctx *ProgCtx) UIElement {
	ctx.StateMutex.RLock()
	defer ctx.StateMutex.RUnlock()

	screen := w.NewGameScreen(10)
	screenPanel := w.NewPanelComponent(rl.Color{R: 20, G: 80, B: 40, A: 255}, screen)

	pot := w.NewPotDisplayComponent(ctx.State.Table.Pot, ctx.State.Table.HighBet)
	screen.SetPotDisplay(pot)

	screen.ResetRiver()
	for _, card := range ctx.State.Table.CommunityCards {
		screen.AddRiverCard(buildCardComponent(card.Symbol, rl.RayWhite))
	}

	screen.ResetOtherPlayers()

	// Sort players by name for consistent display
	var playerNames []string
	for name := range ctx.State.Table.Players {
		if name != ctx.State.Nickname {
			playerNames = append(playerNames, name)
		}
	}
	sort.Strings(playerNames)

	for _, name := range playerNames {
		player := ctx.State.Table.Players[name]
		info := w.NewPlayerInfoComponent(player.IsMyTurn)
		info.AddDesc(w.NewLabelComponent(name, 12, rl.White))
		info.AddDesc(w.NewLabelComponent(fmt.Sprintf("Chips: %d", player.ChipCount), 12, rl.Yellow))
		if player.TotalBet > 0 {
			info.AddDesc(w.NewLabelComponent(fmt.Sprintf("Total Bet: %d", player.TotalBet), 12, rl.Orange))
		}

		// Show status if not active
		if player.ActionTaken != "NONE" {
			info.AddDesc(w.NewLabelComponent(fmt.Sprintf("%s %d", player.ActionTaken, player.ActionAmount), 12, rl.White))
		} else if player.IsFolded {
			info.AddDesc(w.NewLabelComponent("Folded", 12, rl.White))
		} else if player.IsReady {
			info.AddDesc(w.NewLabelComponent("Ready", 12, rl.White))
		}

		for _, c := range player.Cards {
			if c.Hidden {
				info.AddCard(buildHiddenCardComponent())
			} else {
				info.AddCard(buildCardComponent(c.Symbol, rl.RayWhite))
			}
		}
		screen.AddOtherPlayer(info)
	}

	myData, exists := ctx.State.Table.Players[ctx.State.Nickname]

	for _, card := range myData.Cards {
		screen.AddPlayerCard(buildCardComponent(card.Symbol, rl.Gold))
	}

	showActions := exists && myData.IsMyTurn && !myData.IsFolded

	if !myData.IsReady {
		readyBtn := w.NewButtonComponent("Game_Ready", "Ready", 100, 50)
		screen.AddActionButton(readyBtn)
	}

	if ctx.State.Showdown {
		showdownOkBtn := w.NewButtonComponent("Game_ShowOK", "OK", 100, 50)
		screen.AddActionButton(showdownOkBtn)
	} else {
		if showActions {
			if ctx.State.Table.HighBet == 0 {
				checkBtn := w.NewButtonComponent("Game_Check", "Check", 100, 50)
				screen.AddActionButton(checkBtn)

				betStack := w.NewHStack(0)
				betBox := w.NewTextBoxComponent("Game_BetAmount", &ctx.State.BetAmount, 6)
				betBtn := w.NewButtonComponent("Game_Bet", "Bet", 100, 50)
				betStack.AddChild(betBtn)
				betStack.AddChild(betBox)
				screen.AddActionButton(betStack)
			}

			if ctx.State.Table.HighBet > 0 {
				callBtn := w.NewButtonComponent("Game_Call", fmt.Sprintf("Call %d", ctx.State.Table.HighBet), 100, 50)
				screen.AddActionButton(callBtn)
			}

			foldBtn := w.NewButtonComponent("Game_Fold", "Fold", 100, 50)
			screen.AddActionButton(foldBtn)
		}
	}

	leaveBtn := w.NewButtonComponent("Game_Leave", "Leave", 100, 50)
	screen.AddActionButton(leaveBtn)

	return UIElement{dirty: true, component: screenPanel}
}

func buildCardComponent(text string, color rl.Color) w.RGComponent {
	lbl := w.NewCenterComponent(w.NewLabelComponent(text, 20, rl.Black))
	return w.NewPanelComponent(color, lbl)
}

func buildHiddenCardComponent() w.RGComponent {
	lbl := w.NewCenterComponent(w.NewLabelComponent("??", 20, rl.White))
	return w.NewPanelComponent(rl.Red, lbl)
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

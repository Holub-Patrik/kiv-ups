package main

import (
	"fmt"
	"sort"
	"strconv"

	w "poker-client/window"

	rl "github.com/gen2brain/raylib-go/raylib"
)

func buildUI(ctx *ProgCtx) {
	buildMainMenu(ctx)
	buildServerConnectMenu(ctx)
	buildConnectingScreen(ctx)
}

func buildMainMenu(ctx *ProgCtx) {
	mainMenu := w.NewVStack(10)
	connect_btn := w.NewCenterComponent(w.NewButtonComponent("MainMenu_ConnectBtn", "Connect", 150, 50))
	close_btn := w.NewCenterComponent(w.NewButtonComponent("MainMenu_CloseBtn", "Close", 150, 50))
	mainMenu.AddChild(connect_btn)
	mainMenu.AddChild(close_btn)

	mainMenuPanel := w.NewPanelComponent(rl.DarkGray, mainMenu)
	mainMenuBounds := w.NewBoundsBox(0.6, 0.8, mainMenuPanel)

	ctx.UI.MainMenu = UIElement{dirty: true, component: mainMenuBounds}
}

func buildServerConnectMenu(ctx *ProgCtx) {
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

	ctx.UI.ServerSelect = UIElement{dirty: true, component: serverMenuBounds}
}

func buildConnectingScreen(ctx *ProgCtx) {
	connecting := w.NewVStack(10)
	label := w.NewCenterComponent(w.NewLabelComponent("Connecting...", 20, rl.White))
	cancel_btn := w.NewCenterComponent(w.NewButtonComponent("Connecting_CancelBtn", "Cancel", 150, 50))
	connecting.AddChild(label)
	connecting.AddChild(cancel_btn)

	connectingPanel := w.NewPanelComponent(rl.Gray, connecting)
	connectingBounds := w.NewBoundsBox(0.4, 0.4, connectingPanel)

	ctx.UI.Connecting = UIElement{dirty: true, component: connectingBounds}
}

func buildRoomSelectUI(ctx *ProgCtx) w.RGComponent {
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

	return roomBoundsBox
}

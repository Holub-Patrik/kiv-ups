package main

import (
	unet "poker-client/ups_net"
	w "poker-client/window" // Your new compose library

	"fmt"
	"strings"
	"sync"

	rl "github.com/gen2brain/raylib-go/raylib"
)

// --- gamethread.go is unchanged and still required ---

// initProgCtx sets up the entire application context.
func initProgCtx() ProgCtx {
	ctx := ProgCtx{}

	// Init Channels
	ctx.UserInputChan = make(chan UserInputEvent, 10) // Buffered
	ctx.DoneChan = make(chan bool)

	// Init State
	ctx.State.Screen = ScreenMainMenu
	ctx.State.Rooms = make(map[string]Room)
	ctx.StateMutex = sync.RWMutex{}
	ctx.State.ServerIP = "127.0.0.1"
	ctx.State.ServerPort = "8080"

	// Init Network
	ctx.NetHandler = unet.InitNetHandler()

	// Init UI
	buildUI(&ctx)

	return ctx
}

func buildUI(ctx *ProgCtx) {
	mainMenu := w.NewVStack(10)
	connect_btn := w.NewCenterComponent(w.NewButtonComponent("MainMenu_ConnectBtn", "Connect", 150, 50))
	close_btn := w.NewCenterComponent(w.NewButtonComponent("MainMenu_CloseBtn", "Close", 150, 50))
	mainMenu.AddChild(connect_btn)
	mainMenu.AddChild(close_btn)

	mainMenuPanel := w.NewPanelComponent(rl.DarkGray, mainMenu)

	ctx.UI.MainMenu = mainMenuPanel

	serverMenu := w.NewHStack(10)
	serverMenu.AddChild(w.NewTextBoxComponent("Server_IPBox", &ctx.State.ServerIP, &ctx.StateMutex, 16))
	serverMenu.AddChild(w.NewTextBoxComponent("Server_PortBox", &ctx.State.ServerPort, &ctx.StateMutex, 6))
	confirm_btn := w.NewCenterComponent(w.NewButtonComponent("Server_ConfirmBtn", "Confirm", 150, 50))
	serverMenu.AddChild(confirm_btn)

	serverMenuPanel := w.NewPanelComponent(rl.Gray, serverMenu)

	ctx.UI.ServerSelect = serverMenuPanel

	connecting := w.NewVStack(10)
	label := w.NewCenterComponent(w.NewLabelComponent("Connecting...", 20, rl.White))
	cancel_btn := w.NewCenterComponent(w.NewButtonComponent("Connecting_CancelBtn", "Cancel", 150, 50))
	connecting.AddChild(label)
	connecting.AddChild(cancel_btn)

	connectingVPanel := w.NewPanelComponent(rl.Gray, connecting)

	ctx.UI.Connecting = connectingVPanel
}

func buildRoomSelectUI(ctx *ProgCtx) w.RGComponent {
	roomList := w.NewVStack(5)
	roomList.AddChild(w.NewLabelComponent("Select a Room", 24, rl.White))

	ctx.StateMutex.RLock()
	// Copy to local slice to avoid holding lock
	rooms := make([]Room, 0, len(ctx.State.Rooms))
	for _, room := range ctx.State.Rooms {
		rooms = append(rooms, room)
	}
	ctx.StateMutex.RUnlock()

	if len(rooms) == 0 {
		roomList.AddChild(w.NewLabelComponent("No rooms available.", 18, rl.Gray))
	}

	for _, room := range rooms {
		roomText := fmt.Sprintf("%s (%d/%d)", room.Name, room.CurrentPlayers, room.MaxPlayers)
		roomList.AddChild(w.NewButtonComponent("join_"+room.ID, roomText, 150, 50))
	}

	roomList.AddChild(w.NewButtonComponent("RoomSelect_BackBtn", "Back", 150, 50))

	return roomList
}

func handleUIEvent(ctx *ProgCtx, event w.UIEvent) {
	switch event.SourceID {
	case "MainMenu_ConnectBtn":
		ctx.StateMutex.Lock()
		ctx.State.Screen = ScreenServerSelect
		ctx.StateMutex.Unlock()

	case "MainMenu_CloseBtn":
		ctx.UserInputChan <- EvtQuitClicked{}

	case "Server_ConfirmBtn":
		// Get the IP/Port safely from the state
		ctx.StateMutex.RLock()
		host := ctx.State.ServerIP
		port := ctx.State.ServerPort
		ctx.StateMutex.RUnlock()
		ctx.UserInputChan <- EvtConnectClicked{Host: host, Port: port}

	case "Connecting_CancelBtn":
		ctx.UserInputChan <- EvtCancelConnectClicked{}

	case "RoomSelect_BackBtn":
		// TODO: Tell game thread to disconnect or go back
		ctx.StateMutex.Lock()
		ctx.State.Screen = ScreenMainMenu
		ctx.StateMutex.Unlock()

	default:
		after, found := strings.CutPrefix(event.SourceID, "join_")
		if found {
			ctx.UserInputChan <- EvtRoomJoinClicked{RoomID: after}
		}
	}
}

func main() {
	rl.SetConfigFlags(rl.FlagWindowResizable)
	var (
		screenWidth  int32 = 1600
		screenHeight int32 = 1000
	)
	rl.InitWindow(screenWidth, screenHeight, "Test client")
	defer rl.CloseWindow()

	rl.SetTargetFPS(60)

	ctx := initProgCtx()

	// Start the "Game Thread"
	go gameThread(&ctx)

	componentsToDraw := make([]w.RGComponent, 0)

	for !rl.WindowShouldClose() && !ctx.ShouldClose {
		// --- Recalculation ---
		screenBounds := rl.Rectangle{
			X: 0, Y: 0,
			Width:  float32(rl.GetScreenWidth()),
			Height: float32(rl.GetScreenHeight()),
		}

		// Get the current screen safely
		ctx.StateMutex.RLock()
		currentScreen := ctx.State.Screen
		ctx.StateMutex.RUnlock()

		switch currentScreen {
		case ScreenMainMenu:
			ctx.UI.MainMenu.Calculate(screenBounds)
			componentsToDraw = append(componentsToDraw, ctx.UI.MainMenu)

		case ScreenServerSelect:
			ctx.UI.MainMenu.Calculate(screenBounds)
			ctx.UI.ServerSelect.Calculate(ctx.UI.MainMenu.GetBounds())
			componentsToDraw = append(componentsToDraw, ctx.UI.MainMenu, ctx.UI.ServerSelect)

		case ScreenConnecting:
			ctx.UI.MainMenu.Calculate(screenBounds)
			ctx.UI.Connecting.Calculate(ctx.UI.MainMenu.GetBounds())
			componentsToDraw = append(componentsToDraw, ctx.UI.MainMenu, ctx.UI.Connecting)

		case ScreenRoomSelect:
			roomSelect := buildRoomSelectUI(&ctx)
			roomSelect.Calculate(screenBounds)
			componentsToDraw = append(componentsToDraw, ctx.UI.MainMenu, roomSelect)

		case ScreenInGame:
		}

		rl.BeginDrawing()
		rl.DrawFPS(0, 0)
		rl.ClearBackground(rl.Black)

		uiEventChannel := make(chan w.UIEvent, 10)
		for _, component := range componentsToDraw {
			component.Draw(uiEventChannel)
		}

		componentsToDraw = componentsToDraw[:0]

		rl.EndDrawing()

		close(uiEventChannel)
		for event := range uiEventChannel {
			handleUIEvent(&ctx, event)
		}
	}

	// --- Shutdown ---
	ctx.ShouldClose = true
	ctx.UserInputChan <- EvtQuitClicked{} // Wake up game thread
	ctx.NetHandler.Close()                // Close network connection

	fmt.Println("Main: Waiting for GameThread to shut down...")
	<-ctx.DoneChan
	fmt.Println("Main: Shutdown complete.")
}

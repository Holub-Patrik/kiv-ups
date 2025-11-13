// main.go
package main

import (
	unet "poker-client/ups_net" // Your alias
	w "poker-client/window"     // Your alias

	"fmt"
	"sync"

	rg "github.com/gen2brain/raylib-go/raygui"
	rl "github.com/gen2brain/raylib-go/raylib"
)

// --- Include the new data.go and gamethread.go files in your build ---

// --- Your existing ProgCtx, M_Main, etc. are replaced by data.go ---
// --- Your existing NetworkCtx, Room, Player, etc. are replaced by data.go ---

// initProgCtx sets up the entire application context.
func initProgCtx() ProgCtx {
	ctx := ProgCtx{}

	// Init Channels
	ctx.UserInputChan = make(chan UserInputEvent, 10) // Buffered
	// NetMsgInChan and NetMsgOutChan will be set by the GameThread on connection
	ctx.DoneChan = make(chan bool)

	// Init State
	ctx.State.Screen = ScreenMainMenu
	ctx.State.Rooms = make(map[string]Room)
	ctx.StateMutex = sync.RWMutex{}

	// Init Network
	ctx.NetHandler = unet.InitNetHandler()

	// Init UI Layout
	ctx.Layout = make(map[string]rl.Rectangle)
	// You will call your 'calculate' function here
	calculateLayout(&ctx) // We'll adapt your 'calculate'

	return ctx
}

// calculateLayout (replaces your 'calculate' receiver)
func calculateLayout(ctx *ProgCtx) {
	// This function now just calculates rectangles and stores them
	// in ctx.Layout. It doesn't modify menu structs.
	screenWidth := float32(rl.GetScreenWidth())
	screenHeight := float32(rl.GetScreenHeight())

	// Example:
	mainMenuCont := w.GetVerticalMenuRect(0, 0, screenWidth, screenHeight)
	mainMenuButtons := w.GetMenuButtonsVertical(mainMenuCont, 2)
	ctx.Layout["MainMenu_ConnectBtn"] = mainMenuButtons[0]
	ctx.Layout["MainMenu_CloseBtn"] = mainMenuButtons[1]

	serverMenuCont := w.GetHorizontalMenuRect(0, 0, screenWidth, screenHeight)
	serverMenuButtons := w.GetMenuButtonsHorizontal(serverMenuCont, 3)
	ctx.Layout["Server_ContRec"] = serverMenuCont
	ctx.Layout["Server_IPBox"] = serverMenuButtons[0]
	ctx.Layout["Server_PortBox"] = serverMenuButtons[1]
	ctx.Layout["Server_ConfirmBtn"] = serverMenuButtons[2]

	// ... etc. for all UI elements
}

// --- Main Drawing Functions ---
// These now read from ctx.State and ctx.Layout

func drawMainMenu(ctx *ProgCtx) {
	// Draw background, etc.
	// rl.DrawRectangleRec(ctx.Layout["MainMenu_ContRec"], rl.DarkGray)

	if rg.Button(ctx.Layout["MainMenu_ConnectBtn"], "Connect") {
		// Don't change state! Send event.
		ctx.StateMutex.Lock()
		ctx.State.Screen = ScreenServerSelect
		ctx.StateMutex.Unlock()
	}
	if rg.Button(ctx.Layout["MainMenu_CloseBtn"], "Close") {
		ctx.UserInputChan <- EvtQuitClicked{}
	}
}

func drawServerMenu(ctx *ProgCtx) {
	// Draw background
	rl.DrawRectangleRec(ctx.Layout["Server_ContRec"], rl.Gray)

	// This is tricky. raygui's TextBox modifies the string directly.
	// We need to lock, copy, unlock, draw, lock, update, unlock.
	// A simpler way for now:
	ctx.StateMutex.Lock()
	// This is still a bit racy, but better.
	// A more robust solution would be to buffer input locally in main.go.
	rg.TextBox(ctx.Layout["Server_IPBox"], &ctx.State.ServerIP, 16, true)
	rg.TextBox(ctx.Layout["Server_PortBox"], &ctx.State.ServerPort, 6, true)
	ip := ctx.State.ServerIP
	port := ctx.State.ServerPort
	ctx.StateMutex.Unlock()

	if rg.Button(ctx.Layout["Server_ConfirmBtn"], "Confirm") {
		// Send the input to the game thread for processing
		ctx.UserInputChan <- EvtConnectClicked{Host: ip, Port: port}
	}
	// TODO: Add a "Back" button
}

func drawConnectingMenu(ctx *ProgCtx) {
	// This menu is purely informational.
	// The GameThread will change the state when ready.
	// rl.DrawRectangleRec(...)
	rl.DrawText("Connecting...", 100, 100, 20, rl.White)

	// Optionally, add a cancel button
	if rg.Button(ctx.Layout["Connecting_CancelBtn"], "Cancel") {
		ctx.UserInputChan <- EvtCancelConnectClicked{}
	}
}

func drawRoomSelect(ctx *ProgCtx) {
	// Read-lock the state to get the room list
	ctx.StateMutex.RLock()
	// Copy the rooms to a local slice to avoid holding the lock while drawing
	rooms := make([]Room, 0, len(ctx.State.Rooms))
	for _, room := range ctx.State.Rooms {
		rooms = append(rooms, room)
	}
	ctx.StateMutex.RUnlock()

	// Now, iterate over the local 'rooms' slice and draw them
	// ...
	y := 100
	for _, room := range rooms {
		// roomText
		_ = fmt.Sprintf("%s (%d/%d)", room.Name, room.CurrentPlayers, room.MaxPlayers)
		// Draw the roomText and a "Join" button
		// if rg.Button(rl.Rectangle{...}, "Join") {
		// 	  ctx.UserInputChan <- EvtRoomJoinClicked{RoomID: room.ID}
		// }
		y += 40
	}
}

// --- Main Function ---

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

	for !rl.WindowShouldClose() && !ctx.ShouldClose {
		// --- Recalculation (Your logic is fine) ---
		if rl.IsWindowResized() {
			calculateLayout(&ctx)
		}

		// --- Drawing ---
		rl.BeginDrawing()
		rl.DrawFPS(0, 0)
		rl.ClearBackground(rl.Black)

		// Get the current screen safely
		ctx.StateMutex.RLock()
		currentScreen := ctx.State.Screen
		ctx.StateMutex.RUnlock()

		// Draw based on state
		switch currentScreen {
		case ScreenMainMenu:
			drawMainMenu(&ctx)
		case ScreenServerSelect:
			// You can overlay menus
			drawMainMenu(&ctx)
			drawServerMenu(&ctx)
		case ScreenConnecting:
			drawMainMenu(&ctx) // Draw main menu dimmed?
			drawConnectingMenu(&ctx)
		case ScreenRoomSelect:
			drawRoomSelect(&ctx)
		case ScreenInGame:
			// drawGame(&ctx)
		case ScreenError:
			// drawError(&ctx)
		}

		rl.EndDrawing()
	}

	// --- Shutdown ---
	// Signal GameThread to stop (if not already)
	ctx.ShouldClose = true
	// Send a dummy event to wake it up if it's sleeping on the channel
	ctx.UserInputChan <- EvtQuitClicked{}

	fmt.Println("Main: Waiting for GameThread to shut down...")
	// Wait for GameThread to confirm shutdown
	<-ctx.DoneChan
	fmt.Println("Main: Shutdown complete.")
}

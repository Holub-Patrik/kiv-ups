package main

import (
	"fmt"
	unet "poker-client/ups_net"
	"time"

	rg "github.com/gen2brain/raylib-go/raygui"
	rl "github.com/gen2brain/raylib-go/raylib"
)

func initProgCtx() ProgCtx {
	ctx := ProgCtx{}

	ctx.calculate()

	ctx.main_menu.back_col = rl.DarkGray
	ctx.server_menu.back_col = rl.Gray
	ctx.conn_menu.back_col = rl.Gray

	ctx.server_menu.active_input = 0
	ctx.close = false
	ctx.state = Main

	ctx.network = initNetworkCtx()
	ctx.done_chan = make(chan bool)

	return ctx
}

func initNetworkCtx() NetworkCtx {
	ctx := NetworkCtx{}
	ctx.handler = unet.InitNetHandler()
	ctx.state = Disconnected
	return ctx
}

func (ctx *ProgCtx) handleMainMenu() {
	ctx.drawMainMenu()

	connect_pressed := rg.Button(ctx.main_menu.connect_box, "Connect")
	close_pressed := rg.Button(ctx.main_menu.close_box, "Close")

	if close_pressed {
		ctx.close = true
		return
	}

	if connect_pressed {
		ctx.state = ServerSelect
	}
}

func (ctx *ProgCtx) handleServerMenu() {
	ctx.drawMainMenu()
	ctx.drawServerMenu()

	editablePort := false
	editableIP := false

	if rl.IsKeyPressed(rl.KeyTab) {
		ctx.server_menu.active_input++
	}
	active_input := ctx.server_menu.active_input

	isMouseOnIPBox := rl.CheckCollisionPointRec(rl.GetMousePosition(), ctx.server_menu.ip_input_box)
	isMouseOnPortBox := rl.CheckCollisionPointRec(rl.GetMousePosition(), ctx.server_menu.port_input_box)

	if isMouseOnIPBox {
		active_input = 0
	}

	if isMouseOnPortBox {
		active_input = 1
	}

	switch active_input % 2 {
	case 0:
		editableIP = true
	case 1:
		editablePort = true
	}

	rg.TextBox(ctx.server_menu.ip_input_box, &ctx.network.host, 16, editableIP)
	rg.TextBox(ctx.server_menu.port_input_box, &ctx.network.port, 6, editablePort)

	confirm_clicked := rg.Button(ctx.server_menu.confirm_box, "Confirm")
	if confirm_clicked {
		go func() {
			success := ctx.network.handler.Connect(ctx.network.host, ctx.network.port)
			ctx.done_chan <- success
		}()
		ctx.state = Connecting
	}
}

func (ctx *ProgCtx) handleConnectingMenu() {
	ctx.drawMainMenu()
	ctx.drawConnectingMenu()

	success, done := <-ctx.done_chan
	if !done {
		fmt.Println("Still Connecting")
		return
	}

	if !success {
		ctx.state = ServerSelect
	} else {
		go ctx.network.handler.Run()
		go ctx.gameThread()
	}
}

// I need to show the 3 other player
// My player
// Choose action buttons
// TextInput for bet
// Button for bet
// Button for fold
// Button for check
// Button for all-in
// Button for disconnect
func (ctx *ProgCtx) drawGame() {}

func (ctx *ProgCtx) gameThread() {
	// first connect stuffs, then onto the game loop
	for {
		switch ctx.state {
		case Connecting:
			ctx.network.handler.SendMessage(unet.NetMsg{Code: "CONN", Payload: ""})
			msg, ok := ctx.network.handler.GetMessage(time.Minute)
			// server didn't answer
			if !ok {
				ctx.state = Main
				return
			}

			// sever didn't send ok
			if msg.Code != "00OK" {
				ctx.state = Main
				return
			}
			ctx.state = AskingForRooms
		case AskingForRooms:
			ctx.network.handler.SendMessage(unet.NetMsg{Code: "ROMS", Payload: ""})
			// here I have to receive data until
			rooms := make([]unet.NetMsg, 0)
			for {
				msg, ok := ctx.network.handler.GetMessage(time.Minute)
				if !ok {
					ctx.state = Main
					return
				}

				if msg.Code == "ROOM" {
					rooms = append(rooms, msg)
				}

				if msg.Code == "DONE" {
					break
				}
			}

			ctx.state = RoomSelect

		case RoomSelect:
			// here I need to be checking if any of the rooms change, so I change the relevant data in the list
		}
	}
}

func (ctx *ProgCtx) handleRoomAsk() {}

func (ctx *ProgCtx) handleRoomSelect() {}

func main() {
	rl.SetConfigFlags(rl.FlagWindowResizable)
	var (
		screen_width  int32 = 1600
		screen_height int32 = 1000
	)
	rl.InitWindow(screen_width, screen_height, "Test client")

	defer rl.CloseWindow()

	rl.SetTargetFPS(60)

	ctx := initProgCtx()

	var possibly_new_screen_width int32 = screen_width
	var possibly_new_screen_height int32 = screen_height

	for !rl.WindowShouldClose() && !ctx.close {
		{ // recalculation
			possibly_new_screen_width = int32(rl.GetScreenWidth())
			possibly_new_screen_height = int32(rl.GetScreenHeight())

			should_recalc := false
			if possibly_new_screen_width != screen_width {
				should_recalc = true
				screen_width = possibly_new_screen_width
			}

			if possibly_new_screen_height != screen_height {
				should_recalc = true
				screen_height = possibly_new_screen_height
			}

			if should_recalc {
				ctx.calculate()
			}
		}

		{ // drawing
			rl.BeginDrawing()
			rl.DrawFPS(0, 0)
			rl.ClearBackground(rl.Black)

			switch ctx.state {
			case Main:
				ctx.handleMainMenu()
			case ServerSelect:
				ctx.handleServerMenu()
			case Connecting:
				ctx.handleConnectingMenu()
			case AskingForRooms:
				ctx.handleRoomAsk()
			case RoomSelect:
				ctx.handleRoomSelect()
			case Game:
				ctx.drawGame()
			default:
				ctx.close = true
			}
			rl.EndDrawing()
		}
	}
}

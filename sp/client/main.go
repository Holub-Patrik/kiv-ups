package main

import (
	"fmt"

	rg "github.com/gen2brain/raylib-go/raygui"
	rl "github.com/gen2brain/raylib-go/raylib"
)

type Player struct {
	nick   string
	tokens uint64
}

type State int
type NetworkState int

const (
	Main State = iota
	ServerSelect
	Connecting
)

const (
	Disconnected NetworkState = iota
	FailedToConnect
	Connected
	ConnTimeout
)

type ProgCtx struct {
	window_changed bool
	close          bool
	state          State
	main_menu      M_Main
	server_menu    M_Server
	conn_menu      M_Connecting
	network        NetworkCtx
}

type NetworkCtx struct {
	host  string
	port  string
	state NetworkState
}

type M_Main struct {
	cont_rec    rl.Rectangle
	back_col    rl.Color
	connect_box rl.Rectangle
	close_box   rl.Rectangle
}

type M_Server struct {
	active_input   int
	cont_rec       rl.Rectangle
	back_col       rl.Color
	ip_input_box   rl.Rectangle
	port_input_box rl.Rectangle
	confirm_box    rl.Rectangle
}

type M_Connecting struct {
	cont_rec   rl.Rectangle
	back_col   rl.Color
	state_box  rl.Rectangle
	cancel_box rl.Rectangle
}

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

	return ctx
}

func initNetworkCtx() NetworkCtx {
	ctx := NetworkCtx{}
	ctx.state = Disconnected
	return ctx
}

func (ctx *ProgCtx) calculate() {
	screen_width := rl.GetScreenWidth()
	screen_height := rl.GetScreenHeight()

	ctx.main_menu.cont_rec = getVerticalMenuRect(
		0,
		0,
		screen_width,
		screen_height,
	)

	ctx.server_menu.cont_rec = getHorizontalMenuRect(
		ctx.main_menu.cont_rec.X,
		ctx.main_menu.cont_rec.Y,
		ctx.main_menu.cont_rec.Width,
		ctx.main_menu.cont_rec.Height,
	)

	ctx.conn_menu.cont_rec = getHorizontalMenuRect(
		ctx.main_menu.cont_rec.X,
		ctx.main_menu.cont_rec.Y,
		ctx.main_menu.cont_rec.Width,
		ctx.main_menu.cont_rec.Height,
	)

	main_menu_buttons := getMenuButtonsVertical(ctx.main_menu.cont_rec, 2)
	ctx.main_menu.connect_box = main_menu_buttons[0]
	ctx.main_menu.close_box = main_menu_buttons[1]

	connect_menu_buttons := getMenuButtonsHorizontal(ctx.server_menu.cont_rec, 3)
	ctx.server_menu.ip_input_box = connect_menu_buttons[0]
	ctx.server_menu.port_input_box = connect_menu_buttons[1]
	ctx.server_menu.confirm_box = connect_menu_buttons[2]

	conn_menu_buttons := getMenuButtonsVertical(ctx.conn_menu.cont_rec, 2)
	ctx.conn_menu.state_box = conn_menu_buttons[0]
	ctx.conn_menu.cancel_box = conn_menu_buttons[1]
}

func (ctx *ProgCtx) drawMainMenu() {
	rl.DrawRectangleRec(ctx.main_menu.cont_rec, ctx.main_menu.back_col)
	rl.DrawRectangleLinesEx(ctx.main_menu.cont_rec, 5, rl.White)
	rl.DrawRectangleLinesEx(ctx.main_menu.connect_box, 5, rl.White)
	rl.DrawRectangleLinesEx(ctx.main_menu.close_box, 5, rl.White)
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

func (ctx *ProgCtx) drawServerMenu() {
	rl.DrawRectangleRec(ctx.server_menu.cont_rec, ctx.server_menu.back_col)
	rl.DrawRectangleLinesEx(ctx.server_menu.cont_rec, 5, rl.White)
	rl.DrawRectangleLinesEx(ctx.server_menu.ip_input_box, 5, rl.White)
	rl.DrawRectangleLinesEx(ctx.server_menu.port_input_box, 5, rl.White)
	rl.DrawRectangleLinesEx(ctx.server_menu.confirm_box, 5, rl.White)
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
		fmt.Printf("Pressed connect: %s:%s\n", ctx.network.host, ctx.network.port)
		ctx.state = Connecting
	}
}

func (ctx *ProgCtx) drawConnectingMenu() {
	rl.DrawRectangleRec(ctx.conn_menu.cont_rec, ctx.conn_menu.back_col)
	rl.DrawRectangleLinesEx(ctx.conn_menu.cancel_box, 5, rl.White)
	rl.DrawRectangleLinesEx(ctx.conn_menu.state_box, 5, rl.White)
	rl.DrawText("Connecting...", int32(ctx.conn_menu.state_box.X), int32(ctx.conn_menu.state_box.Y), 20, rl.White)
}

func (ctx *ProgCtx) handleConnectingMenu() {
	ctx.drawMainMenu()
	ctx.drawConnectingMenu()
}

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
		default:
			ctx.close = true
		}
		rl.EndDrawing()
	}

}

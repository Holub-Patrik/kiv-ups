package main

import (
	rg "github.com/gen2brain/raylib-go/raygui"
	rl "github.com/gen2brain/raylib-go/raylib"
)

type Player struct {
	nick   string
	tokens uint64
}

type State int

const (
	MainMenu State = iota
	ServerMenu
)

type ProgCtx struct {
	close     bool
	state     State
	ip        string
	port      string
	main_menu MainMenuData
	conn_menu ConnectMenuData
}

type MainMenuData struct {
	cont_rec    rl.Rectangle
	back_col    rl.Color
	connect_rec rl.Rectangle
	close_rec   rl.Rectangle
}

type ConnectMenuData struct {
	cont_rec       rl.Rectangle
	back_col       rl.Color
	ip_input_box   rl.Rectangle
	port_input_box rl.Rectangle
}

func initProgCtx() ProgCtx {
	ret := ProgCtx{}

	ret.main_menu.cont_rec = rl.Rectangle{
		X:      100,
		Y:      100,
		Width:  400,
		Height: 600,
	}

	ret.main_menu.connect_rec = rl.Rectangle{
		X:      ret.main_menu.cont_rec.X + 10,
		Y:      ret.main_menu.cont_rec.Y + 10,
		Width:  (ret.main_menu.cont_rec.Width - 20),
		Height: 50,
	}

	ret.main_menu.close_rec = rl.Rectangle{
		X:      ret.main_menu.connect_rec.X,
		Y:      ret.main_menu.connect_rec.Y + ret.main_menu.connect_rec.Height + 10,
		Width:  ret.main_menu.connect_rec.Width,
		Height: ret.main_menu.connect_rec.Height,
	}

	ret.main_menu.back_col = rl.DarkGray

	ret.conn_menu.cont_rec = rl.Rectangle{
		X:      250,
		Y:      250,
		Width:  500,
		Height: 300,
	}

	ret.conn_menu.back_col = rl.Gray

	ret.conn_menu.ip_input_box = rl.Rectangle{
		X:      ret.conn_menu.cont_rec.X + 10,
		Y:      ret.conn_menu.cont_rec.Y + 10,
		Width:  150,
		Height: (ret.conn_menu.cont_rec.Height - 20),
	}

	ret.conn_menu.port_input_box = rl.Rectangle{
		X:      ret.conn_menu.ip_input_box.X + ret.conn_menu.ip_input_box.Width + 10,
		Y:      ret.conn_menu.ip_input_box.Y,
		Width:  50,
		Height: ret.conn_menu.ip_input_box.Height,
	}

	ret.close = false
	ret.state = MainMenu

	ret.ip = string(make([]byte, 0, 15))
	ret.port = string(make([]byte, 0, 5))

	return ret
}

func (ctx *ProgCtx) drawMainMenu() {
	rl.DrawRectangleRec(ctx.main_menu.cont_rec, ctx.main_menu.back_col)
	rl.DrawRectangleLinesEx(ctx.main_menu.cont_rec, 5, rl.White)
	rl.DrawRectangleLinesEx(ctx.main_menu.connect_rec, 5, rl.White)
	rl.DrawRectangleLinesEx(ctx.main_menu.close_rec, 5, rl.White)
}

func (ctx *ProgCtx) handleMainMenu() {
	ctx.drawMainMenu()

	connect_pressed := rg.Button(ctx.main_menu.connect_rec, "Connect")
	close_pressed := rg.Button(ctx.main_menu.close_rec, "Close")

	if close_pressed {
		ctx.close = true
		return
	}

	if connect_pressed {
		ctx.state = ServerMenu
	}
}

func (ctx *ProgCtx) drawServerMenu() {
	rl.DrawRectangleRec(ctx.conn_menu.cont_rec, ctx.conn_menu.back_col)
	rl.DrawRectangleLinesEx(ctx.conn_menu.cont_rec, 5, rl.White)
	rl.DrawRectangleLinesEx(ctx.conn_menu.ip_input_box, 5, rl.White)
	rl.DrawRectangleLinesEx(ctx.conn_menu.port_input_box, 5, rl.White)
}

func (ctx *ProgCtx) handleServerMenu() {
	ctx.drawMainMenu()
	ctx.drawServerMenu()

	editablePort := false
	editableIP := false
	isMouseOnIPBox := rl.CheckCollisionPointRec(rl.GetMousePosition(), ctx.conn_menu.ip_input_box)
	isMouseOnPortBox := rl.CheckCollisionPointRec(rl.GetMousePosition(), ctx.conn_menu.port_input_box)
	if isMouseOnIPBox {
		editableIP = true
		editablePort = false
	}
	if isMouseOnPortBox {
		editablePort = true
		editableIP = false
	}

	rg.TextBox(ctx.conn_menu.ip_input_box, &ctx.ip, 15, editableIP)

	rg.TextBox(ctx.conn_menu.port_input_box, &ctx.port, 5, editablePort)

}

func main() {
	const (
		screenWidth  = 800
		screenHeight = 600
	)
	rl.InitWindow(1600, 1000, "Test client")
	defer rl.CloseWindow()

	ctx := initProgCtx()

	for !rl.WindowShouldClose() && !ctx.close {
		rl.BeginDrawing()
		rl.DrawFPS(0, 0)
		rl.ClearBackground(rl.Black)

		switch ctx.state {
		case MainMenu:
			ctx.handleMainMenu()
		case ServerMenu:
			ctx.handleServerMenu()
		default:
			ctx.close = true
		}
		rl.EndDrawing()
	}

}

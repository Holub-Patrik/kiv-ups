package main

import (
	w "poker-client/window"

	rl "github.com/gen2brain/raylib-go/raylib"
)

func (ctx *ProgCtx) calculate() {
	screen_width := rl.GetScreenWidth()
	screen_height := rl.GetScreenHeight()

	ctx.main_menu.cont_rec = w.GetVerticalMenuRect(
		0,
		0,
		screen_width,
		screen_height,
	)

	ctx.server_menu.cont_rec = w.GetHorizontalMenuRect(
		ctx.main_menu.cont_rec.X,
		ctx.main_menu.cont_rec.Y,
		ctx.main_menu.cont_rec.Width,
		ctx.main_menu.cont_rec.Height,
	)

	ctx.conn_menu.cont_rec = w.GetHorizontalMenuRect(
		ctx.main_menu.cont_rec.X,
		ctx.main_menu.cont_rec.Y,
		ctx.main_menu.cont_rec.Width,
		ctx.main_menu.cont_rec.Height,
	)

	main_menu_buttons := w.GetMenuButtonsVertical(ctx.main_menu.cont_rec, 2)
	ctx.main_menu.connect_box = main_menu_buttons[0]
	ctx.main_menu.close_box = main_menu_buttons[1]

	connect_menu_buttons := w.GetMenuButtonsHorizontal(ctx.server_menu.cont_rec, 3)
	ctx.server_menu.ip_input_box = connect_menu_buttons[0]
	ctx.server_menu.port_input_box = connect_menu_buttons[1]
	ctx.server_menu.confirm_box = connect_menu_buttons[2]

	conn_menu_buttons := w.GetMenuButtonsVertical(ctx.conn_menu.cont_rec, 2)
	ctx.conn_menu.state_box = conn_menu_buttons[0]
	ctx.conn_menu.cancel_box = conn_menu_buttons[1]
}

func (ctx *ProgCtx) drawMainMenu() {
	rl.DrawRectangleRec(ctx.main_menu.cont_rec, ctx.main_menu.back_col)
	rl.DrawRectangleLinesEx(ctx.main_menu.cont_rec, 5, rl.White)
	rl.DrawRectangleLinesEx(ctx.main_menu.connect_box, 5, rl.White)
	rl.DrawRectangleLinesEx(ctx.main_menu.close_box, 5, rl.White)
}

func (ctx *ProgCtx) drawServerMenu() {
	rl.DrawRectangleRec(ctx.server_menu.cont_rec, ctx.server_menu.back_col)
	rl.DrawRectangleLinesEx(ctx.server_menu.cont_rec, 5, rl.White)
	rl.DrawRectangleLinesEx(ctx.server_menu.ip_input_box, 5, rl.White)
	rl.DrawRectangleLinesEx(ctx.server_menu.port_input_box, 5, rl.White)
	rl.DrawRectangleLinesEx(ctx.server_menu.confirm_box, 5, rl.White)
}

func (ctx *ProgCtx) drawConnectingMenu() {
	rl.DrawRectangleRec(ctx.conn_menu.cont_rec, ctx.conn_menu.back_col)
	rl.DrawRectangleLinesEx(ctx.conn_menu.cancel_box, 5, rl.White)
	rl.DrawRectangleLinesEx(ctx.conn_menu.state_box, 5, rl.White)
	rl.DrawText("Connecting...", int32(ctx.conn_menu.state_box.X), int32(ctx.conn_menu.state_box.Y), 20, rl.White)
}

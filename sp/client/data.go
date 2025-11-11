package main

import (
	rl "github.com/gen2brain/raylib-go/raylib"
	unet "poker-client/ups_net"
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
	GameConnect
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
	player         Player
	done_chan      chan bool
}

type NetworkCtx struct {
	host    string
	port    string
	state   NetworkState
	handler unet.NetHandler
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

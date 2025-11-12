package main

import (
	rl "github.com/gen2brain/raylib-go/raylib"
	unet "poker-client/ups_net"
)

type State int
type NetworkState int
type PlayerState int

const (
	Main State = iota
	ServerSelect
	Connecting
	AskingForRooms
	RoomSelect
	RoomQueue
	Game
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
	trigger        bool
	state          State
	last_state     State
	main_menu      M_Main
	server_menu    M_Server
	conn_menu      M_Connecting
	network        NetworkCtx
	done_chan      chan bool
	game           GameCtx
}

type NetworkCtx struct {
	host    string
	port    string
	state   NetworkState
	handler unet.NetHandler
}

type Room struct {
	current_players int
	turn            int
}

type Player struct {
	nick      string
	tokens    uint64
	connected bool
	folded    bool

	// if 0, then it means he checked
	// has to be checked when betting round is being done
	bet     uint64
	waiting int // -1 -> not waiting, >= 0 waiting
}

// Cards are dealt -> Betting round
// 3 cards of river shown -> Betting round
// 1 more card of river shown -> Betting round
// 1 more card of river shown -> Betting round
// Done
type GameCtx struct {
	p1        Player
	p2        Player
	p3        Player
	p4        Player
	turn      int // total turn
	p_turn    int // player turn
	bet_round int
	rooms     []Room
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

package main

import (
	unet "poker-client/ups_net"
	w "poker-client/window"
	"sync"
)

type UIScreen int

const (
	ScreenMainMenu UIScreen = iota
	ScreenServerSelect
	ScreenConnecting
	ScreenWaitingForRooms
	ScreenRoomSelect
	ScreenInGame
)

type Room struct {
	ID             int
	Name           string
	CurrentPlayers int // small int (2B)
	MaxPlayers     int // small int (2B)
}

type GameState struct {
	Screen       UIScreen
	Rooms        map[int]Room
	IsConnecting bool

	ServerIP   string
	ServerPort string
}

type UserInputEvent any

type EvtConnect struct {
	Host string
	Port string
}

type EvtCancelConnect struct{}

type EvtRoomJoin struct {
	RoomID string
}

type EvtQuit struct{}

type EvtBackToMain struct{}

type UIElement struct {
	dirty     bool
	component w.RGComponent
}

type UIStore struct {
	MainMenu     UIElement
	ServerSelect UIElement
	Connecting   UIElement
	Game         UIElement
}

func (store *UIStore) SetDirty() {
	store.MainMenu.dirty = true
	store.ServerSelect.dirty = true
	store.Connecting.dirty = true
}

type ProgCtx struct {
	State      GameState
	StateMutex sync.RWMutex

	UserInputChan chan UserInputEvent // Render -> Game
	NetMsgInChan  chan unet.NetMsg    // Network -> Game
	NetMsgOutChan chan unet.NetMsg    // Game -> Network
	DoneChan      chan bool           // Game -> Main (to signal shutdown)

	NetHandler  unet.NetHandler
	ShouldClose bool

	UI    UIStore
	Popup PopupManager
}

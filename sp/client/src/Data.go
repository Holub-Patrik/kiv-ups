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
	ScreenReconnecting
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

type Card struct {
	ID     int
	Symbol string
	Hidden bool
}

type PlayerData struct {
	ChipCount    int
	RoundBet     int
	TotalBet     int
	Cards        []Card
	IsMyTurn     bool
	IsFolded     bool
	IsReady      bool
	ActionTaken  string
	ActionAmount int
}

type PokerTable struct {
	Players        map[string]PlayerData
	CommunityCards []Card
	Pot            int
	HighBet        int
	MyNickname     string
	RoundPhase     string // "PreFlop", "Flop", "Turn", "River"
}

type PlayerConfig struct {
	NickName      string
	StartingChips int
}

type GameState struct {
	Screen UIScreen
	Rooms  map[int]Room

	IsConnecting bool

	ServerIP    string
	ServerPort  string
	PlayerCfg   PlayerConfig
	Reconnected bool

	Table     PokerTable
	BetAmount string
	Showdown  bool
}

type UserInputEvent any

type EvtAcceptReconnect struct{}
type EvtDeclineReconnect struct{}
type EvtRefreshRooms struct{}
type EvtCancelConnect struct{}
type EvtQuit struct{}
type EvtBackToMain struct{}

type EvtGameAction struct {
	Action string // "FOLD", "CALL", "BETT", "CHCK", "RDY1"
	Amount string // For betting
}

type EvtConnect struct {
	Host string
	Port string
}

type EvtRoomJoin struct {
	RoomID string
}

type UIElement struct {
	dirty     bool
	component w.RGComponent
}

type UIStore struct {
	MainMenu     UIElement
	ServerSelect UIElement
	Connecting   UIElement
	Reconnecting UIElement
	RoomSelect   UIElement
	Game         UIElement
}

func (store *UIStore) SetDirty() {
	store.MainMenu.dirty = true
	store.ServerSelect.dirty = true
	store.Connecting.dirty = true
	store.Game.dirty = true
}

type ProgCtx struct {
	State      GameState
	StateMutex sync.RWMutex

	UserInputChan chan UserInputEvent // Render -> Game
	DoneChan      chan bool           // Game -> Main (to signal shutdown)

	NetHandler  unet.NetHandler
	EventChan   <-chan unet.NetEvent
	ShouldClose bool

	UI    UIStore
	Popup PopupManager
}

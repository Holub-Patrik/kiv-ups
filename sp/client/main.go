package main

import (
	"fmt"
	"io"
	// "io"
	"net"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	rl "github.com/gen2brain/raylib-go/raylib"
)

// Message types from server
const (
	MsgTypeServerMessage byte = 'S'
	MsgTypeGameState     byte = 'G'
)

// Connection states
const (
	ConnStateDisconnected = iota
	ConnStateConnecting
	ConnStateConnected
	ConnStateError
)

// ServerMessage represents a message from the server with synchronization state
type ServerMessage struct {
	mu      sync.RWMutex
	text    string
	version uint64 // Incremented on each update for optimistic UI updates
	dirty   atomic.Bool
}

func (sm *ServerMessage) Set(text string) {
	sm.mu.Lock()
	sm.text = text
	sm.version++
	sm.mu.Unlock()
	sm.dirty.Store(true)
}

func (sm *ServerMessage) Get() (string, uint64) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.text, sm.version
}

func (sm *ServerMessage) IsDirty() bool {
	return sm.dirty.Load()
}

func (sm *ServerMessage) ClearDirty() {
	sm.dirty.Store(false)
}

// ConnectionState manages connection state with atomic operations
type ConnectionState struct {
	state atomic.Int32
	err   atomic.Value // stores error string
}

func (cs *ConnectionState) Set(state int32) {
	cs.state.Store(state)
}

func (cs *ConnectionState) Get() int32 {
	return cs.state.Load()
}

func (cs *ConnectionState) SetError(err string) {
	cs.err.Store(err)
	cs.Set(ConnStateError)
}

func (cs *ConnectionState) GetError() string {
	if v := cs.err.Load(); v != nil {
		return v.(string)
	}
	return ""
}

// NetworkClient handles the server connection and message processing
type NetworkClient struct {
	conn          net.Conn
	connState     ConnectionState
	serverMsg     ServerMessage
	reconnectChan chan struct{}
	shutdownChan  chan struct{}
	wg            sync.WaitGroup
}

func NewNetworkClient() *NetworkClient {
	nc := &NetworkClient{
		reconnectChan: make(chan struct{}, 1),
		shutdownChan:  make(chan struct{}),
	}
	nc.connState.Set(ConnStateDisconnected)
	nc.serverMsg.Set("No message from server")
	return nc
}

func (nc *NetworkClient) Connect(addr string) {
	nc.wg.Add(1)
	go nc.connectionManager(addr)
}

func (nc *NetworkClient) connectionManager(addr string) {
	defer nc.wg.Done()

	for {
		select {
		case <-nc.shutdownChan:
			return
		case <-nc.reconnectChan:
			// Drain multiple reconnect signals
			for len(nc.reconnectChan) > 0 {
				<-nc.reconnectChan
			}
		default:
		}

		nc.connState.Set(ConnStateConnecting)
		conn, err := net.DialTimeout("tcp", addr, 5*time.Second)
		if err != nil {
			nc.connState.SetError(fmt.Sprintf("Connection failed: %v", err))
			time.Sleep(2 * time.Second)
			continue
		}

		nc.conn = conn
		nc.connState.Set(ConnStateConnected)
		nc.serverMsg.Set("Connected to server")

		// Handle this connection until it fails
		nc.handleConnection()

		// Connection lost
		nc.conn = nil
		nc.connState.Set(ConnStateDisconnected)
		nc.serverMsg.Set("Disconnected from server")
		time.Sleep(2 * time.Second)
	}
}

func (nc *NetworkClient) handleConnection() {
	buf := make([]byte, 4096)
	wroteHello := false
	currectIndex := 0

	// no_response_attempts := 0

	for {
		select {
		case <-nc.shutdownChan:
			fmt.Println("NC Shutdownn")
			return
		default:
		}

		if !wroteHello {
			fmt.Println("Wrote hello to server")
			nc.conn.Write([]byte("Hello From Client"))
			wroteHello = true
		}

		// Set read deadline to allow periodic shutdown checks
		nc.conn.SetReadDeadline(time.Now().Add(100 * time.Millisecond))

		n, err := nc.conn.Read(buf[currectIndex:])
		if err != nil && err != io.EOF {
			nc.connState.SetError(fmt.Sprintf("Read error: %v", err))
			return
		}
		currectIndex += n

		/* if err == io.EOF {
			no_response_attempts += 1
			if no_response_attempts > 200 {
				return
			}
		} */

		if n < 5 { // Minimum message: type(1) + length(4)
			// fmt.Println("Msg too short, continuing")
			continue
		}

		nc.processMessage(buf[:n])
	}
}

func (nc *NetworkClient) processMessage(data []byte) {
	msgType := data[0]
	msgLenStr := string(data[1:5])
	msgLen, err := strconv.ParseUint(msgLenStr, 10, 64)

	if err != nil {
		fmt.Printf("Couldn't convert string to int: %s\n", msgLenStr)
		return
	}

	if len(data) < int(5+msgLen) {
		fmt.Printf("Message wasn't long enough: dataLen: %d | msgLen: %d\n", len(data), msgLen)
		return
	}

	payload := data[5 : 5+msgLen]

	switch msgType {
	case MsgTypeServerMessage:
		nc.serverMsg.Set(string(payload))
	case MsgTypeGameState:
		// Parse game state here
	}
}

func (nc *NetworkClient) RequestReconnect() {
	select {
	case nc.reconnectChan <- struct{}{}:
	default:
	}
}

func (nc *NetworkClient) Shutdown() {
	close(nc.shutdownChan)
	if nc.conn != nil {
		nc.conn.Close()
	}
	nc.wg.Wait()
}

// GUI state tracking
type GUIState struct {
	lastMsgVersion uint64
	msgText        string
}

func main() {
	const (
		screenWidth  = 800
		screenHeight = 600
	)

	client := NewNetworkClient()
	client.Connect("localhost:8080")
	defer client.Shutdown()

	rl.InitWindow(screenWidth, screenHeight, "Poker Client")
	defer rl.CloseWindow()

	rl.SetTargetFPS(60)

	guiState := GUIState{}

	button1Rect := rl.NewRectangle(50, 50, 200, 50)
	button2Rect := rl.NewRectangle(50, 120, 200, 50)
	messageRect := rl.NewRectangle(50, 200, 700, 100)

	for !rl.WindowShouldClose() {
		// Update: Check for dirty data and update GUI state
		if client.serverMsg.IsDirty() {
			text, version := client.serverMsg.Get()
			if version > guiState.lastMsgVersion {
				guiState.msgText = text
				guiState.lastMsgVersion = version
				client.serverMsg.ClearDirty()
			}
		}

		// Input handling
		mousePos := rl.GetMousePosition()
		if rl.IsMouseButtonPressed(rl.MouseLeftButton) {
			if rl.CheckCollisionPointRec(mousePos, button1Rect) {
				client.RequestReconnect()
			}
			if rl.CheckCollisionPointRec(mousePos, button2Rect) {
				// Send message to server (not implemented)
			}
		}

		// Draw
		rl.BeginDrawing()
		rl.ClearBackground(rl.RayWhite)

		// Button 1: Reconnect
		btn1Color := rl.LightGray
		if rl.CheckCollisionPointRec(mousePos, button1Rect) {
			btn1Color = rl.Gray
		}

		rl.DrawRectangleRec(button1Rect, btn1Color)
		rl.DrawRectangleLinesEx(button1Rect, 2, rl.DarkGray)
		rl.DrawText("RECONNECT", 70, 65, 20, rl.Black)

		// Button 2: Placeholder
		btn2Color := rl.LightGray
		if rl.CheckCollisionPointRec(mousePos, button2Rect) {
			btn2Color = rl.Gray
		}
		rl.DrawRectangleRec(button2Rect, btn2Color)
		rl.DrawRectangleLinesEx(button2Rect, 2, rl.DarkGray)
		rl.DrawText("ACTION", 90, 135, 20, rl.Black)

		// Message display
		rl.DrawRectangleLinesEx(messageRect, 2, rl.DarkGray)
		rl.DrawText("Server Message:", 60, 210, 20, rl.DarkGray)
		rl.DrawText(guiState.msgText, 60, 240, 18, rl.Black)

		// Connection state
		connState := client.connState.Get()
		stateText := ""
		stateColor := rl.Black
		switch connState {
		case ConnStateDisconnected:
			stateText = "Status: Disconnected"
			stateColor = rl.Red
		case ConnStateConnecting:
			stateText = "Status: Connecting..."
			stateColor = rl.Orange
		case ConnStateConnected:
			stateText = "Status: Connected"
			stateColor = rl.Green
		case ConnStateError:
			stateText = fmt.Sprintf("Status: Error - %s", client.connState.GetError())
			stateColor = rl.Red
		}
		rl.DrawText(stateText, 50, 320, 16, stateColor)

		rl.EndDrawing()
	}
}

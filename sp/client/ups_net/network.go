package ups_net

import (
	"fmt"
	"net"
	"sync"
	"sync/atomic"
	"time"
)

const (
	chanBufSize  = 100
	arrBufSize   = 256
	magicStr     = "PKR"
	reconnectMax = 30
	reconnectInt = 1 * time.Second
)

// Network events sent to game thread
type NetEvent any
type NetConnecting struct{}
type NetConnected struct{}
type NetReconnecting struct {
	Attempt int
	Max     int
}
type NetReconnected struct{}
type NetDisconnected struct{}
type NetMessage struct {
	Msg NetMsg
}

// Commands from game thread
type NetCommand any
type NetConnect struct {
	Host string
	Port string
}
type NetDisconnect struct{}
type NetShutdown struct{}

type NetHandler struct {
	conn           net.Conn
	connMtx        sync.RWMutex
	userDisconnect atomic.Bool
	state          atomic.Value
	shutdown       atomic.Bool

	eventChan   chan NetEvent   // Network -> Game (events)
	commandChan chan NetCommand // Game -> Network (commands)
	msgOutChan  chan NetMsg     // Game -> Network (outgoing messages)

	aliveTimer    *time.Ticker
	aliveMissed   int
	aliveReceived bool

	reconnectData struct {
		sync.Mutex
		host     string
		port     string
		attempts int
		active   bool
		timer    *time.Timer
	}
}

type ConnectionState int

const (
	StateDisconnected ConnectionState = iota
	StateConnecting
	StateConnected
	StateReconnecting
)

func (nh *NetHandler) Init() {
	nh.eventChan = make(chan NetEvent, chanBufSize)
	nh.commandChan = make(chan NetCommand, chanBufSize)
	nh.msgOutChan = make(chan NetMsg, chanBufSize)
	nh.state.Store(StateDisconnected)
	nh.shutdown.Store(false)
	nh.aliveTimer = time.NewTicker(time.Second * 10)

	nh.aliveMissed = 0
}

// Run is the main network thread
func (nh *NetHandler) Run() {
	fmt.Println("Network thread starting")

	for !nh.shutdown.Load() {
		select {
		case cmd := <-nh.commandChan:
			nh.handleCommand(cmd)

		case msg := <-nh.msgOutChan:
			nh.sendMessage(msg)

		case <-nh.aliveTimer.C:
			if nh.state.Load() == StateConnected {
				nh.checkAlive()
				nh.clearAlive()
				nh.sendAlive()
			}

		default:
			time.Sleep(10 * time.Millisecond)
		}
	}

	fmt.Println("Network thread shutting down")
	nh.cleanup()
}

func (nh *NetHandler) checkAlive() {
	if !nh.aliveReceived {
		nh.aliveMissed += 1
	}

	if nh.aliveMissed > 3 {
		nh.handleConnectionLost()
	}
}

func (nh *NetHandler) clearAlive() {
	nh.aliveReceived = false
}

func (nh *NetHandler) sendAlive() {
	nh.sendMessage(NetMsg{Code: "ALV?"})
}

func (nh *NetHandler) handleCommand(cmd NetCommand) {
	switch c := cmd.(type) {
	case NetConnect:
		nh.userDisconnect.Store(false)
		nh.handleConnect(c.Host, c.Port)

	case NetDisconnect:
		nh.userDisconnect.Store(true)
		nh.handleDisconnect()

	case NetShutdown:
		nh.shutdown.Store(true)

	default:
		fmt.Printf("Unknown command: %T\n", c)
	}
}

func (nh *NetHandler) handleConnect(host, port string) {
	if nh.getState() != StateDisconnected {
		fmt.Println("Already connected or connecting")
		return
	}

	if nh.userDisconnect.Load() {
		nh.state.Store(StateDisconnected)
		return
	}

	nh.setState(StateConnecting)
	nh.eventChan <- NetConnecting{}

	nh.reconnectData.Lock()
	nh.reconnectData.host = host
	nh.reconnectData.port = port
	nh.reconnectData.attempts = 0
	nh.reconnectData.active = false
	nh.reconnectData.Unlock()

	nh.connectWithRetry(host, port, false)
}

func (nh *NetHandler) handleDisconnect() {
	nh.reconnectData.Lock()
	nh.reconnectData.active = false
	if nh.reconnectData.timer != nil {
		nh.reconnectData.timer.Stop()
	}
	nh.reconnectData.Unlock()

	nh.disconnectInternal()
	nh.setState(StateDisconnected)
	nh.eventChan <- NetDisconnected{}
}

func (nh *NetHandler) disconnectInternal() {
	nh.connMtx.Lock()
	defer nh.connMtx.Unlock()

	if nh.conn != nil {
		nh.conn.Close()
		nh.conn = nil
	}
}

func (nh *NetHandler) connectWithRetry(host, port string, isReconnect bool) {
	if isReconnect {
		nh.setState(StateReconnecting)
	}

	if nh.userDisconnect.Load() {
		nh.state.Store(StateDisconnected)
		return
	}

	for {
		if nh.shutdown.Load() {
			return
		}

		currentState := nh.getState()
		if currentState == StateDisconnected && !isReconnect {
			return
		}

		if isReconnect {
			nh.reconnectData.Lock()
			if !nh.reconnectData.active {
				nh.reconnectData.Unlock()
				return
			}
			attempt := nh.reconnectData.attempts + 1
			nh.reconnectData.attempts = attempt
			nh.reconnectData.Unlock()

			if attempt > reconnectMax {
				fmt.Println("Reconnection attempts exhausted")
				nh.disconnectInternal()
				nh.setState(StateDisconnected)
				nh.eventChan <- NetDisconnected{}
				return
			}

			nh.eventChan <- NetReconnecting{
				Attempt: attempt,
				Max:     reconnectMax,
			}
		}

		conn, err := net.Dial("tcp", host+":"+port)
		if err == nil {
			nh.setConnection(conn)
			nh.setState(StateConnected)

			if isReconnect {
				nh.eventChan <- NetReconnected{}
				nh.reconnectData.Lock()
				nh.reconnectData.active = false
				nh.reconnectData.attempts = 0
				if nh.reconnectData.timer != nil {
					nh.reconnectData.timer.Stop()
				}
				nh.reconnectData.Unlock()
			} else {
				nh.eventChan <- NetConnected{}
			}

			nh.aliveMissed = 0
			nh.aliveReceived = false
			nh.aliveTimer.Reset(time.Second * 10)

			go nh.readerLoop(conn)
			return
		}

		fmt.Printf("Connection failed: %v\n", err)

		if !isReconnect {
			nh.disconnectInternal()
			nh.setState(StateDisconnected)
			nh.eventChan <- NetDisconnected{}
			return
		}

		select {
		case <-time.After(reconnectInt):
			continue
		case cmd := <-nh.commandChan:
			switch cmd.(type) {
			case NetDisconnect:
				nh.setState(StateDisconnected)
				nh.eventChan <- NetDisconnected{}
				return
			}
		}
	}
}

func (nh *NetHandler) readerLoop(conn net.Conn) {
	fmt.Println("Reader thread starting")
	defer fmt.Println("Reader thread exiting")

	buffer := [arrBufSize]byte{}
	parser := Parser{}
	parser.Init()

	for {
		if nh.shutdown.Load() {
			return
		}

		conn.SetReadDeadline(time.Now().Add(30 * time.Second))
		bytesRead, err := conn.Read(buffer[:])

		if err != nil {
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				continue
			}

			fmt.Printf("Reader error: %v\n", err)
			nh.handleConnectionLost()
			return
		}

		fmt.Printf("Received %d bytes\n", bytesRead)
		nh.processBuffer(buffer[:bytesRead], &parser)
	}
}

func (nh *NetHandler) processBuffer(buffer []byte, parser *Parser) {
	var totalParsed uint64 = 0

	for totalParsed < uint64(len(buffer)) {
		results := parser.ParseBytes(buffer[totalParsed:])

		if results.Error {
			fmt.Println("Protocol error, disconnecting")
			nh.handleConnectionLost()
			return
		}

		if results.parser_done {
			fmt.Printf("Parsed message: %s\n", results.code)

			switch results.code {
			case "ALV!":
				nh.aliveReceived = true
			case "PING":
				nh.sendMessage(NetMsg{Code: "PING"})
			default:
				nh.eventChan <- NetMessage{
					Msg: NetMsg{
						Code:    results.code,
						Payload: results.payload,
					},
				}
			}

			totalParsed += results.BytesParsed
			parser.ResetParser()
			continue
		}

		totalParsed += results.BytesParsed
	}
}

func (nh *NetHandler) handleConnectionLost() {
	currentState := nh.getState()
	if currentState != StateConnected {
		return
	}

	if nh.userDisconnect.Load() {
		nh.state.Store(StateDisconnected)
		return
	}

	nh.disconnectInternal()
	nh.setState(StateReconnecting)

	nh.reconnectData.Lock()
	if nh.reconnectData.host != "" && nh.reconnectData.port != "" {
		nh.reconnectData.active = true
		nh.reconnectData.attempts = 0
		host := nh.reconnectData.host
		port := nh.reconnectData.port
		nh.reconnectData.Unlock()

		go nh.connectWithRetry(host, port, true)
		return
	}
	nh.reconnectData.Unlock()

	nh.setState(StateDisconnected)
	nh.eventChan <- NetDisconnected{}
}

func (nh *NetHandler) sendMessage(msg NetMsg) error {
	if nh.getState() != StateConnected {
		return fmt.Errorf("not connected")
	}

	nh.connMtx.RLock()
	conn := nh.conn
	nh.connMtx.RUnlock()

	if conn == nil {
		return fmt.Errorf("no connection")
	}

	conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
	defer conn.SetWriteDeadline(time.Time{}) // Reset deadline

	data := msg.ToString()
	_, err := conn.Write([]byte(data))

	if err != nil {
		if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
			fmt.Println("Write timeout - connection may be dead")
		}

		nh.handleConnectionLost()
		return err
	}

	return nil
}

func (nh *NetHandler) getState() ConnectionState {
	return nh.state.Load().(ConnectionState)
}

func (nh *NetHandler) setState(state ConnectionState) {
	nh.state.Store(state)
}

func (nh *NetHandler) setConnection(conn net.Conn) {
	nh.connMtx.Lock()
	nh.conn = conn
	nh.connMtx.Unlock()
}

func (nh *NetHandler) cleanup() {
	nh.disconnectInternal()
	close(nh.eventChan)
	close(nh.commandChan)
	close(nh.msgOutChan)
}

func (nh *NetHandler) EventChan() <-chan NetEvent {
	return nh.eventChan
}

func (nh *NetHandler) SendCommand(cmd NetCommand) {
	select {
	case nh.commandChan <- cmd:
	default:
		fmt.Println("Command channel full")
	}
}

func (nh *NetHandler) SendNetMsg(msg NetMsg) {
	select {
	case nh.msgOutChan <- msg:
	default:
		fmt.Println("Message channel full")
	}
}

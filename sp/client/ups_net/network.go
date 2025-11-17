package ups_net

import (
	"fmt"
	"net"
	"os"
	"strings"
	"sync"
	"time"
)

const chanBufSize = 100
const arrBufSize = 256
const magicStr = "PKR"

const (
	PAYLOAD_INDENTIFIER   byte = 'P'
	NOPAYLOAD_INDENTIFIER byte = 'N'
)

type NetMsg struct {
	Code    string
	Payload string
}

// opening/closing socket
// This has to be included within the Game Thread so it can ask for messages from the network thread
type NetHandler struct {
	conn   net.Conn
	msgIn  chan NetMsg
	msgOut chan NetMsg

	wg           sync.WaitGroup
	shutdownOnce sync.Once
}

// receiving messages from client
type MsgAcceptor struct {
	conn     net.Conn
	msg_chan chan NetMsg
	nh       *NetHandler
}

func NewNetHandler() *NetHandler {
	msg_in := make(chan NetMsg, chanBufSize)
	msg_out := make(chan NetMsg, chanBufSize)

	netHandler := NetHandler{
		msgIn:  msg_in,
		msgOut: msg_out,
	}

	return &netHandler
}

// Attempts to connect to the given IP
// If it for any reason fails, it returns false
// If it connects, it sets the networkHandler connection to the retrieved connection
func (nh *NetHandler) Connect(host string, port string) bool {
	fmt.Println("NetHandler: Attempting connect")
	maybe_conn, err := net.Dial("tcp", host+":"+port)

	if err != nil {
		fmt.Println("NetHandler: Connect Failed")
		return false
	}

	fmt.Println("NetHandler: Success")
	nh.conn = maybe_conn
	return true
}

func (nh *NetHandler) shutdownFunc() {
	fmt.Println("NetHandler: Shutting down...")

	close(nh.msgOut)

	if nh.conn != nil {
		nh.conn.Close()
	}
}

func (nh *NetHandler) Shutdown() {
	nh.shutdownOnce.Do(nh.shutdownFunc)
}

func (nh *NetHandler) MsgIn() chan NetMsg {
	return nh.msgIn
}

func (nh *NetHandler) MsgOut() chan NetMsg {
	return nh.msgOut
}

func (nh *NetHandler) Run() {
	nh.wg.Add(2)

	// startups the 2 actual compute threads
	go nh.sendMessages() // startup sending messages

	acceptor := MsgAcceptor{
		conn:     nh.conn,
		msg_chan: nh.msgIn,
	}
	go acceptor.AcceptMessages() // startup receiving messages

	nh.wg.Wait()

	close(nh.msgIn)
}

func (nh *NetHandler) sendMessages() {
	defer nh.wg.Done()
	fmt.Println("Sender Thread Starting ... ")

	msg_builder := strings.Builder{}
	for msg := range nh.msgOut {
		byte_msg := []byte(msg.ToStringWithBuilder(&msg_builder))
		_, err := nh.conn.Write(byte_msg)

		if err != nil {
			// write error, means socket was closed
			fmt.Println("Sender Thread: Write error:", err)
			nh.Shutdown()
			return
		}
	}

	fmt.Println("Sender Thread Ending")
}

// creates the string that can be transmitted with network.Write()
func (msg *NetMsg) ToString() string {
	builder := strings.Builder{}
	builder.WriteString(magicStr)

	payload_len := len(msg.Payload)
	if payload_len > 0 {
		builder.WriteByte('P')
	} else {
		builder.WriteByte('N')
	}

	builder.Write([]byte(msg.Code))
	if payload_len > 0 {
		len_str := fmt.Sprintf("%04d", payload_len)

		builder.Write([]byte(len_str))
		builder.Write([]byte(msg.Payload))
	}

	builder.WriteByte('\n')
	return builder.String()
}

// creates the string that can be transmitted with network.Write()
func (msg *NetMsg) ToStringWithBuilder(builder *strings.Builder) string {
	builder.WriteString(magicStr)

	payload_len := len(msg.Payload)
	if payload_len > 0 {
		builder.WriteByte(PAYLOAD_INDENTIFIER)
	} else {
		builder.WriteByte(NOPAYLOAD_INDENTIFIER)
	}

	builder.WriteString(msg.Code)
	if payload_len > 0 {
		len_str := fmt.Sprintf("%04d", payload_len)

		builder.WriteString(len_str)
		builder.WriteString(msg.Payload)
	}

	builder.WriteByte('\n')
	ret_str := builder.String()
	builder.Reset()

	return ret_str
}

func (self *MsgAcceptor) AcceptMessages() {
	fmt.Println("Accepter Thread Starting")
	buffer := [arrBufSize]byte{}

	parser := Parser{}
	parser.Init()

	for {
		// waits for 20 milliseconds if anything is received
		deadline := time.Now().Add(time.Millisecond * 20)
		self.conn.SetReadDeadline(deadline)
		bytes_read, err := self.conn.Read(buffer[:])

		if err != nil {
			if os.IsTimeout(err) {
				continue
			}

			fmt.Println("Accepter Thread: Read error:", err)
			self.nh.Shutdown()
			return
		}

		var total_parsed_bytes uint64 = 0
		results := ParseResults{}

		for {
			results = parser.ParseBytes(buffer[:bytes_read])

			if results.error_occured {
				fmt.Println("Accepter Thread: Client sent goobledegook")
				self.nh.Shutdown()
				return
			}

			if results.parser_done {
				fmt.Println("Parsed correct message, sending out")
				self.msg_chan <- NetMsg{Code: results.code, Payload: results.payload}
				fmt.Println("Sent out")
				parser.ResetParser()
			}

			total_parsed_bytes += results.bytes_parsed
			if total_parsed_bytes >= uint64(bytes_read) {
				break
			}
		}
	}
}

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

	wg sync.WaitGroup
}

// receiving messages from client
type MsgAcceptor struct {
	conn     net.Conn
	msg_chan chan NetMsg
	nh       *NetHandler
}

// Attempts to connect to the given IP
// If it for any reason fails, it returns false
// If it connects, it sets the networkHandler connection to the retrieved connection
func (nh *NetHandler) Connect(host string, port string) bool {
	if nh.conn != nil {
		return false
	}

	fmt.Println("NetHandler: Attempting connect")
	maybe_conn, err := net.Dial("tcp", host+":"+port)

	if err != nil {
		fmt.Println("NetHandler: Connect Failed ->", err)
		return false
	}

	fmt.Println("NetHandler: Success")
	nh.conn = maybe_conn
	nh.msgIn = make(chan NetMsg, chanBufSize)
	nh.msgOut = make(chan NetMsg, chanBufSize)

	go nh.sendMessages()
	acceptor := MsgAcceptor{
		conn:     nh.conn,
		msg_chan: nh.msgIn,
		nh:       nh,
	}
	go acceptor.AcceptMessages()
	nh.wg.Add(2)

	return true
}

func (nh *NetHandler) Disconnect() {
	fmt.Println("NetHandler: Disconnecting...")

	// this should stop sender
	if nh.msgOut != nil {
		close(nh.msgOut)
		nh.msgOut = nil
	}

	// this should stop receiver
	if nh.conn != nil {
		nh.conn.Close()
		nh.conn = nil
	}

	// if both threads are already dead, this should be noop
	nh.wg.Wait()

	if nh.msgIn != nil {
		close(nh.msgIn)
		nh.msgIn = nil
	}

	fmt.Println("NetHandler: Disconnected")
}

func (nh *NetHandler) MsgIn() chan NetMsg {
	return nh.msgIn
}

func (nh *NetHandler) MsgOut() chan NetMsg {
	return nh.msgOut
}

func (nh *NetHandler) sendMessages() {
	fmt.Println("Sender Thread Starting ... ")

	msg_builder := strings.Builder{}
	for msg := range nh.msgOut {
		fmt.Println("NetHandler: Sending ->", msg.Code, msg.Payload)
		byte_msg := []byte(msg.ToStringWithBuilder(&msg_builder))
		_, err := nh.conn.Write(byte_msg)

		if err != nil {
			// write error, means socket was closed
			fmt.Println("Sender Thread: Write error:", err)
			nh.wg.Done()
			nh.Disconnect()
			return
		}
	}

	fmt.Println("Sender Thread Ending")
	nh.wg.Done()
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
			self.nh.wg.Done()
			self.nh.Disconnect()
			return
		}

		var total_parsed_bytes uint64 = 0
		results := ParseResults{}

		for {
			results = parser.ParseBytes(buffer[total_parsed_bytes:bytes_read])

			if results.error_occured {
				fmt.Println("Accepter Thread: Client sent goobledegook")
				// even though the thread is still running, it will quickly close
				self.nh.wg.Done()
				self.nh.Disconnect()
				return
			}

			if results.parser_done {
				code_msg := ""
				if results.payload != "" {
					code_msg = "Payload: " + results.payload
				}
				fmt.Println("Parsed correct message <- Code:", results.code, code_msg)
				self.msg_chan <- NetMsg{Code: results.code, Payload: results.payload}
				total_parsed_bytes += results.bytes_parsed
				parser.ResetParser()
			}

			total_parsed_bytes += results.bytes_parsed
			if total_parsed_bytes >= uint64(bytes_read) {
				break
			}
		}
	}
}

package ups_net

import (
	"fmt"
	"net"
	"strings"
)

const bufferSize = 100

type NetMsg struct {
	code    string
	payload string
}

// opening/closing socket
type NetHandler struct {
	connection  net.Conn
	msg_in      chan NetMsg
	msg_out     chan NetMsg
	in_shutdown chan struct{}
}

// receiving messages from client
type MsgAcceptor struct {
	connection   net.Conn
	msg_chan     chan NetMsg
	msg_shutdown chan struct{}
}

func InitNetHandler() NetHandler {
	msg_in := make(chan NetMsg, 100)
	msg_out := make(chan NetMsg, 100)
	shutdown_in := make(chan struct{})

	netHandler := NetHandler{
		msg_in:      msg_in,
		msg_out:     msg_out,
		in_shutdown: shutdown_in,
	}

	return netHandler
}

// Attempts to connect to the given IP
// If it for any reason fails, it returns false
// If it connects, it sets the networkHandler connection to the retrieved connection
func (nh *NetHandler) Connect(host string, port string) bool {
	maybe_conn, err := net.Dial("tcp", host+":"+port)
	if err != nil {
		return false
	}

	nh.connection = maybe_conn
	return true
}

func (nh *NetHandler) Run() {

}

func (nh *NetHandler) SendMessages() {
	msg_builder := strings.Builder{}
	for msg := range nh.msg_out {
		msg_builder.Write([]byte("PKR"))
		payload_len := len(msg.payload)
		if payload_len > 0 {
			msg_builder.WriteByte('P')
		} else {
			msg_builder.WriteByte('N')
		}
		msg_builder.Write([]byte(msg.code))
		if payload_len > 0 {
			len_str := fmt.Sprintf("%04d", payload_len)

			msg_builder.Write([]byte(len_str))
			msg_builder.Write([]byte(msg.payload))
		}
		msg_builder.WriteByte('\n')

		nh.connection.Write([]byte(msg_builder.String()))
		msg_builder.Reset()
	}
	// this should happen when the msg_out is closed
}

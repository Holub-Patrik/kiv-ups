package main

import (
	"fmt"
	"net"
	"os"
)

const (
	SERVER_HOST = "147.228.67.113"
	SERVER_PORT = "4242"
	SERVER_TYPE = "tcp"
)

func main() {
	connection, err := net.Dial(SERVER_TYPE, SERVER_HOST+":"+SERVER_PORT)
	if err != nil {
		fmt.Println("Failed to dial Server: ", err.Error())
		os.Exit(1)
	}

	connection.Write([]byte("KIVUPSnick001412Holub Patrik"))
	// connection.Write([]byte("KIVUPSchat00090061234560"))
	connection.Write([]byte("KIVUPSdisc001412HolubPatrik"))

	connection.Close()
}

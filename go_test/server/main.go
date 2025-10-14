package main

import (
	"fmt"
	"net"
	"os"
)

const (
	SERVER_HOST = "localhost"
	SERVER_PORT = "10000"
	SERVER_TYPE = "tcp"
)

func main() {
	fmt.Println("Server starting ...")
	server, err := net.Listen(SERVER_TYPE, SERVER_HOST+":"+SERVER_PORT)
	if err != nil {
		fmt.Println("Server couldn't start (Listening):", err.Error())
		os.Exit(1)
	}

	defer server.Close()

	fmt.Println("Listening on: ", SERVER_HOST+":"+SERVER_PORT)
	fmt.Println("Waiting for client connection ...")

	for {
		connection, err := server.Accept()
		if err != nil {
			fmt.Println("Error accepting: ", err.Error())
			os.Exit(1)
		}
		fmt.Println("Got connection")

		go processClient(connection)
	}
}

func processClient(connection net.Conn) {
	buffer := make([]byte, 1024)
	mLen, err := connection.Read(buffer)
	if err != nil {
		fmt.Println("Error reading: ", err.Error())
	}

	fmt.Println("Received: ", string(buffer[:mLen]))
	_, err = connection.Write([]byte("Thanks! Got your message: " + string(buffer[:mLen])))
	connection.Close()
}

package main

import (
	"log"
	"net"
)

func agentToBroker(secretChecksum *[32]byte) {
	for {
		connBroker, err := net.Dial("tcp", addressBroker)
		if err != nil {
			connBroker.Close()
			log.Println("agent to broker:", err)
			return
		}
		connBroker.Write(secretChecksum[:])
		go func() {
			s, err := readChecksumFromSocket(connBroker)
			if err != nil {
				connBroker.Close()
				log.Println("agent to broker:", err)
				return
			}
			if s == string(secretChecksum[:]) {
				agentToServer()
			} else {
				connBroker.Close()
				log.Println("possible attack detected")
			}
		}()
		connPool <- connBroker
	}
}

func agentToServer() {
	connServer, err := net.Dial("tcp", addressServer)
	if err != nil {
		log.Println("agent to server:", err)
		connServer.Close()
		return
	}
	connBroker := <-connPool
	go pipe(connBroker, connServer, BUFFER_SIZE)
	go pipe(connServer, connBroker, BUFFER_SIZE)
}

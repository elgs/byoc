package main

import (
	"log"
	"net"
)

func agentToBroker() {
	for {
		connBroker, err := net.Dial("tcp", addressBrokerForAgents)
		if err != nil {
			log.Println("agent to broker:", err)
			connBroker.Close()
			return
		}
		go func() {
			s := make([]byte, 5)
			connBroker.Read(s)
			if string(s) == "hello" {
				agentToServer()
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

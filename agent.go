package main

import (
	"fmt"
	"log"
	"net"
)

func agentToBroker(secretChecksum *[32]byte) {
	if *agentTargetAddress == "" {
		log.Println("agent target address is required")
		log.Println("use -target-address flag")
		return
	}

	for {
		connBroker, err := net.Dial("tcp", fmt.Sprintf("%s:%s", *agentBrokerHost, *agentBrokerPort))
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
				agentToTarget()
			} else {
				connBroker.Close()
				log.Println("possible attack detected")
			}
		}()
		connPool <- connBroker
	}
}

func agentToTarget() {
	connServer, err := net.Dial("tcp", *agentTargetAddress)
	if err != nil {
		log.Println("agent to server:", err)
		connServer.Close()
		return
	}
	connBroker := <-connPool
	go pipe(connBroker, connServer, BUFFER_SIZE)
	go pipe(connServer, connBroker, BUFFER_SIZE)
}

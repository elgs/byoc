package main

import (
	"encoding/binary"
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
		connBroker, err := net.Dial("tcp", fmt.Sprintf("%s:%d", *agentBrokerHost, *agentBrokerPort))
		if err != nil {
			connBroker.Close()
			log.Println("agent to broker:", err)
			return
		}
		connBroker.Write(secretChecksum[:])
		binary.Write(connBroker, binary.LittleEndian, *agentPublicPort)
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
		if connPool[*agentPublicPort] == nil {
			connPool[*agentPublicPort] = make(chan net.Conn, CONN_POOL_SIZE)
		}
		connPool[*agentPublicPort] <- connBroker
	}
}

func agentToTarget() {
	connServer, err := net.Dial("tcp", *agentTargetAddress)
	if err != nil {
		log.Println("agent to server:", err)
		connServer.Close()
		return
	}
	if connPool[*agentPublicPort] == nil {
		connPool[*agentPublicPort] = make(chan net.Conn, CONN_POOL_SIZE)
	}
	connBroker := <-connPool[*agentPublicPort]
	go pipe(connBroker, connServer, BUFFER_SIZE)
	go pipe(connServer, connBroker, BUFFER_SIZE)
}

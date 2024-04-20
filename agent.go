package main

import (
	"encoding/binary"
	"fmt"
	"log"
	"net"
	"time"
)

func agentToBroker(secretChecksum *[32]byte) {
	if *agentTargetAddress == "" {
		log.Println("agent target address is required")
		log.Println("use -target-address flag")
		return
	}

	for {
		log.Println("agent state", agentState)
		if agentState == 0 {
			agentState = 1
		} else if agentState == 1 {
			log.Println("agent is being initialized")
			time.Sleep(1 * time.Second)
			continue
		}

		connBroker, err := net.Dial("tcp", fmt.Sprintf("%s:%d", *agentBrokerHost, *agentBrokerPort))
		if err != nil {
			connBroker.Close()
			log.Println("agent to broker:", err)
			return
		}
		// 1. agent sends secret checksum to broker - 32 bytes
		connBroker.Write(secretChecksum[:])
		// 2. broker answers true or false
		var checksumResult bool
		binary.Read(connBroker, binary.LittleEndian, &checksumResult)
		if !checksumResult {
			connBroker.Close()
			log.Println("failed to authenticate with broker")
			return
		}

		// 3. agent sends suggested public port to broker - 8 bytes
		binary.Write(connBroker, binary.LittleEndian, *agentPublicPort)
		// 4. broker answers actual public port - 8 bytes
		binary.Read(connBroker, binary.LittleEndian, agentPublicPort)
		go func() {
			// 5. agent receives secret checksum from broker to trigger agent to connect to target - 32 bytes
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
		if agentConnPool[*agentPublicPort] == nil {
			agentConnPool[*agentPublicPort] = make(chan net.Conn, CONN_POOL_SIZE)
		}
		agentConnPool[*agentPublicPort] <- connBroker
		agentState = 2
	}
}

func agentToTarget() {
	connServer, err := net.Dial("tcp", *agentTargetAddress)
	if err != nil {
		log.Println("agent to server:", err)
		connServer.Close()
		return
	}
	if agentConnPool[*agentPublicPort] == nil {
		agentConnPool[*agentPublicPort] = make(chan net.Conn, CONN_POOL_SIZE)
	}
	connBroker := <-agentConnPool[*agentPublicPort]
	go pipe(connBroker, connServer, BUFFER_SIZE)
	go pipe(connServer, connBroker, BUFFER_SIZE)
}

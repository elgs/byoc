package main

import (
	"bytes"
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
			log.Println("agent to broker:", err)
			continue
		}
		// 1. agent sends secret checksum to broker - 32 bytes
		binary.Write(connBroker, binary.LittleEndian, secretChecksum)
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

		// 5. broker sends lenth of broker public host - 8 bytes
		var publicHostLen uint64
		binary.Read(connBroker, binary.LittleEndian, &publicHostLen)
		// 6. broker sends broker public host
		bs := make([]byte, publicHostLen)
		binary.Read(connBroker, binary.LittleEndian, bs)
		*brokerPublicHost = string(bs)
		fmt.Println("public host:", *brokerPublicHost, "public port:", *agentPublicPort)

		go func() {
			// 50. agent receives secret checksum from broker to trigger agent to connect to target - 32 bytes
			bs, err := readChecksumFromSocket(connBroker)
			if err != nil {
				<-agentConnPool[*agentPublicPort]
				connBroker.Close()
				log.Println("agent to broker:", err)
				return
			}
			if bytes.Equal(bs[:], secretChecksum[:]) {
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
	}
}

func agentToTarget() {
	connServer, err := net.Dial("tcp", *agentTargetAddress)
	if err != nil {
		log.Println("agent to server:", err)
		return
	}
	if agentConnPool[*agentPublicPort] == nil {
		agentConnPool[*agentPublicPort] = make(chan net.Conn, CONN_POOL_SIZE)
	}
	connBroker := <-agentConnPool[*agentPublicPort]
	go pipe(connBroker, connServer, BUFFER_SIZE)
	go pipe(connServer, connBroker, BUFFER_SIZE)
}

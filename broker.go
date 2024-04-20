package main

import (
	"encoding/binary"
	"fmt"
	"log"
	"net"
	"time"
)

func brokerForPublic(secretChecksum *[32]byte, publicPort *uint64) {
	listener, err := net.Listen("tcp", fmt.Sprintf("%s:%d", *brokerPublicHost, *publicPort))
	if err != nil {
		log.Println(err, *publicPort, "retrying in 5 seconds")
		time.Sleep(5 * time.Second)
		// generate a random port between 1024 and 65535
		*publicPort = 1024 + uint64(time.Now().UnixNano())%64511
		brokerForPublic(secretChecksum, publicPort)
		return
	}
	go func() {
		for {
			connClient, err := listener.Accept()
			if err != nil {
				log.Println("broker for clients:", err)
				continue
			}
			log.Println("client connected from ", connClient.RemoteAddr())
			go func() {
				cpKey := fmt.Sprintf("%x%d", secretChecksum, *publicPort)
				select {
				case connAgent := <-brokerConnPool[cpKey]:
					// 5. broker sends secret checksum to agent to trigger agent to connect to target - 32 bytes
					connAgent.Write(secretChecksum[:])
					go pipe(connClient, connAgent, BUFFER_SIZE)
					go pipe(connAgent, connClient, BUFFER_SIZE)
				case <-time.After(5 * time.Second):
					connClient.Close()
					log.Println("no agent available")
				}
			}()
		}
	}()
}

func brokerForAgents(secretChecksum *[32]byte) {
	listener, err := net.Listen("tcp", fmt.Sprintf("%s:%d", *brokerAgentHost, *brokerAgentPort))
	if err != nil {
		log.Fatalln(err)
	}

	for {
		connAgent, err := listener.Accept()
		if err != nil {
			log.Println("broker for agents:", err)
			continue
		}
		go func() {
			// 1. broker receives secret checksum from agent - 32 bytes
			s, err := readChecksumFromSocket(connAgent)
			if err != nil {
				connAgent.Close()
				log.Println("broker for agents:", err)
				return
			}
			if s != string(secretChecksum[:]) {
				// 2. broker answers true or false
				binary.Write(connAgent, binary.LittleEndian, false)
				connAgent.Close()
				log.Println("possible attack detected")
				return
			}
			// 2. broker answers true or false
			binary.Write(connAgent, binary.LittleEndian, true)

			// 3. broker receives suggested public port from agent - 8 bytes
			var publicPort uint64
			err = binary.Read(connAgent, binary.LittleEndian, &publicPort)
			if err != nil {
				connAgent.Close()
				log.Println("broker for agents:", err)
				return
			}
			log.Println("agent connected from ", connAgent.RemoteAddr())

			cpKey := fmt.Sprintf("%x%d", secretChecksum, publicPort)
			if brokerConnPool[cpKey] == nil {
				brokerConnPool[cpKey] = make(chan net.Conn, CONN_POOL_SIZE)
				brokerForPublic(secretChecksum, &publicPort)
			}
			// 4. broker answers actual public port - 8 bytes
			binary.Write(connAgent, binary.LittleEndian, publicPort)
			brokerConnPool[cpKey] <- connAgent
		}()
	}
}

/**
  1. agent sends secret checksum to broker - 32 bytes
	2. broker answers true or false
	3. agent sends suggested public port to broker - 8 bytes
	4. broker answers actual public port - 8 bytes
	5. broker sends secret checksum to agent to trigger agent to connect to target - 32 bytes
*/

package main

import (
	"encoding/binary"
	"fmt"
	"log"
	"net"
)

func brokerForPublic(secretChecksum *[32]byte, publicPort uint64) {
	listener, err := net.Listen("tcp", fmt.Sprintf("%s:%d", *brokerPublicHost, publicPort))
	if err != nil {
		log.Println(err)
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
				if connPool[publicPort] == nil {
					connPool[publicPort] = make(chan net.Conn, CONN_POOL_SIZE)
				}
				connAgent := <-connPool[publicPort]
				connAgent.Write(secretChecksum[:])
				go pipe(connClient, connAgent, BUFFER_SIZE)
				go pipe(connAgent, connClient, BUFFER_SIZE)
			}()
		}
	}()
}

func brokerForAgents(secretChecksum *[32]byte) {
	listener, err := net.Listen("tcp", fmt.Sprintf("%s:%d", *brokerAgentHost, *brokerAgentPort))
	if err != nil {
		log.Println(err)
	}
	go func() {
		for {
			connAgent, err := listener.Accept()
			if err != nil {
				log.Println("broker for agents:", err)
				continue
			}
			s, err := readChecksumFromSocket(connAgent)
			if err != nil {
				connAgent.Close()
				log.Println("broker for agents:", err)
				continue
			}
			if s != string(secretChecksum[:]) {
				connAgent.Close()
				log.Println("possible attack detected")
				continue
			}
			var publicPort uint64
			err = binary.Read(connAgent, binary.LittleEndian, &publicPort)
			if err != nil {
				connAgent.Close()
				log.Println("broker for agents:", err)
				continue
			}
			log.Println("agent connected from ", connAgent.RemoteAddr())
			if connPool[publicPort] == nil {
				connPool[publicPort] = make(chan net.Conn, CONN_POOL_SIZE)
				go brokerForPublic(secretChecksum, publicPort)
			}
			connPool[publicPort] <- connAgent
		}
	}()
}

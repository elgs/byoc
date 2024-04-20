package main

import (
	"fmt"
	"log"
	"net"
)

func brokerForClients(secretChecksum *[32]byte) {
	listener, err := net.Listen("tcp", fmt.Sprintf("%s:%d", brokerPublicHost, 8080))
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
				connAgent := <-connPool
				connAgent.Write(secretChecksum[:])
				go pipe(connClient, connAgent, BUFFER_SIZE)
				go pipe(connAgent, connClient, BUFFER_SIZE)
			}()
		}
	}()
}

func brokerForAgents(secretChecksum *[32]byte) {
	listener, err := net.Listen("tcp", fmt.Sprintf("%s:%s", brokerAgentHost, brokerAgentPort))
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
			log.Println("agent connected from ", connAgent.RemoteAddr())
			connPool <- connAgent
		}
	}()
}

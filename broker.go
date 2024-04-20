package main

import (
	"log"
	"net"
)

func brokerForClients() {
	listener, err := net.Listen("tcp", addressBrokerForClients)
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
			go func() {
				connAgent := <-connPool
				connAgent.Write([]byte("hello"))
				go pipe(connClient, connAgent, BUFFER_SIZE)
				go pipe(connAgent, connClient, BUFFER_SIZE)
			}()
		}
	}()
}

func brokerForAgents() {
	listener, err := net.Listen("tcp", addressBrokerForAgents)
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
			log.Println("agent connected from ", connAgent.RemoteAddr())
			connPool <- connAgent
		}
	}()
}

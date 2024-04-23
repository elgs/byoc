package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"log"
	"net"
	"time"
)

func brokerForPublic(secretChecksum *[32]byte, publicPort *int64) {
	listener, err := net.Listen("tcp", fmt.Sprintf("%s:%d", *brokerPublicHost, *publicPort))
	if err != nil {
		log.Println(err, *publicPort, "retrying in 1 seconds")
		time.Sleep(1 * time.Second)
		// generate a random port between 1024 and 65535
		*publicPort = getRandomPort()
		brokerForPublic(secretChecksum, publicPort)
		return
	}
	cpKey := fmt.Sprintf("%x|%d", *secretChecksum, *publicPort)
	mu.Lock()
	if brokerConnPool[cpKey] == nil {
		brokerConnPool[cpKey] = make(chan net.Conn, CONN_POOL_SIZE)
	}
	mu.Unlock()
	go func() {
		for {
			connClient, err := listener.Accept()
			if err != nil {
				log.Println("broker for clients:", err)
				break
			}
			log.Println("client connected from ", connClient.RemoteAddr())
			go func() {
				select {
				case connAgent := <-brokerConnPool[cpKey]:
					// 50. broker sends secret checksum to agent to trigger agent to connect to target - 32 bytes
					binary.Write(connAgent, binary.LittleEndian, secretChecksum)
					go pipe(connClient, connAgent, BUFFER_SIZE)
					go pipe(connAgent, connClient, BUFFER_SIZE)
				case <-time.After(5 * time.Second):
					connClient.Close()
					listener.Close()
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
			bs, err := readChecksumFromSocket(connAgent)
			if err != nil {
				connAgent.Close()
				log.Println("broker for agents:", err)
				return
			}
			if !bytes.Equal(bs[:], secretChecksum[:]) {
				// 2. broker answers true or false
				binary.Write(connAgent, binary.LittleEndian, false)
				connAgent.Close()
				log.Println("possible attack detected")
				return
			}
			// 2. broker answers true or false
			err = binary.Write(connAgent, binary.LittleEndian, true)
			if err != nil {
				connAgent.Close()
				log.Println("broker for agents:", err)
				return
			}

			// 3. broker receives suggested public port from agent - 8 bytes
			var publicPort int64
			err = binary.Read(connAgent, binary.LittleEndian, &publicPort)
			if err != nil {
				connAgent.Close()
				log.Println("broker for agents:", err)
				return
			}
			log.Println("agent connected from ", connAgent.RemoteAddr())

			if publicPort == 0 {
				// generate a random port between 1024 and 65535
				publicPort = getRandomPort()
			}

			if publicPort < 0 {
				publicPort = -publicPort
				brokerForPublic(&bs, &publicPort) //////////////////////////////////////////
				// 4. broker answers actual public port - 8 bytes
				err = binary.Write(connAgent, binary.LittleEndian, publicPort)
				if err != nil {
					connAgent.Close()
					log.Println("broker for agents:", err)
					return
				}
				// 5. broker sends lenth of broker public host - 8 bytes
				publicHostLen := uint64(len(*brokerPublicHost))
				err = binary.Write(connAgent, binary.LittleEndian, publicHostLen)
				if err != nil {
					connAgent.Close()
					log.Println("broker for agents:", err)
					return
				}
				// 6. broker sends broker public host
				err = binary.Write(connAgent, binary.LittleEndian, []byte(*brokerPublicHost))
				if err != nil {
					connAgent.Close()
					log.Println("broker for agents:", err)
					return
				}
			}
			cpKey := fmt.Sprintf("%x|%d", bs, publicPort)
			mu.Lock()
			channel := brokerConnPool[cpKey]
			mu.Unlock()
			channel <- connAgent
		}()
	}
}

/**
  1. agent sends secret checksum to broker - 32 bytes
	2. broker answers true or false
	3. agent sends suggested public port to broker, a negative number for suggestion, a positive number to skip to after 6. - 8 bytes
	4. broker answers actual public port - 8 bytes
	5. broker sends lenth of broker public host - 8 bytes
	6. broker sends broker public host
	50. broker sends secret checksum to agent to trigger agent to connect to target - 32 bytes
*/

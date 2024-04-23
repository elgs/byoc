package main

import (
	"crypto/sha256"
	"encoding/binary"
	"flag"
	"log"
	"net"
	"os"
	"os/signal"
	"runtime"
	"sync"
	"syscall"
	"time"
)

const BUFFER_SIZE = 4096
const CONN_POOL_SIZE = 2
const DEFAULT_AGENT_BROKER_PORT = 18080

var brokerPublicHost *string
var brokerAgentHost *string
var brokerAgentPort *uint64

var agentPublicPort *int64
var agentBrokerHost *string
var agentBrokerPort *uint64
var agentTargetAddress *string

var agentConnPool = make(map[int64]chan net.Conn)
var brokerConnPool = make(map[string]chan net.Conn) // key: secret checksum | public port, no space in between

var mu = sync.Mutex{}

var agentState = 0 // 0: not initialized, 1: initialized

func main() {
	startBroker := flag.Bool("broker", false, "start broker")
	brokerPublicHost = flag.String("public-host", "[::]", "broker's public host")
	brokerAgentHost = flag.String("agent-host", "[::]", "broker's agent host")
	brokerAgentPort = flag.Uint64("agent-port", DEFAULT_AGENT_BROKER_PORT, "broker's agent port")

	agentPublicPort = flag.Int64("public-port", 0, "agent's public port, default for a random port")
	agentBrokerHost = flag.String("broker-host", "localhost", "agent's broker host")
	agentBrokerPort = flag.Uint64("broker-port", DEFAULT_AGENT_BROKER_PORT, "agent's broker port")
	agentTargetAddress = flag.String("target-address", "", "agent's target address")

	secret := flag.String("secret", "", "secret")
	flag.Parse()

	secretChecksum := sha256.Sum256([]byte(*secret))
	if *startBroker {
		brokerForAgents(&secretChecksum)
	} else {
		agentToBroker(&secretChecksum)
	}
	Hook(nil)
}

func Hook(clean func()) {
	sigs := make(chan os.Signal, 1)
	done := make(chan bool, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigs
		if clean != nil {
			clean()
		}
		done <- true
	}()
	<-done
}

func pipe(connLocal net.Conn, connDst net.Conn, bufSize int) {
	var buffer = make([]byte, bufSize)
	for {
		runtime.Gosched()
		n, err := connLocal.Read(buffer)
		if err != nil {
			connLocal.Close()
			connDst.Close()
			// log.Println("pipe is closed:", err)
			break
		}
		if n > 0 {
			_, err := connDst.Write(buffer[0:n])
			if err != nil {
				connLocal.Close()
				connDst.Close()
				log.Println("io error:", err)
				break
			}
		}
	}
}

func readChecksumFromSocket(conn net.Conn) ([32]byte, error) {
	buf := [32]byte{}
	err := binary.Read(conn, binary.LittleEndian, &buf)
	return buf, err
}

func getRandomPort() int64 {
	return 1024 + time.Now().UnixNano()%64511
}

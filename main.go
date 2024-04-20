package main

import (
	"crypto/sha256"
	"flag"
	"log"
	"net"
	"os"
	"os/signal"
	"runtime"
	"syscall"
)

var brokerPublicHost *string
var brokerAgentHost *string
var brokerAgentPort *string

var agentBrokerHost *string
var agentBrokerPort *string
var agentTargetAddress *string

var connPool = make(chan net.Conn, 2)

const BUFFER_SIZE = 4096

func main() {
	startBroker := flag.Bool("broker", false, "start broker")
	brokerPublicHost = flag.String("public-host", "[::]", "broker's public host")
	brokerAgentHost = flag.String("agent-host", "[::]", "broker's agent host")
	brokerAgentPort = flag.String("agent-port", "18080", "broker's agent port")

	agentBrokerHost = flag.String("broker-host", "localhost", "agent's broker host")
	agentBrokerPort = flag.String("broker-port", "18080", "agent's broker port")
	agentTargetAddress = flag.String("target-address", "", "agent's target address")

	secret := flag.String("secret", "", "secret")
	flag.Parse()

	secretChecksum := sha256.Sum256([]byte(*secret))
	if *startBroker {
		brokerForAgents(&secretChecksum)
		brokerForClients(&secretChecksum)
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
			log.Println("pipe is closed:", err)
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

func readChecksumFromSocket(conn net.Conn) (string, error) {
	buf := make([]byte, 32)
	n, err := conn.Read(buf)
	if err != nil {
		return "", err
	}
	return string(buf[:n]), nil
}

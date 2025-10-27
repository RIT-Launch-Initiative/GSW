package main

import (
	"encoding/binary"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"net/http"
	_ "net/http/pprof"

	"github.com/AarC10/GSW-V2/lib/ipc"
	"github.com/AarC10/GSW-V2/lib/tlm"
	"github.com/AarC10/GSW-V2/proc"
)

const (
	PRINT_INTERVAL = 3
)

var TELEMETRY_PACKETS = []TelemetryPacket{
	{"TimestampsOnly", 10000, 16},
	{"17Bytes", 10001, 17},
	{"26Bytes", 10002, 26},
	{"30Bytes", 10003, 30},
	{"38Bytes", 10004, 38},
	{"OneKbyte", 10005, 1024},
}

func initProfiling(pprofPort int) {
	go func() {
		log.Printf("Running pprof server at localhost:%d", pprofPort)
		err := http.ListenAndServe(fmt.Sprintf("localhost:%d", pprofPort), nil)
		if err != nil {
			log.Fatalf("Error starting pprof server: %v", err)
		}
	}()
}

var packetsReceived = 0

func main() {
	_, err := proc.ParseConfig("data/test/benchmark.yaml")
	if err != nil {
		log.Fatal(err)
	}

	isReader := flag.Bool("reader", false, "run a gsw reader")
	isWriter := flag.Bool("writer", false, "run a gsw writer")
	profilePort := flag.Int("pprof", 0, "run pprof at a port")
	serverAddress := flag.String("writer_host", "localhost", "the gsw host that the writer will attempt to write to")

	flag.Parse()

	if !*isReader && !*isWriter {
		log.Fatal("use -reader and/or -writer to start the process as a reader or writer")
	}

	if *profilePort != 0 {
		initProfiling(*profilePort)
	}

	if *isReader {
		log.Println("running reader")
		go reader()
	}
	if *isWriter {
		log.Println("running writer")
		go writer(*serverAddress)
	}

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	<-sigChan
}

var averageDiff uint64
var averageUdpShmDiff uint64
var averageBenchShmDiff uint64

func packetReader(packet tlm.TelemetryPacket, rcvChan chan []byte) {
	for data := range rcvChan {
		timestamp := uint64(time.Now().UnixNano())
		header := (*ipc.ShmHeader)(unsafe.Pointer(&data[0]))
		udpTimestamp := binary.LittleEndian.Uint64(data[16:24])

		udpShmDiff := header.Timestamp - udpTimestamp
		benchShmDiff := timestamp - header.Timestamp
		totalDiff := timestamp - udpTimestamp
		if averageDiff == 0 {
			averageDiff = totalDiff
		}

		averageDiff = (averageDiff + totalDiff) / 2
		averageUdpShmDiff = (udpShmDiff + totalDiff) / 2
		averageBenchShmDiff = (benchShmDiff + totalDiff) / 2

		packetsReceived++
	}
}

func reader() {
	var lastPacketsReceived = 0
	go func() {
		for {
			time.Sleep(time.Second * PRINT_INTERVAL)
			var sb strings.Builder
			sb.WriteString(fmt.Sprintf("%d packets/second\n", (packetsReceived-lastPacketsReceived)/PRINT_INTERVAL))
			sb.WriteString(fmt.Sprintf("Average Diff: %d\n", averageDiff))
			sb.WriteString(fmt.Sprintf("Average UDP SHM Diff: %d\n", averageUdpShmDiff))
			sb.WriteString(fmt.Sprintf("Average Bench SHM Diff: %d\n", averageBenchShmDiff))

			fmt.Print(sb.String())

			lastPacketsReceived = packetsReceived
		}
	}()

	for _, packet := range proc.GswConfig.TelemetryPackets {
		outChan := make(chan []byte)
		go proc.TelemetryPacketReader(packet, outChan, "/dev/shm")
		go packetReader(packet, outChan)
	}
}

type TelemetryPacket struct {
	Name string
	Port int
	Size int
}

func createPacket(size int) []byte {
	timestamp := time.Now().UnixNano()

	packet := make([]byte, size)

	binary.LittleEndian.PutUint64(packet[0:8], uint64(timestamp))

	return packet
}

func packetWriter(serverAddress string, port, size int) error {
	serverAddr, err := net.ResolveUDPAddr("udp", fmt.Sprintf("%s:%d", serverAddress, port))
	if err != nil {
		return fmt.Errorf("resolving address: %w", err)
	}

	conn, err := net.DialUDP("udp", nil, serverAddr)
	if err != nil {
		return fmt.Errorf("dialing udp (%s): %w", serverAddr.String(), err)
	}
	defer conn.Close()

	for {
		packet := createPacket(size)
		_, err := conn.Write(packet)
		if err != nil {
			fmt.Printf("error writing packet (%s): %v\n", serverAddr.String(), err)
		}
	}
}

func writer(serverAddress string) {
	var wg sync.WaitGroup

	for _, packet := range TELEMETRY_PACKETS {
		wg.Add(1)
		go func(serverAddress string, port, size int) {
			defer wg.Done()
			packetWriter(serverAddress, port, size)
		}(serverAddress, packet.Port, packet.Size)
	}
	wg.Wait()
}

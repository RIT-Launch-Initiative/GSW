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
	"sync/atomic"
	"syscall"
	"time"

	"net/http"
	_ "net/http/pprof"

	"github.com/AarC10/GSW-V2/lib/ipc"
	"github.com/AarC10/GSW-V2/lib/tlm"
	"github.com/AarC10/GSW-V2/proc"
)

const (
	PRINT_INTERVAL = 3
)

func initProfiling(pprofPort int) {
	go func() {
		log.Printf("Running pprof server at localhost:%d", pprofPort)
		err := http.ListenAndServe(fmt.Sprintf("localhost:%d", pprofPort), nil)
		if err != nil {
			log.Fatalf("Error starting pprof server: %v", err)
		}
	}()
}

func main() {
	_, err := proc.ParseConfig("data/test/benchmark.yaml")
	if err != nil {
		log.Fatal(err)
	}

	isReader := flag.Bool("reader", false, "run a gsw reader")
	isWriter := flag.Bool("writer", false, "run a gsw writer")
	writerSleep := flag.Duration("writer_sleep", 0, "approximately how long the writer will sleep between packets")
	serverAddress := flag.String("writer_host", "localhost", "the gsw host that the writer will attempt to write to")
	profilePort := flag.Int("pprof", 0, "run pprof at a port")

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
		go writer(*serverAddress, *writerSleep)
	}

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	<-sigChan
}

var totalPacketsReceived atomic.Uint64

func packetReader(packet tlm.TelemetryPacket) {
	reader, err := proc.NewIpcShmReaderForPacket(packet, "/dev/shm")
	if err != nil {
		log.Fatal(fmt.Errorf("couldn't create reader for packet: %w", err))
	}

	defer reader.Cleanup()

	var totalPacketsReader uint64
	var packetsLost uint64

	var averageDiff uint64
	var averageUdpShmDiff uint64
	var averageBenchShmDiff uint64

	go func() {
		var lastPacketsReceived uint64
		var lastPacketsLost uint64
		for {
			time.Sleep(time.Second * PRINT_INTERVAL)
			var sb strings.Builder

			packetsInterval := totalPacketsReader - lastPacketsReceived
			packetsLostInterval := packetsLost - lastPacketsLost

			sb.WriteString(fmt.Sprintf("[%s] Packets/s: %d\n", packet.Name, packetsInterval/PRINT_INTERVAL))
			sb.WriteString(fmt.Sprintf("[%s] Reader lost/s: %d (%.3f%%)\n", packet.Name, packetsLostInterval/PRINT_INTERVAL, (float64(packetsLostInterval)/float64(packetsInterval+packetsLostInterval))*100))
			sb.WriteString(fmt.Sprintf("[%s] Average Diff: %d\n", packet.Name, averageDiff))
			sb.WriteString(fmt.Sprintf("[%s] Average UDP SHM Diff: %d\n", packet.Name, averageUdpShmDiff))
			sb.WriteString(fmt.Sprintf("[%s] Average Bench SHM Diff: %d\n", packet.Name, averageBenchShmDiff))

			lastPacketsReceived = totalPacketsReader
			lastPacketsLost = packetsLost

			fmt.Print(sb.String())
		}
	}()

	var lastPacketSequence uint64

	for {
		p, err := reader.Read()
		if err != nil {
			log.Fatal(fmt.Errorf("couldn't read packet: %w", err))
		}
		shmPacket, ok := p.(*ipc.ShmReaderMessage)
		if !ok {
			log.Fatal(fmt.Errorf("packet is not from shm IPC reader"))
		}

		data := shmPacket.Data()
		receiveTimestamp := shmPacket.ReceiveTimestamp()
		packetSequence := binary.BigEndian.Uint64(data[8:16])

		udpTimestamp := binary.BigEndian.Uint64(data[0:8])
		timestamp := uint64(time.Now().UnixNano())

		udpShmDiff := receiveTimestamp - udpTimestamp
		benchShmDiff := timestamp - receiveTimestamp
		totalDiff := timestamp - udpTimestamp
		if averageDiff == 0 {
			averageDiff = totalDiff
		}

		averageDiff = (averageDiff + totalDiff) / 2
		averageUdpShmDiff = (averageUdpShmDiff + udpShmDiff) / 2
		averageBenchShmDiff = (averageBenchShmDiff + benchShmDiff) / 2

		// the latter condition is a simplification that could mean that some
		// lost packets are not accounted for during an overflow.
		if lastPacketSequence != 0 && lastPacketSequence <= packetSequence {
			packetsLost += uint64(packetSequence-lastPacketSequence) - 1
		}
		lastPacketSequence = packetSequence

		totalPacketsReader += 1
		totalPacketsReceived.Add(1)
	}
}

func reader() {
	go func() {
		var lastPacketsReceived uint64
		for {
			time.Sleep(time.Second * PRINT_INTERVAL)
			totalLoaded := totalPacketsReceived.Load()
			fmt.Printf("Total: %d packets/second\n", (totalLoaded-lastPacketsReceived)/PRINT_INTERVAL)

			lastPacketsReceived = totalLoaded
		}
	}()

	for _, packet := range proc.GswConfig.TelemetryPackets {
		go packetReader(packet)
	}
}

type TelemetryPacket struct {
	Name string
	Port int
	Size int
}

func createPacket(size int, seq uint64) []byte {
	timestamp := time.Now().UnixNano()

	packet := make([]byte, size)

	binary.BigEndian.PutUint64(packet[0:8], uint64(timestamp))
	binary.BigEndian.PutUint64(packet[8:16], seq)

	return packet
}

func packetWriter(serverAddress string, port, size int, writerSleep time.Duration) error {
	serverAddr, err := net.ResolveUDPAddr("udp", fmt.Sprintf("%s:%d", serverAddress, port))
	if err != nil {
		return fmt.Errorf("resolving address: %w", err)
	}

	conn, err := net.DialUDP("udp", nil, serverAddr)
	if err != nil {
		return fmt.Errorf("dialing udp (%s): %w", serverAddr.String(), err)
	}
	defer func() {
		err := conn.Close()
		if err != nil {
			log.Println("couldn't close connection", err)
		}
	}()

	var sequence uint64
	if writerSleep != 0 {
		ticker := time.NewTicker(writerSleep)
		defer ticker.Stop()

		for {
			<-ticker.C
			packet := createPacket(size, sequence)
			_, err := conn.Write(packet)
			if err != nil {
				fmt.Printf("error writing packet (%s): %v\n", serverAddr.String(), err)
			}
			sequence += 1
		}
	} else {
		for {
			packet := createPacket(size, sequence)
			_, err := conn.Write(packet)
			if err != nil {
				fmt.Printf("error writing packet (%s): %v\n", serverAddr.String(), err)
			}
			sequence += 1
		}

	}

}

func writer(serverAddress string, writerSleep time.Duration) {
	var wg sync.WaitGroup

	for _, packet := range proc.GswConfig.TelemetryPackets {
		size := proc.GetPacketSize(packet)
		wg.Add(1)
		go func(serverAddress string, port, size int, writerSleep time.Duration) {
			defer wg.Done()
			err := packetWriter(serverAddress, port, size, writerSleep)
			if err != nil {
				log.Fatal("error running packet writer:", err)
			}
		}(serverAddress, packet.Port, size, writerSleep)
	}
	wg.Wait()
}

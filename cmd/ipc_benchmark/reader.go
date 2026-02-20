package main

import (
	"context"
	"encoding/binary"
	"fmt"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/AarC10/GSW-V2/lib/ipc"
	"github.com/AarC10/GSW-V2/lib/logger"
	"github.com/AarC10/GSW-V2/lib/tlm"
	"github.com/AarC10/GSW-V2/proc"
	"go.uber.org/zap"
)

const (
	PRINT_INTERVAL = 3
)

var totalPacketsReceived atomic.Uint64
var totalPacketsLost atomic.Uint64

func packetReader(ctx context.Context, packet tlm.TelemetryPacket) *OutputPacket {
	reader, err := proc.NewIpcShmReaderForPacket(packet, "/dev/shm")
	if err != nil {
		logger.Fatal("couldn't create reader for packet", zap.Error(err))
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
			select {
			case <-time.After(time.Second * PRINT_INTERVAL):
			case <-ctx.Done():
				return
			}
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

			fmt.Fprint(os.Stderr, sb.String())
		}
	}()

	var lastPacketSequence uint64

	for {
		if err := ctx.Err(); err != nil {
			break
		}
		p, err := reader.Read(ctx)
		if err != nil {
			logger.Fatal("couldn't read packet", zap.Error(err))
		}
		shmPacket, ok := p.(*ipc.ShmReaderMessage)
		if !ok {
			logger.Fatal("packet is not from shm IPC reader")
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
			lost := packetSequence - lastPacketSequence - 1
			packetsLost += lost
			totalPacketsLost.Add(lost)
		}
		lastPacketSequence = packetSequence

		totalPacketsReader += 1
		totalPacketsReceived.Add(1)
	}

	return &OutputPacket{
		Lost:     packetsLost,
		Received: totalPacketsReader,
		Size:     uint64(proc.GetPacketSize(packet)),
		Name:     packet.Name,
	}
}

func reader(ctx context.Context, packets []*tlm.TelemetryPacket) *ReaderOutput {
	go func() {
		var lastPacketsReceived uint64
		for {
			select {
			case <-time.After(time.Second * PRINT_INTERVAL):
			case <-ctx.Done():
				return
			}
			totalLoaded := totalPacketsReceived.Load()
			fmt.Fprintf(os.Stderr, "Total: %d packets/second\n", (totalLoaded-lastPacketsReceived)/PRINT_INTERVAL)

			lastPacketsReceived = totalLoaded
		}
	}()

	start := time.Now()

	output := ReaderOutput{
		Packets: []OutputPacket{},
	}

	var outputMu sync.Mutex

	var wg sync.WaitGroup

	for _, packet := range packets {
		wg.Add(1)
		go func(packet *tlm.TelemetryPacket) {
			defer wg.Done()
			o := packetReader(ctx, *packet)
			outputMu.Lock()
			output.Packets = append(output.Packets, *o)
			outputMu.Unlock()
		}(packet)
	}

	wg.Wait()

	output.Runtime = time.Since(start)
	output.TotalPacketsLost = totalPacketsLost.Load()
	output.TotalPacketsReceived = totalPacketsReceived.Load()

	return &output
}

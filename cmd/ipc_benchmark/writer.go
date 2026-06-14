package main

import (
	"context"
	"encoding/binary"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/AarC10/GSW-V2/lib/logger"
	"github.com/AarC10/GSW-V2/lib/tlm"
	"github.com/AarC10/GSW-V2/proc"
	"go.uber.org/zap"
)

func createPacket(size int, seq uint64) []byte {
	timestamp := time.Now().UnixNano()

	packet := make([]byte, size)

	binary.BigEndian.PutUint64(packet[0:8], uint64(timestamp))
	binary.BigEndian.PutUint64(packet[8:16], seq)

	return packet
}

func packetWriter(ctx context.Context, serverAddress string, port, size int, writerSleep time.Duration) error {
	serverAddr, err := net.ResolveUDPAddr("udp", fmt.Sprintf("%s:%d", serverAddress, port))
	if err != nil {
		return fmt.Errorf("resolving address: %w", err)
	}
	log := logger.Log().Named("packet_writer").With(zap.String("server", serverAddr.String()))

	conn, err := net.DialUDP("udp", nil, serverAddr)
	if err != nil {
		return fmt.Errorf("dialing udp (%s): %w", serverAddr.String(), err)
	}
	defer func() {
		err := conn.Close()
		if err != nil {
			log.Error("couldn't close connection to gsw", zap.Error(err))
		}
	}()

	var sequence uint64
	if writerSleep != 0 {
		ticker := time.NewTicker(writerSleep)
		defer ticker.Stop()

		for {
			if err := ctx.Err(); err != nil {
				return nil
			}
			<-ticker.C

			packet := createPacket(size, sequence)
			_, err := conn.Write(packet)
			if err != nil {
				log.Error("error writing packet", zap.Error(err))
			}
			sequence += 1
		}
	} else {
		for {
			if err := ctx.Err(); err != nil {
				return nil
			}
			packet := createPacket(size, sequence)
			_, err := conn.Write(packet)
			if err != nil {
				log.Error("error writing packet", zap.Error(err))
			}
			sequence += 1
		}
	}
}

func writer(ctx context.Context, serverAddress string, packets []*tlm.TelemetryPacket, writerSleep time.Duration) {
	var wg sync.WaitGroup

	for _, packet := range packets {
		size := proc.GetPacketSize(*packet)
		wg.Add(1)
		go func(serverAddress string, port, size int, writerSleep time.Duration) {
			defer wg.Done()
			err := packetWriter(ctx, serverAddress, port, size, writerSleep)
			if err != nil {
				logger.Fatal("error running packet writer", zap.Error(err))
			}
		}(serverAddress, packet.Port, size, writerSleep)
	}
	wg.Wait()
}

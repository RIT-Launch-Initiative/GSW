package proc

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strconv"
	"sync"

	"github.com/AarC10/GSW-V2/lib/ipc"
	"github.com/AarC10/GSW-V2/lib/logger"
	"github.com/AarC10/GSW-V2/lib/tlm"
	"go.uber.org/zap"
)

// newIpcShmHandlerForPacket creates a shared memory IPC handler for a telemetry packet
// If write is true, the handler will be created for writing to shared memory
// If write is false, the handler will be created for reading from shared memory
func newIpcShmHandlerForPacket(packet tlm.TelemetryPacket, write bool, shmDir string) (*ipc.ShmHandler, error) {
	handler, err := ipc.NewShmHandler(strconv.Itoa(packet.Port), GetPacketSize(packet), write, shmDir)
	if err != nil {
		return nil, fmt.Errorf("error creating shared memory handler: %v", err)
	}

	return handler, nil
}

// TelemetryPacketWriter is a goroutine that listens for telemetry data on a UDP port and writes it to shared memory
func TelemetryPacketWriter(ctx context.Context, packet tlm.TelemetryPacket, outChannel chan []byte, shmDir string) error {
	log := logger.Log().Named("decom").With(zap.String("packet", packet.Name))
	packetSize := GetPacketSize(packet)
	shmWriter, err := newIpcShmHandlerForPacket(packet, true, shmDir)
	if shmWriter == nil {
		return fmt.Errorf("creating shared memory writer: %w", err)
	}
	defer shmWriter.Cleanup()

	log.Info(fmt.Sprintf("Packet size: %d bytes %d bits", packetSize, packetSize*8))

	addr, err := net.ResolveUDPAddr("udp", fmt.Sprintf(":%d", packet.Port))
	if err != nil {
		return fmt.Errorf("resolving listen address: %w", err)
	}

	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		return fmt.Errorf("listening: %w", err)
	}

	var closeConnOnce sync.Once
	closeConn := func() {
		closeConnOnce.Do(func() {
			if err := conn.Close(); err != nil {
				log.Error("error closing UDP connection", zap.Error(err))
			}
		})
	}
	defer closeConn()

	stopf := context.AfterFunc(ctx, closeConn)
	defer stopf()

	log.Info(fmt.Sprintf("Listening on %d for telemetry packet...", packet.Port))

	// Receive data
	buffer := make([]byte, packetSize)
	for {
		n, _, err := conn.ReadFromUDP(buffer)
		if err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}

			// a closed connection would be unrecoverable, so return the error
			if errors.Is(err, net.ErrClosed) {
				return err
			}

			log.Error("error reading from UDP", zap.Error(err))
			continue
		}

		if n == packetSize {
			err := shmWriter.Write(buffer)
			if err != nil {
				log.Error("error writing to shared memory", zap.Error(err))
			}

			select {
			case outChannel <- buffer:
				break
			default:
				break
			}
		} else {
			log.Error("received packet of incorrect size", zap.Int("expected", packetSize), zap.Int("received", n))
		}
	}
}

// NewIpcShmReaderForPacket creates a shared memory IPC reader for a telemetry packet.
func NewIpcShmReaderForPacket(packet tlm.TelemetryPacket, shmDir string) (ipc.Reader, error) {
	return newIpcShmHandlerForPacket(packet, false, shmDir)
}

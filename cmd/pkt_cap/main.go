package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/AarC10/GSW-V2/lib/logger"
	"github.com/AarC10/GSW-V2/proc"
	"github.com/google/gopacket"
	"github.com/google/gopacket/pcap"
	"github.com/google/gopacket/pcapgo"
	"go.uber.org/zap"
)

var shmDir = flag.String("shm", "/dev/shm", "directory to use for shared memory")

func getFilter() (string, error) {
	if len(proc.GswConfig.TelemetryPackets) == 0 {
		return "", fmt.Errorf("no telemetry packets configured")
	}

	ports := make([]string, 0, len(proc.GswConfig.TelemetryPackets))
	for _, packet := range proc.GswConfig.TelemetryPackets {
		ports = append(ports, fmt.Sprintf("udp port %d", packet.Port))
	}

	return strings.Join(ports, " or "), nil
}

func createOutputFile() (*os.File, error) {
	timestamp := time.Now().Format("2006-01-02_15-04-05")
	if err := os.MkdirAll("captures", 0755); err != nil {
		return nil, fmt.Errorf("creating captures directory: %w", err)
	}

	filename := fmt.Sprintf("captures/%s_%s.pcap", proc.GswConfig.Name, timestamp)
	return os.Create(filename)
}

func capture(ctx context.Context) error {
	filter, err := getFilter()
	if err != nil {
		return fmt.Errorf("building capture filter: %w", err)
	}

	snaplen := uint32(65535)
	handle, err := pcap.OpenLive("any", int32(snaplen), true, 100*time.Millisecond)
	if err != nil {
		return fmt.Errorf("opening pcap handle: %w", err)
	}
	defer handle.Close()

	if err := handle.SetBPFFilter(filter); err != nil {
		return fmt.Errorf("setting BPF filter: %w", err)
	}

	pcapFile, err := createOutputFile()
	if err != nil {
		return fmt.Errorf("creating output file: %w", err)
	}
	defer func() {
		if err := pcapFile.Close(); err != nil {
			logger.Error("failed closing pcap file", zap.Error(err))
		}
	}()

	bufferedFile := bufio.NewWriterSize(pcapFile, 128*1024)
	defer func() {
		if err := bufferedFile.Flush(); err != nil {
			logger.Error("failed flushing buffered writer", zap.Error(err))
		}
	}()

	pcapWriter := pcapgo.NewWriterNanos(bufferedFile)
	if err := pcapWriter.WriteFileHeader(snaplen, handle.LinkType()); err != nil {
		return fmt.Errorf("writing pcap file header: %w", err)
	}

	logger.Info("network capture started",
		zap.String("filter", filter),
		zap.String("file", pcapFile.Name()),
	)

	go func() {
		<-ctx.Done()
		handle.Close()
	}()

	packetSource := gopacket.NewPacketSource(handle, handle.LinkType())
	for packet := range packetSource.Packets() {
		if err := pcapWriter.WritePacket(packet.Metadata().CaptureInfo, packet.Data()); err != nil {
			logger.Error("failed writing packet", zap.Error(err))
		}
	}

	return ctx.Err()
}
func main() {
	flag.Parse()
	logger.InitLogger()

	configData, err := proc.ReadTelemetryConfigFromShm(*shmDir)
	if err != nil {
		logger.Fatal("couldn't read config from gsw", zap.Error(err))
	}
	if _, err = proc.ParseConfigBytes(configData); err != nil {
		logger.Fatal("couldn't parse gsw config", zap.Error(err))
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		sig := <-sigs
		logger.Info("received signal", zap.String("signal", sig.String()))
		cancel()
	}()

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := capture(ctx); err != nil {
			logger.Error("network capture error", zap.Error(err))
		}
	}()

	wg.Wait()
	logger.Info("network capture stopped")
}

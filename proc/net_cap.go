package proc

import (
	"bufio"
	"context"
	"fmt"
	"github.com/AarC10/GSW-V2/lib/logger"
	"github.com/google/gopacket"
	"github.com/google/gopacket/pcap"
	"github.com/google/gopacket/pcapgo"
	"go.uber.org/zap"

	"os"
	"strings"
	"time"
)

func getFilter() string {
	ports := []string{}
	for _, packet := range GswConfig.TelemetryPackets {
		ports = append(ports, fmt.Sprintf("udp port %d", packet.Port))
	}

	filter := strings.Join(ports, " or ")
	return filter
}

func createOutputFile() (*os.File, error) {
	// Note the date format isn't random. This is reference time used in Go for formatting time
	timestamp := time.Now().Format("2006-01-02_15-04-05")

	// TODO: Configurable output directory
	if _, err := os.Stat("captures"); os.IsNotExist(err) {
		err := os.Mkdir("captures", 0755)
		if err != nil {
			return nil, fmt.Errorf("error creating captures directory: %v", err)
		}
	}

	filename := fmt.Sprintf("captures/%s_%s.pcap", GswConfig.Name, timestamp)

	return os.Create(filename)
}

func NetworkCapture(ctx context.Context) error {
	snaplen := uint32(1600)
	filter := getFilter()

	handle, err := pcap.OpenLive("any", int32(snaplen), true, 100*time.Millisecond)
	if err != nil {
		logger.Error("failed opening pcap handle:", zap.Error(err))
		return err
	}

	if err := handle.SetBPFFilter(filter); err != nil {
		logger.Error("failed setting BPF filter:", zap.Error(err))
		return err
	}

	pcapFile, err := createOutputFile()
	if err != nil {
		logger.Error("failed creating output file:", zap.Error(err))
		return err
	}
	defer pcapFile.Close()

	// TODO: 128 KB buffer. Make this configurable?
	bufferedFile := bufio.NewWriterSize(pcapFile, 128*1024)
	defer func(bufferedFile *bufio.Writer) {
		err := bufferedFile.Flush()
		if err != nil {
			logger.Error("failed flushing buffered writer:", zap.Error(err))
			return
		}
	}(bufferedFile)

	pcapWriter := pcapgo.NewWriterNanos(bufferedFile)
	if err := pcapWriter.WriteFileHeader(snaplen, handle.LinkType()); err != nil {
		logger.Error("failed writing pcap file header:", zap.Error(err))
		return err
	}

	packetSource := gopacket.NewPacketSource(handle, handle.LinkType())

	logger.Info("Network capture started with filter:", zap.String("filter", filter))
	logger.Info("Writing captured packets to file:", zap.String("filename", pcapFile.Name()))

	go func() {
		<-ctx.Done()
		handle.Close()
	}()

	for packet := range packetSource.Packets() {
		if err := pcapWriter.WritePacket(packet.Metadata().CaptureInfo, packet.Data()); err != nil {
			logger.Error("failed writing packet", zap.Error(err))
		}
	}

	return nil
}

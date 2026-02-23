package proc

import (
	"bufio"
	"context"
	"fmt"
	"github.com/AarC10/GSW-V2/lib/logger"
	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
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
	timestamp := fmt.Sprintf("%d", time.Now().Unix())

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

func NetworkCapture(ctx context.Context) {
	snaplen := uint32(1600)
	filter := getFilter()

	handle, err := pcap.OpenLive("any", int32(snaplen), true, pcap.BlockForever)
	if err != nil {
		logger.Error("failed opening pcap handle:", zap.Error(err))
		return
	}
	defer handle.Close()

	pcapFile, err := createOutputFile()
	if err != nil {
		logger.Error("failed creating output file:", zap.Error(err))
		return
	}
	defer pcapFile.Close()

	bufferedFile := bufio.NewWriterSize(pcapFile, 1<<20)
	defer bufferedFile.Flush()

	pcapWriter := pcapgo.NewWriterNanos(bufferedFile)
	if err := pcapWriter.WriteFileHeader(snaplen, handle.LinkType()); err != nil {
		logger.Error("failed writing pcap file header:", zap.Error(err))
		return
	}

	if err := handle.SetBPFFilter(filter); err != nil {
		logger.Error("failed setting BPF filter:", zap.Error(err))
		return
	}

	packetSource := gopacket.NewPacketSource(handle, handle.LinkType())

	logger.Info("Network capture started with filter:", zap.String("filter", filter))
	logger.Info("Writing captured packets to file:", zap.String("filename", pcapFile.Name()))

	for {
		select {
		case <-ctx.Done():
			logger.Info("Network capture stopped.")
			return
		case packet := <-packetSource.Packets():
			if err := pcapWriter.WritePacket(packet.Metadata().CaptureInfo, packet.Data()); err != nil {
				logger.Error("failed writing packet to pcap file:", zap.Error(err))
			}
		}
	}

}

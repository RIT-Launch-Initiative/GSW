package proc

import (
	"context"
	"fmt"
	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/google/gopacket/pcap"
	"github.com/google/gopacket/pcapgo"

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
		fmt.Printf("Error opening pcap handle: %v\n", err)
		return
	}
	defer handle.Close()

	pcapFile, err := createOutputFile()
	if err != nil {
		fmt.Printf("Error creating output file: %v\n", err)
		return
	}
	defer pcapFile.Close()

	pcapWriter := pcapgo.NewWriterNanos(pcapFile)
	if err := pcapWriter.WriteFileHeader(snaplen, layers.LinkTypeEthernet); err != nil {
		fmt.Printf("Error writing pcap file header: %v\n", err)
		return
	}

	if err := handle.SetBPFFilter(filter); err != nil {
		fmt.Printf("Error setting BPF filter: %v\n", err)
		return
	}

	packetSource := gopacket.NewPacketSource(handle, handle.LinkType())

	for {
		select {
		case <-ctx.Done():
			fmt.Println("Network capture stopped.")
			return
		case packet := <-packetSource.Packets():
			if err := pcapWriter.WritePacket(packet.Metadata().CaptureInfo, packet.Data()); err != nil {
				fmt.Printf("Error writing packet to pcap file: %v\n", err)
			}
		}
	}

}

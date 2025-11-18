package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"net/http"
	_ "net/http/pprof"

	"github.com/AarC10/GSW-V2/lib/tlm"
	"github.com/AarC10/GSW-V2/proc"
)

type packetsMapFlagValue map[*tlm.TelemetryPacket]struct{}

// String implementation for flag.Value.
// Used for diagnostics.
func (p *packetsMapFlagValue) String() string {
	output := make([]string, 0, len(*p))
	for packet := range *p {
		output = append(output, packet.Name)
	}

	return strings.Join(output, ", ")
}

// Set implementation for flag.Value.
// Called for every flag to set the flag value.
func (p *packetsMapFlagValue) Set(value string) error {
	for _, packet := range proc.GswConfig.TelemetryPackets {
		if packet.Name != value {
			continue
		}
		(*p)[&packet] = struct{}{}
		return nil
	}
	return fmt.Errorf("packet not declared in config (restart gsw_service?)")
}

// Packets gets a slice of packets from the packets map.
// If no packets are defined, returns the entire config.
func (p *packetsMapFlagValue) Packets() []*tlm.TelemetryPacket {
	if len(*p) == 0 {
		output := make([]*tlm.TelemetryPacket, len(proc.GswConfig.TelemetryPackets))
		for i, packet := range proc.GswConfig.TelemetryPackets {
			output[i] = &packet
		}
		return output
	} else {
		output := make([]*tlm.TelemetryPacket, 0, len(*p))
		for packet := range *p {
			output = append(output, packet)
		}
		return output
	}
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

func main() {
	configData, err := proc.ReadTelemetryConfigFromShm("/dev/shm")
	if err != nil {
		log.Fatal(fmt.Errorf("couldn't read config from shm: %w", err))
	}
	_, err = proc.ParseConfigBytes(configData)
	if err != nil {
		log.Fatal(fmt.Errorf("couldn't parse shm config: %w", err))
	}

	isReader := flag.Bool("reader", false, "run a gsw reader")
	isWriter := flag.Bool("writer", false, "run a gsw writer")
	writerSleep := flag.Duration("writer_sleep", 0, "approximately how long the writer will sleep between packets")
	serverAddress := flag.String("writer_host", "localhost", "the gsw host that the writer will attempt to write to")
	profilePort := flag.Int("pprof", 0, "run pprof at a port")
	packets := make(packetsMapFlagValue)
	flag.Var(&packets, "packet", "only this packet will be written or read")

	flag.Parse()

	if !*isReader && !*isWriter {
		log.Fatal("use -reader and/or -writer to start the process as a reader or writer")
	}

	if *profilePort != 0 {
		initProfiling(*profilePort)
	}

	packetsSlice := packets.Packets()

	if *isReader {
		log.Println("running reader")
		go reader(packetsSlice)
	}
	if *isWriter {
		log.Println("running writer")
		go writer(*serverAddress, packetsSlice, *writerSleep)
	}

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	<-sigChan
}

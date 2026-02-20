package main

import (
	"context"
	"flag"
	"fmt"
	"strings"
	"sync"

	"net/http"
	_ "net/http/pprof"

	"github.com/AarC10/GSW-V2/lib/logger"
	"github.com/AarC10/GSW-V2/lib/tlm"
	"github.com/AarC10/GSW-V2/proc"
	"go.uber.org/zap"
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
		pprofAddr := fmt.Sprintf("localhost:%d", pprofPort)
		logger.Info("Running pprof server", zap.String("addr", pprofAddr))
		err := http.ListenAndServe(pprofAddr, nil)
		if err != nil {
			logger.Fatal("error starting pprof server", zap.Error(err))
		}
	}()
}

func main() {
	configData, err := proc.ReadTelemetryConfigFromShm("/dev/shm")
	if err != nil {
		logger.Fatal("couldn't read config from shm", zap.Error(err))
	}
	_, err = proc.ParseConfigBytes(configData)
	if err != nil {
		logger.Fatal("couldn't parse shm config", zap.Error(err))
	}

	timeout := flag.Duration("duration", 0, "the test duration")
	isReader := flag.Bool("reader", false, "run a gsw reader")
	var readerOutputFormat outputFormatFlagValue
	flag.Var(&readerOutputFormat, "output", "output format (options: json or a go template, defaults to pretty printing)")

	isWriter := flag.Bool("writer", false, "run a gsw writer")
	writerSleep := flag.Duration("writer_sleep", 0, "approximately how long the writer will sleep between packets")
	serverAddress := flag.String("writer_host", "localhost", "the gsw host that the writer will attempt to write to")

	profilePort := flag.Int("pprof", 0, "run pprof at a port")

	packets := make(packetsMapFlagValue)
	flag.Var(&packets, "packet", "only this packet will be written or read")

	flag.Parse()

	if !*isReader && !*isWriter {
		logger.Fatal("use -reader and/or -writer to start the process as a reader or writer")
	}

	if *profilePort != 0 {
		initProfiling(*profilePort)
	}

	ctx, cancel := context.WithCancel(context.Background())
	if *timeout != 0 {
		ctx, cancel = context.WithTimeout(ctx, *timeout)
	}
	defer cancel()

	packetsSlice := packets.Packets()

	var wg sync.WaitGroup
	if *isReader {
		logger.Info("running reader")
		wg.Add(1)
		go func() {
			defer wg.Done()
			output := reader(ctx, packetsSlice)
			outputString, err := readerOutputFormat.GenerateReaderOutput(*output)
			if err != nil {
				logger.Fatal("couldn't generate output", zap.Error(err))
			}
			fmt.Print(outputString)
		}()
	}
	if *isWriter {
		logger.Info("running writer")
		wg.Add(1)
		go func() {
			defer wg.Done()
			writer(ctx, *serverAddress, packetsSlice, *writerSleep)
		}()
	}

	wg.Wait()
}

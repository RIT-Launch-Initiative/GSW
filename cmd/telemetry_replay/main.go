package main

import (
	"context"
	"encoding/csv"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/AarC10/GSW-V2/lib/logger"
	"github.com/AarC10/GSW-V2/lib/tlm"
	"github.com/AarC10/GSW-V2/proc"
	"go.uber.org/zap"
)

var (
	shmDir       = flag.String("shm", "/dev/shm", "directory to use for shared memory")
	configPath   = flag.String("config", "", "path to telemetry YAML config file (reads from shm if empty)")
	csvPath      = flag.String("csv", "", "path to telemetry CSV file to replay")
	serverAddr   = flag.String("server", "127.0.0.1", "destination UDP server address")
	packetsFlag  = flag.String("packets", "", "comma-separated list of packet names to send (all if empty)")
	speed        = flag.Float64("speed", 1.0, "playback speed multiplier (e.g. 2.0 for 2x, 0 for max speed)")
	startFlag    = flag.String("start", "", "start offset (e.g. 10s, 1m) or absolute timestamp (unix ns or RFC3339)")
	durationFlag = flag.Duration("duration", 0, "maximum playback duration from start (0 for entire file)")
	loop         = flag.Bool("loop", false, "loop playback continuously")
	ignoreExtra  = flag.Bool("ignore-extra", false, "ignore CSV fields not found in telemetry metadata")
)

var metadataColumns = map[string]bool{
	"name": true,
	"tags": true,
	"time": true,
}

func main() {
	flag.Parse()

	if *csvPath == "" {
		logger.Fatal("-csv flag is required")
	}

	if err := loadConfig(); err != nil {
		logger.Fatal("error loading telemetry config", zap.Error(err))
	}

	selectedPackets, err := selectPackets(*packetsFlag)
	if err != nil {
		logger.Fatal("error selecting packets", zap.Error(err))
	}

	conns, err := openUDPConnections(*serverAddr, selectedPackets)
	if err != nil {
		logger.Fatal("error opening UDP connections", zap.Error(err))
	}
	defer func() {
		for _, conn := range conns {
			_ = conn.Close()
		}
	}()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		<-sigChan
		logger.Info("shutting down")
		cancel()
	}()

	for {
		if err := replayCSV(ctx, *csvPath, selectedPackets, conns); err != nil {
			if errors.Is(err, context.Canceled) {
				return
			}
			logger.Fatal("error replaying CSV", zap.Error(err))
		}
		if !*loop {
			break
		}
		logger.Info("looping CSV playback")
	}
}

func loadConfig() error {
	var configData []byte
	var err error

	if *configPath != "" {
		configData, err = os.ReadFile(*configPath)
		if err != nil {
			return fmt.Errorf("reading config file: %w", err)
		}
	} else {
		configData, err = proc.ReadTelemetryConfigFromShm(*shmDir)
		if err != nil {
			return fmt.Errorf("reading config from shm: %w", err)
		}
	}

	if _, err := proc.ParseConfigBytes(configData); err != nil {
		return fmt.Errorf("parsing config: %w", err)
	}
	return nil
}

func selectPackets(filter string) ([]tlm.TelemetryPacket, error) {
	if filter == "" {
		return proc.GswConfig.TelemetryPackets, nil
	}

	wanted := make(map[string]bool)
	for _, name := range strings.Split(filter, ",") {
		name = strings.TrimSpace(name)
		if name != "" {
			wanted[name] = true
		}
	}

	var selected []tlm.TelemetryPacket
	found := make(map[string]bool)
	for _, pkt := range proc.GswConfig.TelemetryPackets {
		if wanted[pkt.Name] {
			selected = append(selected, pkt)
			found[pkt.Name] = true
		}
	}

	for name := range wanted {
		if !found[name] {
			return nil, fmt.Errorf("unknown packet in -packets: %s", name)
		}
	}
	return selected, nil
}

func openUDPConnections(host string, packets []tlm.TelemetryPacket) (map[string]*net.UDPConn, error) {
	conns := make(map[string]*net.UDPConn, len(packets))
	for _, pkt := range packets {
		addr, err := net.ResolveUDPAddr("udp", fmt.Sprintf("%s:%d", host, pkt.Port))
		if err != nil {
			return nil, fmt.Errorf("resolving address for %s: %w", pkt.Name, err)
		}
		conn, err := net.DialUDP("udp", nil, addr)
		if err != nil {
			return nil, fmt.Errorf("dialing UDP for %s (%s): %w", pkt.Name, addr.String(), err)
		}
		conns[pkt.Name] = conn
		logger.Info("ready to stream packet", zap.String("packet", pkt.Name), zap.String("addr", addr.String()))
	}
	return conns, nil
}

func replayCSV(ctx context.Context, path string, packets []tlm.TelemetryPacket, conns map[string]*net.UDPConn) error {
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("opening csv: %w", err)
	}
	defer f.Close()

	reader := csv.NewReader(f)
	header, err := reader.Read()
	if err != nil {
		return fmt.Errorf("reading csv header: %w", err)
	}

	timeIdx, colMap, err := validateHeader(header)
	if err != nil {
		return err
	}

	seq := make(map[string]uint64, len(packets))
	var firstTS, startTS, endTS, prevTS int64
	initialized := false

	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		row, err := reader.Read()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("reading csv row: %w", err)
		}

		ts, err := strconv.ParseInt(row[timeIdx], 10, 64)
		if err != nil {
			return fmt.Errorf("invalid timestamp %q: %w", row[timeIdx], err)
		}

		if !initialized {
			firstTS = ts
			startTS, err = computeStartTS(firstTS, *startFlag)
			if err != nil {
				return err
			}
			if *durationFlag > 0 {
				endTS = startTS + durationFlag.Nanoseconds()
			}
			initialized = true
		}

		if ts < startTS {
			continue
		}
		if endTS > 0 && ts > endTS {
			return nil
		}

		if prevTS != 0 && *speed > 0 && ts > prevTS {
			sleepDur := time.Duration(float64(ts-prevTS) / *speed)
			timer := time.NewTimer(sleepDur)
			select {
			case <-ctx.Done():
				timer.Stop()
				return ctx.Err()
			case <-timer.C:
			}
		}
		prevTS = ts

		for _, pkt := range packets {
			buf, matched, err := buildPacket(pkt, row, colMap, ts, seq[pkt.Name])
			if err != nil {
				logger.Error("error building packet", zap.String("packet", pkt.Name), zap.Error(err))
				continue
			}
			if !matched {
				continue
			}

			logger.Info("sending packet", zap.String("packet", pkt.Name), zap.String("data", hex.EncodeToString(buf)))

			if _, err := conns[pkt.Name].Write(buf); err != nil {
				logger.Error("error sending packet", zap.String("packet", pkt.Name), zap.Error(err))
				continue
			}
			seq[pkt.Name]++
		}
	}
}

func validateHeader(header []string) (int, map[string]int, error) {
	timeIdx := -1
	colMap := make(map[string]int)
	var unmatched []string

	for i, col := range header {
		if col == "time" {
			timeIdx = i
			continue
		}
		if metadataColumns[col] {
			continue
		}
		if _, ok := proc.GswConfig.Measurements[col]; !ok {
			unmatched = append(unmatched, col)
			continue
		}
		colMap[col] = i
	}

	if timeIdx == -1 {
		return -1, nil, fmt.Errorf("csv missing required 'time' column")
	}
	if len(unmatched) > 0 {
		if !*ignoreExtra {
			return -1, nil, fmt.Errorf("csv fields not found in telemetry metadata: %s", strings.Join(unmatched, ", "))
		}
		logger.Info("ignoring extra csv fields", zap.Strings("fields", unmatched))
	}
	return timeIdx, colMap, nil
}

func computeStartTS(firstTS int64, start string) (int64, error) {
	if start == "" {
		return firstTS, nil
	}
	if d, err := time.ParseDuration(start); err == nil {
		return firstTS + d.Nanoseconds(), nil
	}
	if ns, err := strconv.ParseInt(start, 10, 64); err == nil {
		return ns, nil
	}
	if t, err := time.Parse(time.RFC3339, start); err == nil {
		return t.UnixNano(), nil
	}
	return 0, fmt.Errorf("invalid -start value %q (use duration like 10s, unix ns, or RFC3339)", start)
}

func buildPacket(pkt tlm.TelemetryPacket, row []string, colMap map[string]int, csvTS int64, seq uint64) ([]byte, bool, error) {
	payloadSize := 0
	hasData := false

	for _, name := range pkt.Measurements {
		meas, ok := proc.GswConfig.Measurements[name]
		if !ok {
			return nil, false, fmt.Errorf("measurement %s missing from config", name)
		}
		payloadSize += meas.Size

		if idx, exists := colMap[name]; exists && row[idx] != "" {
			hasData = true
		}
	}

	if !hasData {
		return nil, false, nil
	}

	buf := make([]byte, payloadSize)

	offset := 0
	for _, name := range pkt.Measurements {
		meas := proc.GswConfig.Measurements[name]
		idx, exists := colMap[name]
		if !exists || row[idx] == "" {
			offset += meas.Size
			continue
		}

		var raw []byte
		var err error
		raw, err = tlm.SerializeMeasurementValueString(meas, row[idx])
		if err != nil {
			return nil, false, fmt.Errorf("serializing %s=%q: %w", name, row[idx], err)
		}

		copy(buf[offset:offset+meas.Size], raw)
		offset += meas.Size
	}

	return buf, true, nil
}

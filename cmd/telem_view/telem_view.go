package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/AarC10/GSW-V2/lib/logger"
	"github.com/AarC10/GSW-V2/lib/tlm"
	"github.com/AarC10/GSW-V2/lib/util"
	"github.com/AarC10/GSW-V2/proc"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"go.uber.org/zap"
)

const valueColWidth = 12

var shmDir = flag.String("shm", "/dev/shm", "directory to use for shared memory")
var fpsLimit = flag.Int("fps", 0, "max UI frames per second (0 = unlimited)")

var updateCounter atomic.Uint64
var pendingUpdate atomic.Bool

// padValue will left justify any string into a field of width valueColWidth
func padValue(s string) string {
	return fmt.Sprintf("%-*s", valueColWidth, s)
}

func main() {
	flag.Parse()
	configData, err := proc.ReadTelemetryConfigFromShm(*shmDir)
	if err != nil {
		logger.Fatal("couldn't read config from gsw", zap.Error(err))
	}
	if _, err = proc.ParseConfigBytes(configData); err != nil {
		logger.Fatal("couldn't parse gsw config", zap.Error(err))
	}

	var hexOn atomic.Bool
	var binOn atomic.Bool
	hexOn.Store(false)
	binOn.Store(false)

	app := tview.NewApplication()
	table := tview.NewTable().
		SetBorders(false)

	// top bar: left title, right update rate
	topLeft := tview.NewTextView().SetDynamicColors(true).SetText("[::b]Telemetry Viewer")
	topRight := tview.NewTextView().SetDynamicColors(true).SetTextAlign(tview.AlignRight).SetText("0FPS")
	topBar := tview.NewFlex().SetDirection(tview.FlexColumn).
		AddItem(topLeft, 0, 1, false).
		AddItem(topRight, 12, 0, false)

	// Name column
	table.SetCell(0, 0,
		tview.NewTableCell("[::b]Name").
			SetAlign(tview.AlignLeft))
	// Value column, padded so it's exactly valueColWidth wide
	table.SetCell(0, 1,
		tview.NewTableCell("[::b]Value"+strings.Repeat(" ", valueColWidth-len("Value"))).
			SetAlign(tview.AlignCenter))
	// HEX and BIN as before, with a little left padding
	table.SetCell(0, 2,
		tview.NewTableCell("[::b]     HEX").
			SetAlign(tview.AlignCenter))
	table.SetCell(0, 3,
		tview.NewTableCell("[::b]     BIN").
			SetAlign(tview.AlignCenter))

	row := 1
	for _, packet := range proc.GswConfig.TelemetryPackets {
		for _, name := range packet.Measurements {
			// pad the initial “–” in the Value column
			table.SetCell(row, 0, tview.NewTableCell(name))
			table.SetCell(row, 1, tview.NewTableCell(padValue("–")))
			table.SetCell(row, 2, tview.NewTableCell(""))
			table.SetCell(row, 3, tview.NewTableCell(""))
			row++
		}
		// spacer row
		table.SetCell(row, 0, tview.NewTableCell(" "))
		row++
	}

	statusBar := tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignCenter)
	updateStatus := func() {
		h, b := "OFF", "OFF"
		if hexOn.Load() {
			h = "ON"
		}
		if binOn.Load() {
			b = "ON"
		}
		statusBar.SetText(fmt.Sprintf("(h) HEX %s  | (b) BINARY %s ", h, b))
	}
	updateStatus()

	go func() {
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		for range ticker.C {
			count := updateCounter.Swap(0)
			rateStr := fmt.Sprintf("%dFPS", count)
			app.QueueUpdateDraw(func() {
				topRight.SetText(rateStr)
			})
		}
	}()

	go func() {
		var interval time.Duration
		if *fpsLimit > 0 {
			interval = time.Second / time.Duration(*fpsLimit)
		} else {
			// when fps limit is 0 (unlimited), do ~60 FPS
			interval = time.Second / 60
		}
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for range ticker.C {
			if !pendingUpdate.Load() {
				continue
			}
			app.QueueUpdateDraw(func() {
				// clear pending flag and count this frame
				pendingUpdate.Store(false)
				updateCounter.Add(1)
			})
		}
	}()

	// live telem readers
	rowIndex := 1
	for _, packet := range proc.GswConfig.TelemetryPackets {
		go func(pkt tlm.TelemetryPacket, baseRow int) {
			log := logger.Log().Named("packet_reader").With(zap.String("packet", pkt.Name))
			reader, err := proc.NewIpcShmReaderForPacket(pkt, *shmDir)
			if err != nil {
				log.Error("error creating reader", zap.Error(err))
				return
			}
			defer reader.Cleanup()

			for {
				p, err := reader.Read(context.TODO())
				if err != nil {
					log.Error("error reading packet", zap.Error(err))
					continue
				}
				data := p.Data()

				offset := 0

				// prep slices to collect all updates for this packet
				measCount := len(pkt.Measurements)
				valStrs := make([]string, measCount)
				hexStrs := make([]string, measCount)
				binStrs := make([]string, measCount)

				// capture flags locally so we avoid closure/capture races
				hexLocal := hexOn.Load()
				binLocal := binOn.Load()

				for i, name := range pkt.Measurements {
					meas, ok := proc.GswConfig.Measurements[name]
					if !ok || offset+meas.Size > len(data) {
						valStrs[i] = padValue("–")
						continue
					}
					val, err := tlm.InterpretMeasurementValue(meas, data[offset:offset+meas.Size])
					if err != nil {
						val = "err"
					}

					// format value
					switch v := val.(type) {
					case float32, float64:
						val = fmt.Sprintf("%.8f", v)
					}
					valStr := fmt.Sprintf("%v", val)
					valStrs[i] = padValue(valStr)

					// HEX
					if hexLocal {
						hexStrs[i] = util.Base16String(data[offset:offset+meas.Size], 1)
					}

					// BIN
					if binLocal {
						var parts []string
						for _, b := range data[offset : offset+meas.Size] {
							s := fmt.Sprintf("%08b", b)
							parts = append(parts, s[:4]+" "+s[4:])
						}
						binStrs[i] = strings.Join(parts, " ")
					}

					offset += meas.Size
				}

				// enqueue UI mutation for the entire measurement group (batch)
				app.QueueUpdate(func() {
					for i := 0; i < measCount; i++ {
						table.GetCell(baseRow+i, 1).SetText(valStrs[i])
						table.GetCell(baseRow+i, 2).SetText(hexStrs[i])
						table.GetCell(baseRow+i, 3).SetText(binStrs[i])
					}
				})
				// mark pending updates to draw
				pendingUpdate.Store(true)
			}
		}(packet, rowIndex)

		rowIndex += len(packet.Measurements) + 1
	}

	// Capture 'h' and 'b' globally
	app.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Rune() {
		case 'h', 'H':
			hexOn.Store(!hexOn.Load())
			updateStatus()
		case 'b', 'B':
			binOn.Store(!binOn.Load())
			updateStatus()
		}
		return event
	})

	// table on top, status bar at bottom
	flex := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(topBar, 1, 1, false).
		AddItem(table, 0, 1, true).
		AddItem(statusBar, 1, 1, false)

	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		<-sigChan
		app.Stop()
	}()

	if err := app.SetRoot(flex, true).Run(); err != nil {
		panic(err)
	}
}

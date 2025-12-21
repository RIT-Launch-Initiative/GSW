package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/AarC10/GSW-V2/lib/tlm"
	"github.com/AarC10/GSW-V2/lib/util"
	"github.com/AarC10/GSW-V2/proc"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

const valueColWidth = 12

var shmDir = flag.String("shm", "/dev/shm", "directory to use for shared memory")

var updateCounter uint64

// padValue will left justify any string into a field of width valueColWidth
func padValue(s string) string {
	return fmt.Sprintf("%-*s", valueColWidth, s)
}

func main() {
	flag.Parse()
	configData, err := proc.ReadTelemetryConfigFromShm(*shmDir)
	if err != nil {
		fmt.Println("*** Error accessing config file. Make sure the GSW service is running. ***")
		fmt.Printf("(%v)\n", err)
		return
	}
	if _, err = proc.ParseConfigBytes(configData); err != nil {
		fmt.Printf("Error parsing YAML: %v\n", err)
		return
	}

	hexOn := false
	binOn := false

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
		if hexOn {
			h = "ON"
		}
		if binOn {
			b = "ON"
		}
		statusBar.SetText(fmt.Sprintf("(h) HEX %s  | (b) BINARY %s ", h, b))
	}
	updateStatus()

	go func() {
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		for range ticker.C {
			count := atomic.SwapUint64(&updateCounter, 0)
			rateStr := fmt.Sprintf("%dFPS", count)
			app.QueueUpdateDraw(func() {
				topRight.SetText(rateStr)
			})
		}
	}()

	// live telem readers
	rowIndex := 1
	for _, packet := range proc.GswConfig.TelemetryPackets {
		go func(pkt tlm.TelemetryPacket, baseRow int) {
			reader, err := proc.NewIpcShmReaderForPacket(pkt, *shmDir)
			if err != nil {
				fmt.Printf("Error creating reader: %v\n", err)
				return
			}
			defer reader.Cleanup()

			for {
				p, err := reader.Read()
				if err != nil {
					fmt.Printf("Error reading packet: %v\n", err)
					continue
				}
				data := p.Data()

				offset := 0
				for i, name := range pkt.Measurements {
					meas, ok := proc.GswConfig.Measurements[name]
					if !ok || offset+meas.Size > len(data) {
						continue
					}
					val, err := tlm.InterpretMeasurementValue(meas, data[offset:offset+meas.Size])
					if err != nil {
						val = "err"
					}

					// format & pad value
					switch v := val.(type) {
					case float32, float64:
						val = fmt.Sprintf("%.8f", v)
					}
					valStr := fmt.Sprintf("%v", val)
					valStr = padValue(valStr)

					// HEX
					hexStr := ""
					if hexOn {
						hexStr = util.Base16String(data[offset:offset+meas.Size], 1)
					}

					// BIN
					binStr := ""
					if binOn {
						var parts []string
						for _, b := range data[offset : offset+meas.Size] {
							s := fmt.Sprintf("%08b", b)
							parts = append(parts, s[:4]+" "+s[4:])
						}
						binStr = strings.Join(parts, " ")
					}

					// update table
					app.QueueUpdateDraw(func() {
						table.GetCell(baseRow+i, 1).SetText(valStr)
						table.GetCell(baseRow+i, 2).SetText(hexStr)
						table.GetCell(baseRow+i, 3).SetText(binStr)
						// count this UI update
						atomic.AddUint64(&updateCounter, 1)
					})
					offset += meas.Size
				}
			}
		}(packet, rowIndex)

		rowIndex += len(packet.Measurements) + 1
	}

	// Capture 'h' and 'b' globally
	app.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Rune() {
		case 'h', 'H':
			hexOn = !hexOn
			updateStatus()
		case 'b', 'B':
			binOn = !binOn
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

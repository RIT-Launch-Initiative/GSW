package main

import (
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/AarC10/GSW-V2/lib/ipc"
	"github.com/AarC10/GSW-V2/lib/tlm"
	"github.com/AarC10/GSW-V2/lib/util"
	"github.com/AarC10/GSW-V2/proc"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

func main() {
	configReader, err := ipc.CreateIpcShmReader("telemetry-config")
	if err != nil {
		fmt.Println("*** Error accessing config file. Make sure the GSW service is running. ***")
		fmt.Printf("(%v)\n", err)
		return
	}
	data, err := configReader.ReadNoTimestamp()
	if err != nil {
		fmt.Printf("Error reading shared memory: %v\n", err)
		return
	}
	if _, err = proc.ParseConfigBytes(data); err != nil {
		fmt.Printf("Error parsing YAML: %v\n", err)
		return
	}

	hexOn := false
	binOn := false

	app := tview.NewApplication()
	table := tview.NewTable().
		SetBorders(false)

	table.SetCell(0, 0, tview.NewTableCell("[::b]Name"))
	table.SetCell(0, 1, tview.NewTableCell("[::b]Value"))
	table.SetCell(0, 2, tview.NewTableCell("[::b]HEX"))
	table.SetCell(0, 3, tview.NewTableCell("[::b]BIN"))

	row := 1
	for _, packet := range proc.GswConfig.TelemetryPackets {
		for _, name := range packet.Measurements {
			table.SetCell(row, 0, tview.NewTableCell(name))
			table.SetCell(row, 1, tview.NewTableCell("–"))
			table.SetCell(row, 2, tview.NewTableCell("")) // hex column
			table.SetCell(row, 3, tview.NewTableCell("")) // binary column
			row++
		}
		// Spacer row
		table.SetCell(row, 0, tview.NewTableCell(" "))
		row++
	}

	statusBar := tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignCenter)
	updateStatus := func() {
		hexText := "OFF"
		if hexOn {
			hexText = "ON"
		}
		binText := "OFF"
		if binOn {
			binText = "ON"
		}
		statusBar.SetText(fmt.Sprintf("(h) HEX %s  | (b) BINARY %s ", hexText, binText))
	}
	updateStatus()

	// live telem readers
	rowIndex := 1
	for _, packet := range proc.GswConfig.TelemetryPackets {
		outChan := make(chan []byte)
		go proc.TelemetryPacketReader(packet, outChan)

		go func(pkt tlm.TelemetryPacket, baseRow int) {
			for data := range outChan {
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

					hexStr := ""
					if hexOn {
						hexStr = util.Base16String(data[offset:offset+meas.Size], 1)
					}

					binStr := ""
					if binOn {
						var parts []string
						for _, b := range data[offset : offset+meas.Size] {
							s := fmt.Sprintf("%08b", b)
							parts = append(parts, s[:4]+" "+s[4:])
						}
						binStr = strings.Join(parts, " ")
					}

					// update
					app.QueueUpdateDraw(func() {
						table.GetCell(baseRow+i, 1).SetText(fmt.Sprintf("%v", val))
						table.GetCell(baseRow+i, 2).SetText(hexStr)
						table.GetCell(baseRow+i, 3).SetText(binStr)
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
		AddItem(table, 0, 1, true).
		AddItem(statusBar, 1, 1, false)

	// sig handler
	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		<-sigChan
		app.Stop()
	}()

	// Run
	if err := app.SetRoot(flex, true).Run(); err != nil {
		panic(err)
	}
}

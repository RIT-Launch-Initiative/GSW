package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/AarC10/GSW-V2/lib/ipc"
	"github.com/AarC10/GSW-V2/lib/tlm"
	"github.com/AarC10/GSW-V2/lib/util"
	"github.com/AarC10/GSW-V2/proc"
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
	_, err = proc.ParseConfigBytes(data)
	if err != nil {
		fmt.Printf("Error parsing YAML: %v\n", err)
		return
	}

	app := tview.NewApplication()
	table := tview.NewTable().SetBorders(false)

	row := 0
	for _, packet := range proc.GswConfig.TelemetryPackets {
		for _, name := range packet.Measurements {
			table.SetCell(row, 0, tview.NewTableCell(name))
			table.SetCell(row, 1, tview.NewTableCell("..."))
			table.SetCell(row, 2, tview.NewTableCell("[hex]"))
			row++
		}
		// Spacer
		table.SetCell(row, 0, tview.NewTableCell(" "))
		row++
	}

	rowIndex := 0
	for _, packet := range proc.GswConfig.TelemetryPackets {
		outChan := make(chan []byte)
		go proc.TelemetryPacketReader(packet, outChan)

		go func(pkt tlm.TelemetryPacket, rowStart int) {
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
					hex := util.Base16String(data[offset:offset+meas.Size], 1)

					// Update UI
					app.QueueUpdateDraw(func() {
						table.GetCell(rowStart+i, 1).SetText(fmt.Sprintf("%v", val))
						table.GetCell(rowStart+i, 2).SetText(hex)
					})

					offset += meas.Size
				}
			}
		}(packet, rowIndex)
		rowIndex += len(packet.Measurements) + 1
	}

	// SIGINT handler
	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		<-sigChan
		app.Stop()
	}()

	if err := app.SetRoot(table, true).Run(); err != nil {
		panic(err)
	}
}

package main

import (
	"fmt"
	"image/color"
	"log"
	"os"

	"github.com/AarC10/GSW-V2/lib/ipc"
	"github.com/AarC10/GSW-V2/lib/tlm"
	"github.com/AarC10/GSW-V2/proc"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
)

const (
	screenWidth  = 1920
	screenHeight = 500
)

type Window struct {
	inited      bool
	measurments map[string]string
}

func packetInterpreter(graphics *Window, packet tlm.TelemetryPacket, rcvChan chan []byte) {
	for {
		data := <-rcvChan
		offset := 0
		for _, measurmentName := range packet.Measurements {
			measurement, ok := proc.GswConfig.Measurements[measurmentName]
			if !ok {
				continue
			}

			value, err := tlm.InterpretMeasurementValue(measurement, data[offset:offset+measurement.Size])
			if err != nil {
				continue
			}

			graphics.measurments[measurmentName] = fmt.Sprintf("%v", value)

			offset += measurement.Size
		}
	}
}

func packetHandler(graphics *Window) {
	for _, packet := range proc.GswConfig.TelemetryPackets {
		outChan := make(chan []byte)
		go proc.TelemetryPacketReader(packet, outChan)
		go packetInterpreter(graphics, packet, outChan)
	}
}

func (graphics *Window) init() {
	defer func() {
		graphics.inited = true
	}()

	graphics.measurments = make(map[string]string)

	//Setup to read from SHM
	configReader, err := ipc.CreateIpcShmReader("telemetry-config")
	if err != nil {
		fmt.Println("*** Error accessing config file. Make sure the GSW service is running. ***")
		fmt.Printf("(%v)\n", err)
		os.Exit(1)
	}

	data, err := configReader.ReadNoTimestamp()
	if err != nil {
		fmt.Printf("Error reading shared memory: %v\n", err)
		os.Exit(1)
	}

	_, err = proc.ParseConfigBytes(data)
	if err != nil {
		fmt.Printf("Error parsing YAML: %v\n", err)
		os.Exit(1)
	}

	go packetHandler(graphics)
}

func (graphics *Window) Update() error {
	if !graphics.inited {
		graphics.init()
	}

	return nil
}

func (graphics *Window) Draw(screen *ebiten.Image) {
	screen.Fill(color.RGBA{0, 255, 0, 100})
	val, ok := graphics.measurments["PRESS_MS5611"]
	if ok {
		ebitenutil.DebugPrintAt(screen, fmt.Sprintf("%s: %s", "PRESS_MS5611", val), 0, 0)
	}

}

func (graphics *Window) Layout(outsideWidth, outsideHeight int) (int, int) {
	return screenWidth, screenHeight
}

func main() {
	// Graphics Starts
	ebiten.SetWindowSize(screenWidth, screenHeight)
	ebiten.SetWindowTitle("RIT Launch Initiative Ground Station Overlay")
	if err := ebiten.RunGame(&Window{}); err != nil {
		log.Fatal(err)
	}
}

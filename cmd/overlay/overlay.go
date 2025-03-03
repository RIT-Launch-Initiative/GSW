package main

import (
	"fmt"
	"image/color"
	"log"
	"math"
	"os"
	"strconv"
	"strings"

	"github.com/AarC10/GSW-V2/lib/ipc"
	"github.com/AarC10/GSW-V2/lib/tlm"
	"github.com/AarC10/GSW-V2/proc"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
)

const (
	SCREEN_WIDTH      = 1920
	SCREEN_HEIGHT     = 500
	STATION_PREASSURE = 103  // in kPa, currently from Sapceport America, Truth or Consequences, NM 3/2/2025
	STATION_TEMP      = 16   // in degrees C, currently from Sapceport America, Truth or Consequences, NM 3/2/2025
	STATION_ELEVATION = 1400 // in meters, currently from Sapceport America, Truth or Consequences, NM 3/2/2025
)

type Window struct {
	inited        bool
	measurments   map[string]string
	displayValues map[string]string
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
	graphics.displayValues = make(map[string]string)

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

	// Caclulate altitude
	ms5611P, ms5611POk := graphics.measurments["PRESS_MS5611"]
	if ms5611POk {
		pressFloat, _ := strconv.ParseFloat(strings.TrimSpace(ms5611P), 64)
		altitude := (1 - math.Pow(pressFloat/1013.25, 0.190284)) * 145366.45
		graphics.displayValues["altitude"] = fmt.Sprintf("%v", int(altitude))
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
	return SCREEN_WIDTH, SCREEN_HEIGHT
}

func main() {
	ebiten.SetWindowSize(SCREEN_WIDTH, SCREEN_HEIGHT)
	ebiten.SetWindowTitle("RIT Launch Initiative Ground Station Overlay")
	if err := ebiten.RunGame(&Window{}); err != nil {
		log.Fatal(err)
	}
}

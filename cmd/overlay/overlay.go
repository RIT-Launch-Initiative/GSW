package main

import (
	"bytes"
	"fmt"
	"image/color"
	"log"
	"math"
	"os"
	"strconv"
	"strings"

	"github.com/AarC10/GSW-V2/assets/fonts"
	"github.com/AarC10/GSW-V2/lib/ipc"
	"github.com/AarC10/GSW-V2/lib/tlm"
	"github.com/AarC10/GSW-V2/proc"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/text/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

const (
	SCREEN_WIDTH      = 1920
	SCREEN_HEIGHT     = 300
	STATION_PREASSURE = 103  // in kPa, currently from Sapceport America, Truth or Consequences, NM 3/2/2025
	STATION_TEMP      = 16   // in degrees C, currently from Sapceport America, Truth or Consequences, NM 3/2/2025
	STATION_ELEVATION = 4600 // in feet, currently from Sapceport America, Truth or Consequences, NM 3/2/2025
)

var (
	robotoFontSource *text.GoTextFaceSource
)

type Window struct {
	inited        bool
	measurments   map[string]string
	displayValues map[string]string
	middle        *ebiten.Image
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

	source, err := text.NewGoTextFaceSource(bytes.NewReader(fonts.RobotoMonoVariable_ttf))
	if err != nil {
		log.Fatal(err)
	}
	robotoFontSource = source

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

	graphics.middle = ebiten.NewImage(800, SCREEN_HEIGHT)
}

func (graphics *Window) Update() error {
	if !graphics.inited {
		graphics.init()
	}

	// Caclulate altitude
	ms5611P, ms5611POk := graphics.measurments["PRESS_MS5611"]
	if ms5611POk {
		pressFloat, _ := strconv.ParseFloat(strings.TrimSpace(ms5611P), 64)
		altitude := (1 - math.Pow((pressFloat*10)/1013.25, 0.190284)) * 145366.45
		graphics.displayValues["altitude"] = fmt.Sprintf("%v", int(altitude)-STATION_ELEVATION)
	}

	return nil
}

func (graphics *Window) drawMiddle(screen *ebiten.Image) {
	// Draw background
	vector.DrawFilledRect(graphics.middle, 0, 0, 800, SCREEN_HEIGHT, color.Black, false)

	// Draw altittude
	val, ok := graphics.displayValues["altitude"]
	if ok {
		altitude := fmt.Sprintf("Altitude: %5s ft", val)
		altOp := &text.DrawOptions{}
		altOp.GeoM.Translate(400, 20)
		altOp.ColorScale.ScaleWithColor(color.White)
		text.Draw(graphics.middle, altitude, &text.GoTextFace{
			Source: robotoFontSource,
			Size:   24,
		}, altOp)
	}

	middleOp := &ebiten.DrawImageOptions{}
	middleOp.GeoM.Translate(SCREEN_WIDTH/2-400, 0)
	screen.DrawImage(graphics.middle, middleOp)
}

func (graphics *Window) Draw(screen *ebiten.Image) {
	screen.Fill(color.RGBA{0, 255, 0, 100})

	graphics.drawMiddle(screen)
}

func (graphics *Window) Layout(outsideWidth, outsideHeight int) (int, int) {
	return SCREEN_WIDTH, SCREEN_HEIGHT
}

func main() {
	ebiten.SetWindowSize(SCREEN_WIDTH, SCREEN_HEIGHT)
	ebiten.SetWindowTitle("RIT Launch Initiative Ground Station Overlay")
	ebiten.SetRunnableOnUnfocused(true)
	if err := ebiten.RunGame(&Window{}); err != nil {
		log.Fatal(err)
	}
}

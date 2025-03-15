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
	"sync"

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
	SCREEN_HEIGHT     = 200
	SIDE_WIDTH        = 400
	MIDDLE_WIDTH      = 600
	STATION_PREASSURE = 103  // in kPa, currently from Sapceport America, Truth or Consequences, NM 3/2/2025
	STATION_TEMP      = 16   // in degrees C, currently from Sapceport America, Truth or Consequences, NM 3/2/2025
	STATION_ELEVATION = 4600 // in feet, currently from Sapceport America, Truth or Consequences, NM 3/2/2025
)

var (
	robotoFontSource *text.GoTextFaceSource
)

type Window struct {
	inited        bool
	measurments   sync.Map
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

			graphics.measurments.Store(measurmentName, fmt.Sprintf("%v", value))

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

	graphics.displayValues = make(map[string]string)

	source, err := text.NewGoTextFaceSource(bytes.NewReader(fonts.RobotoMonoVariable_ttf))
	if err != nil {
		log.Fatal(err)
	}
	robotoFontSource = source

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

	graphics.displayValues["altitude"] = "0"
	graphics.displayValues["vbat"] = "0.00"
	graphics.displayValues["temperature"] = "0.00"
	graphics.displayValues["acceleration"] = "0.0"
}

func (graphics *Window) Update() error {
	if !graphics.inited {
		graphics.init()
	}

	// Caclulate altitude
	ms5611P, ms5611POk := graphics.measurments.Load("PRESS_MS5611")
	if ms5611POk {
		pressFloat, _ := strconv.ParseFloat(strings.TrimSpace(ms5611P.(string)), 64)
		altitude := (1 - math.Pow((pressFloat*10)/1013.25, 0.190284)) * 145366.45
		graphics.displayValues["altitude"] = fmt.Sprintf("%v", int(altitude)-STATION_ELEVATION)
	}

	// Get VBAT
	VBAT, VBATOk := graphics.measurments.Load("VOLT_BATT")
	if VBATOk {
		graphics.displayValues["vbat"] = VBAT.(string)
	}

	// Get Temp
	ms5611T, ms5611TOk := graphics.measurments.Load("TEMP_MS5611")
	if ms5611TOk {
		tempFloat, _ := strconv.ParseFloat(strings.TrimSpace(ms5611T.(string)), 64)
		graphics.displayValues["temperature"] = fmt.Sprintf("%.2f", tempFloat)
	}

	// Calculate m/s^2
	adxX, xOk := graphics.measurments.Load("ADX_ACCEL_X")
	adxY, yOk := graphics.measurments.Load("ADX_ACCEL_Y")
	adxZ, zOk := graphics.measurments.Load("ADX_ACCEL_Z")
	if xOk && yOk && zOk {
		xFloat, _ := strconv.ParseFloat(strings.TrimSpace(adxX.(string)), 64)
		yFloat, _ := strconv.ParseFloat(strings.TrimSpace(adxY.(string)), 64)
		zFloat, _ := strconv.ParseFloat(strings.TrimSpace(adxZ.(string)), 64)
		acceleration := math.Sqrt(math.Pow(xFloat, 2) + math.Pow(yFloat, 2) + math.Pow(zFloat, 2))
		graphics.displayValues["acceleration"] = fmt.Sprintf("%.1f", acceleration)
	}

	return nil
}

func (graphics *Window) drawLeft(screen *ebiten.Image) {
	leftSec := ebiten.NewImage(SIDE_WIDTH, SCREEN_HEIGHT)
	// Draw background
	vector.DrawFilledRect(leftSec, 0, 0, SIDE_WIDTH, SCREEN_HEIGHT, color.Black, false)

	// VBat
	val, ok := graphics.displayValues["vbat"]
	if ok {
		voltage := fmt.Sprintf("VBAT: %3sV", val)
		voltOp := &text.DrawOptions{}
		width, _ := text.Measure(voltage, &text.GoTextFace{
			Source: robotoFontSource,
			Size:   24,
		}, 5)
		voltOp.GeoM.Translate((SIDE_WIDTH/2)-(width/2), 20)
		voltOp.ColorScale.ScaleWithColor(color.White)
		text.Draw(leftSec, voltage, &text.GoTextFace{
			Source: robotoFontSource,
			Size:   24,
		}, voltOp)
	}

	// Temperature
	val, ok = graphics.displayValues["temperature"]
	if ok {
		temp := fmt.Sprintf("Temperature: %6s°C", val)
		tempOp := &text.DrawOptions{}
		width, _ := text.Measure(temp, &text.GoTextFace{
			Source: robotoFontSource,
			Size:   24,
		}, 5)
		tempOp.GeoM.Translate((SIDE_WIDTH/2)-(width/2), 60)
		tempOp.ColorScale.ScaleWithColor(color.White)
		text.Draw(leftSec, temp, &text.GoTextFace{
			Source: robotoFontSource,
			Size:   24,
		}, tempOp)
	}

	// Acceleration
	val, ok = graphics.displayValues["acceleration"]
	if ok {
		acceleration := fmt.Sprintf("Acceleration: %5sm/s^2", val)
		accelOp := &text.DrawOptions{}
		width, _ := text.Measure(acceleration, &text.GoTextFace{
			Source: robotoFontSource,
			Size:   24,
		}, 5)
		accelOp.GeoM.Translate((SIDE_WIDTH/2)-(width/2), 100)
		accelOp.ColorScale.ScaleWithColor(color.White)
		text.Draw(leftSec, acceleration, &text.GoTextFace{
			Source: robotoFontSource,
			Size:   24,
		}, accelOp)
	}

	// Draw section to overlay
	leftOp := &ebiten.DrawImageOptions{}
	leftOp.GeoM.Translate(SCREEN_WIDTH/2-(MIDDLE_WIDTH/2+SIDE_WIDTH), 0)
	screen.DrawImage(leftSec, leftOp)
}

func (graphics *Window) drawMiddle(screen *ebiten.Image) {
	middleSec := ebiten.NewImage(MIDDLE_WIDTH, SCREEN_HEIGHT)
	// Draw background
	vector.DrawFilledRect(middleSec, 0, 0, MIDDLE_WIDTH, SCREEN_HEIGHT, color.Black, false)

	// Draw altittude
	val, ok := graphics.displayValues["altitude"]
	if ok {
		altitude := fmt.Sprintf("Altitude: %5s ft", val)
		altOp := &text.DrawOptions{}
		width, _ := text.Measure(altitude, &text.GoTextFace{
			Source: robotoFontSource,
			Size:   24,
		}, 5)
		altOp.GeoM.Translate((MIDDLE_WIDTH/2)-(width/2), 20)
		altOp.ColorScale.ScaleWithColor(color.White)
		text.Draw(middleSec, altitude, &text.GoTextFace{
			Source: robotoFontSource,
			Size:   24,
		}, altOp)
	}

	// TODO: Spinny gyro rocket orientation thing

	// TODO: Speed

	// Draw setion to overlay
	middleOp := &ebiten.DrawImageOptions{}
	middleOp.GeoM.Translate(SCREEN_WIDTH/2-(MIDDLE_WIDTH/2), 0)
	screen.DrawImage(middleSec, middleOp)
}

func (graphics *Window) drawRight(screen *ebiten.Image) {
	rightSec := ebiten.NewImage(SIDE_WIDTH, SCREEN_HEIGHT)
	// Draw background
	vector.DrawFilledRect(rightSec, 0, 0, SIDE_WIDTH, SCREEN_HEIGHT, color.Black, false)

	// TODO: State stuff

	// Draw section to overlay
	rightOp := &ebiten.DrawImageOptions{}
	rightOp.GeoM.Translate(SCREEN_WIDTH/2+(MIDDLE_WIDTH/2), 0)
	screen.DrawImage(rightSec, rightOp)
}

func (graphics *Window) Draw(screen *ebiten.Image) {
	screen.Fill(color.RGBA{0, 255, 0, 100})

	graphics.drawLeft(screen)
	graphics.drawMiddle(screen)
	graphics.drawRight(screen)
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

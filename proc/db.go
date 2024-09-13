package proc

import (
	"fmt"
	"github.com/AarC10/GSW-V2/lib/db"
	"github.com/AarC10/GSW-V2/lib/tlm"
	"time"
)

func DatabaseWriter(handler db.Handler, packet TelemetryPacket, channel chan []byte) {
	measGroup := initMeasurementGroup(packet)

	for {
		data := <-channel
		updateMeasurementGroup(packet, measGroup, data)
		if handler.Insert(measGroup) != nil {
			fmt.Printf("Error storing packet results in database.")
		}
	}
}

func initMeasurementGroup(packet TelemetryPacket) db.MeasurementGroup {
	measurements := make([]db.Measurement, len(packet.Measurements))
	measurementGroup := db.MeasurementGroup{Measurements: measurements}

	for i, measurementName := range packet.Measurements {
		measurements[i].Name = measurementName
	}

	return measurementGroup
}

func updateMeasurementGroup(packet TelemetryPacket, measurements db.MeasurementGroup, data []byte) {
	offset := 0

	measurements.Timestamp = time.Now().UnixNano()
	for i, measurementName := range packet.Measurements {
		measurement, ok := GswConfig.Measurements[measurementName]
		if !ok {
			fmt.Printf("\t\tMeasurement '%s' not found\n", measurementName)
			continue
		}

		measurements.Measurements[i].Value = tlm.InterpretMeasurementValueString(measurement, data[offset:offset+measurement.Size])
		offset += measurement.Size
	}
}

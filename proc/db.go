package proc

import (
	"github.com/AarC10/GSW-V2/lib/db"
	"github.com/AarC10/GSW-V2/lib/tlm"
	"time"
)

func DatabaseWriter(handler db.Handler, chan []TelemetryPacket) {
	handler.Initialize()

	defer handler.Close()
}

func TelemetryPacketToMeasGroup(packet TelemetryPacket, data []byte) db.MeasurementGroup {
	measurements := make([]db.Measurement, len(packet.Measurements))
	offset := 0

	for i, measurementName := range packet.Measurements {
		measurement := GswConfig.Measurements[measurementName]
		valStr := tlm.InterpretMeasurementValueString(measurement, data[offset:offset+measurement.Size])

		measurements[i] = db.Measurement{packet.Name, valStr}
	}

	// Get unix timestamp
	return db.MeasurementGroup{time.Now().UnixNano(), measurements}
}

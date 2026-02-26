package db

// MeasurementGroup is a group of measurements to be sent to the database
type MeasurementGroup struct {
	DatabaseName string        // Maps to InfluxDB v2 measurement name
	Timestamp    int64         // Unix timestamp in nanoseconds
	Measurements []Measurement // List of measurements to be sent
}

// Measurement is a single measurement to be sent to the database
type Measurement struct {
	Name  string
	Value string
}

// Config holds all configuration needed to initialize a DB handler
type Config struct {
	URL    string
	Token  string
	Org    string
	Bucket string

	BatchSize     uint // Points to buffer before auto-flush (default: 100)
	FlushInterval uint // Max ms before flushing a partial batch (default: 1000)
}

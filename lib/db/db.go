package db

// Handler is an interface for database access implementations
type Handler interface {
	Initialize(host string, port int) error
	Insert(measurements MeasurementGroup) error
	CreateQuery(measurements MeasurementGroup) string
	Close() error
}

// BatchHandler extends Handler with batch write support
type BatchHandler interface {
	Handler
	Flush() error
}

// Config holds all configuration needed to initialize any Handler
// V1 only uses Host/Port. V2 requires URL, Token, Org, Bucket.
type Config struct {
	// V1
	Host string
	Port int

	// V2
	URL    string
	Token  string
	Org    string
	Bucket string

	// Batching (V2 only)
	BatchSize     uint   // Points to buffer before flushing
	FlushInterval uint   // Max ms before flushing partial batch
	Precision     string // "ns", "us", "ms", "s"
}

// MeasurementGroup is a group of measurements to be sent to the database
type MeasurementGroup struct {
	DatabaseName string
	Timestamp    int64
	Measurements []Measurement
}

// Measurement is a single measurement to be sent to the database
type Measurement struct {
	Name  string // Name of the measurement
	Value string // Value of the measurement
}

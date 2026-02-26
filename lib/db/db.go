package db

// Handler is an interface for database access implementations
type Handler interface {
	// Initialize sets up the database .
	Initialize(host string, port int) error
	// Insert sends the measurement data to the database.
	Insert(measurements MeasurementGroup) error
	// CreateQuery generates the database query for measurementGroup.
	CreateQuery(measurements MeasurementGroup) string
	// Close closes the database client when done.
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
	DatabaseName string        // Name of the database
	Timestamp    int64         // Unix timestamp in nanoseconds
	Measurements []Measurement // List of measurements to be sent
}

// Measurement is a single measurement to be sent to the database
type Measurement struct {
	Name  string // Name of the measurement
	Value string // Value of the measurement
}

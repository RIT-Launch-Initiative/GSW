package db

// Handler is an interface for database access implementations
type Handler interface {
	// Initialize sets up the database client
	Initialize() error
	// Insert sends the measurement data to the database
	Insert(measurements MeasurementGroup) error
	// CreateQuery generates the database query for measurementGroup
	CreateQuery(measurements MeasurementGroup) string
	// Close closes the database client when done
	Close() error
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

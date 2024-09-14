package db

type Handler interface {
	Initialize() error
	Insert(measurements MeasurementGroup) error
	CreateQuery(measurements MeasurementGroup) string
	Close() error
}

type MeasurementGroup struct {
	DatabaseName string
	Timestamp    int64
	Measurements []Measurement
}

type Measurement struct {
	Name  string
	Value string
}

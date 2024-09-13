package db

type Handler interface {
	Initialize() error
	Insert(measurements MeasurementGroup) error
	CreateQuery(measurements MeasurementGroup) string
	Close()
}

type MeasurementGroup struct {
	Timestamp    int64
	Measurements []Measurement
}

type Measurement struct {
	Name  string
	Value string
}

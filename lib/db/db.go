package db

type Handler interface {
	Initialize()
	Insert(measurements MeasurementGroup) error
	CreateQuery(measurements MeasurementGroup) string
	Close()
}

type MeasurementGroup struct {
	timestamp    int64
	Measurements []Measurement
}

type Measurement struct {
	Name  string
	Value string
}

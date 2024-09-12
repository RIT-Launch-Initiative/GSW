package db

type Handler interface {
	Insert(measurements []Measurement) error
	CreateQuery(measurements []Measurement) string
}

type Measurement struct {
	Name  string
	Value string
}

package db

import (
	"fmt"
	"github.com/influxdata/influxdb-client-go/v2"
)

type InfluxDBHandler struct {
	client influxdb2.Client
	org    string
	bucket string
}

// Initialize sets up the InfluxDB client
func (h *InfluxDBHandler) Initialize() {
	// TODO: Get URL and token from config
	//h.client = influxdb2.NewClient(url, token)
}

// CreateQuery generates the InfluxDB line protocol query for measurementGroup
func (h *InfluxDBHandler) CreateQuery(measurementGroup MeasurementGroup) string {
	var query string

	for _, measurement := range measurementGroup.Measurements {
		query += fmt.Sprintf("%s,value=%s %d\n", measurement.Name, measurement.Value, measurementGroup.timestamp)
	}
	return query
}

// Insert sends the measurement data to InfluxDB
func (h *InfluxDBHandler) Insert(measurementGroup MeasurementGroup) error {
	// TODO: Implement
	//query := h.CreateQuery(measurementGroup)

	return nil
}

// Close closes the InfluxDB client when done
func (h *InfluxDBHandler) Close() {
	h.client.Close()
}

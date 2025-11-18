package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"text/template"
	"time"
)

type ReaderOutput struct {
	TotalPacketsLost     uint64
	TotalPacketsReceived uint64
	Runtime              time.Duration
	Packets              []OutputPacket
}

type OutputPacket struct {
	Name     string
	Size     uint64
	Lost     uint64
	Received uint64
}

type outputFormatType int

const (
	outputFormatTypePrettyPrint outputFormatType = iota
	outputFormatTypeJSON
	outputFormatTypeTemplate
)

type outputFormatFlagValue struct {
	formatType     outputFormatType
	templateString *string
	template       *template.Template
}

func (f *outputFormatFlagValue) String() string {
	switch f.formatType {
	case outputFormatTypeJSON:
		return "json"
	case outputFormatTypeTemplate:
		return fmt.Sprintf("template: %s", *f.templateString)
	case outputFormatTypePrettyPrint:
		return "pretty print"
	default:
		return "invalid output format"
	}
}

func (f *outputFormatFlagValue) Set(s string) error {
	if s == "json" {
		f.formatType = outputFormatTypeJSON
		return nil
	} else {
		tmpl, err := template.New("output_format").Parse(s)
		if err != nil {
			return fmt.Errorf("couldn't parse output format as template: %w", err)
		}
		f.template = tmpl
		f.templateString = &s
		f.formatType = outputFormatTypeTemplate
		return nil
	}
}

func (o *ReaderOutput) PrettyPrint() string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Total runtime: %s\n", o.Runtime))
	sb.WriteString(fmt.Sprintf("Total packets received: %d (%d/s)\n", o.TotalPacketsReceived, o.TotalPacketsReceived/uint64(o.Runtime.Seconds())))
	sb.WriteString(fmt.Sprintf("Total packets lost: %d (%.3f%%)\n", o.TotalPacketsLost, (float64(o.TotalPacketsReceived)/float64(o.TotalPacketsLost+o.TotalPacketsReceived))*100))
	for _, p := range o.Packets {
		sb.WriteString(fmt.Sprintf("[%s] (%d B):\n", p.Name, p.Size))
		sb.WriteString(fmt.Sprintf("\t[%s] Packets received: %d (%d/s)\n", p.Name, p.Received, p.Received/uint64(o.Runtime.Seconds())))
		sb.WriteString(fmt.Sprintf("\t[%s] Packets lost: %d (%.3f%%)\n", p.Name, p.Lost, (float64(p.Received)/float64(p.Received+p.Lost))*100))
	}
	return sb.String()
}

func (o *ReaderOutput) JSON() (string, error) {
	prettyJson, err := json.MarshalIndent(o, "", "\t")
	if err != nil {
		return "", fmt.Errorf("generating json output: %w", err)
	}

	return string(prettyJson), nil
}

func (o *ReaderOutput) Template(tmpl *template.Template) (string, error) {
	var buf bytes.Buffer
	err := tmpl.Execute(&buf, o)
	if err != nil {
		return "", fmt.Errorf("generating template output: %w", err)
	}
	return buf.String(), nil
}

func (f *outputFormatFlagValue) GenerateReaderOutput(output ReaderOutput) (string, error) {
	switch f.formatType {
	case outputFormatTypeJSON:
		return output.JSON()
	case outputFormatTypePrettyPrint:
		return output.PrettyPrint(), nil
	case outputFormatTypeTemplate:
		return output.Template(f.template)
	default:
		return "", fmt.Errorf("unexpected outputFormatType: %#v", f.formatType)
	}
}

package db

import (
	"testing"
	"time"
)

func TestParsePrecision(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    Precision
		wantErr bool
	}{
		{name: "default empty", input: "", want: PrecisionNS},
		{name: "ns", input: "ns", want: PrecisionNS},
		{name: "uppercase and spaces", input: "  MS  ", want: PrecisionMS},
		{name: "us", input: "us", want: PrecisionUS},
		{name: "s", input: "s", want: PrecisionS},
		{name: "invalid", input: "minutes", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParsePrecision(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestPrecisionDuration(t *testing.T) {
	tests := []struct {
		precision Precision
		want      time.Duration
		wantErr   bool
	}{
		{precision: PrecisionNS, want: time.Nanosecond},
		{precision: PrecisionUS, want: time.Microsecond},
		{precision: PrecisionMS, want: time.Millisecond},
		{precision: PrecisionS, want: time.Second},
		{precision: Precision("bogus"), wantErr: true},
	}

	for _, tt := range tests {
		got, err := tt.precision.Duration()
		if tt.wantErr {
			if err == nil {
				t.Fatalf("expected error for precision %q", tt.precision)
			}
			continue
		}
		if err != nil {
			t.Fatalf("unexpected error for precision %q: %v", tt.precision, err)
		}
		if got != tt.want {
			t.Fatalf("got %v, want %v", got, tt.want)
		}
	}
}

package main

import (
	"strings"
	"testing"

	"github.com/AarC10/GSW-V2/lib/db"
	"github.com/spf13/viper"
)

func newTestConfig() *viper.Viper {
	cfg := viper.New()
	cfg.SetEnvPrefix("GSW")
	cfg.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	cfg.AutomaticEnv()
	return cfg
}

func TestResolveDBConfigFromEnvironmentUsesV2(t *testing.T) {
	t.Setenv("GSW_DATABASE_V2_URL", "http://influxdb:8086")
	t.Setenv("GSW_DATABASE_V2_TOKEN", "test-token")
	t.Setenv("GSW_DATABASE_V2_ORG", "gsw")
	t.Setenv("GSW_DATABASE_V2_BUCKET", "gsw")
	t.Setenv("GSW_DATABASE_V2_BATCH_SIZE", "250")
	t.Setenv("GSW_DATABASE_V2_FLUSH_INTERVAL_MS", "1500")
	t.Setenv("GSW_DATABASE_V2_PRECISION", "ms")

	cfg := newTestConfig()

	got, err := resolveDBConfig(cfg)
	if err != nil {
		t.Fatalf("resolveDBConfig returned error: %v", err)
	}
	if got.v2 == nil {
		t.Fatal("expected v2 config to be selected")
	}
	if got.v1 != nil {
		t.Fatal("expected v1 config to remain unset")
	}

	want := db.InfluxDBV2Config{
		URL:           "http://influxdb:8086",
		Token:         "test-token",
		Org:           "gsw",
		Bucket:        "gsw",
		BatchSize:     250,
		FlushInterval: 1500,
		Precision:     db.PrecisionMS,
	}
	if *got.v2 != want {
		t.Fatalf("got %+v, want %+v", *got.v2, want)
	}
}

func TestResolveDBConfigPrefersV2OverV1(t *testing.T) {
	cfg := viper.New()
	cfg.Set("database_host_name", "legacy")
	cfg.Set("database_port_number", 8089)
	cfg.Set("database_v2.url", "http://influxdb:8086")
	cfg.Set("database_v2.token", "token")
	cfg.Set("database_v2.org", "gsw")
	cfg.Set("database_v2.bucket", "gsw")

	got, err := resolveDBConfig(cfg)
	if err != nil {
		t.Fatalf("resolveDBConfig returned error: %v", err)
	}
	if got.v2 == nil {
		t.Fatal("expected v2 config to be selected")
	}
	if got.v1 != nil {
		t.Fatal("expected v1 config to remain unset when v2 is configured")
	}
}

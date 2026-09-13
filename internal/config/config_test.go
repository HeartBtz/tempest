package config

import (
	"reflect"
	"testing"
)

func TestAllowedHostsEnvironmentOverride(t *testing.T) {
	t.Setenv("TEMPEST_ALLOWED_HOSTS", " Tempest.Example.,10.0.0.5,tempest.example,localhost ")
	cfg := Default()
	applyEnvOverrides(cfg)
	want := []string{"tempest.example", "10.0.0.5", "localhost"}
	if !reflect.DeepEqual(cfg.Security.AllowedHosts, want) {
		t.Fatalf("allowed hosts = %#v, want %#v", cfg.Security.AllowedHosts, want)
	}
}

func TestAllowedHostsEnvironmentDropsUnsafeValues(t *testing.T) {
	t.Setenv("TEMPEST_ALLOWED_HOSTS", "*,https://example.com,example.com:8377,0.0.0.0,[::],bad host,-bad.example,good.example")
	cfg := Default()
	applyEnvOverrides(cfg)
	want := []string{"good.example"}
	if !reflect.DeepEqual(cfg.Security.AllowedHosts, want) {
		t.Fatalf("allowed hosts = %#v, want %#v", cfg.Security.AllowedHosts, want)
	}
}

func TestEmptyAllowedHostsEnvironmentClearsFileValues(t *testing.T) {
	t.Setenv("TEMPEST_ALLOWED_HOSTS", "")
	cfg := Default()
	cfg.Security.AllowedHosts = []string{"from-file.example"}
	applyEnvOverrides(cfg)
	if len(cfg.Security.AllowedHosts) != 0 {
		t.Fatalf("allowed hosts = %#v, want empty override", cfg.Security.AllowedHosts)
	}
}

func TestAllowedOriginsEnvironmentOverride(t *testing.T) {
	t.Setenv("TEMPEST_ALLOWED_ORIGINS", " HTTPS://Tempest.Example.:443,https://tempest.example,http://127.0.0.1:8377,https://[::1]:8443 ")
	cfg := Default()
	applyEnvOverrides(cfg)
	want := []string{"https://tempest.example", "http://127.0.0.1:8377", "https://[::1]:8443"}
	if !reflect.DeepEqual(cfg.Security.AllowedOrigins, want) {
		t.Fatalf("allowed origins = %#v, want %#v", cfg.Security.AllowedOrigins, want)
	}
}

func TestAllowedOriginsEnvironmentDropsUnsafeValues(t *testing.T) {
	t.Setenv("TEMPEST_ALLOWED_ORIGINS", "*,example.com,ftp://example.com,https://user@example.com,https://example.com/path,https://example.com?x=1,https://0.0.0.0,https://example.com:0443,https://good.example")
	cfg := Default()
	applyEnvOverrides(cfg)
	want := []string{"https://good.example"}
	if !reflect.DeepEqual(cfg.Security.AllowedOrigins, want) {
		t.Fatalf("allowed origins = %#v, want %#v", cfg.Security.AllowedOrigins, want)
	}
}

func TestEmptyAllowedOriginsEnvironmentClearsFileValues(t *testing.T) {
	t.Setenv("TEMPEST_ALLOWED_ORIGINS", "")
	cfg := Default()
	cfg.Security.AllowedOrigins = []string{"https://from-file.example"}
	applyEnvOverrides(cfg)
	if len(cfg.Security.AllowedOrigins) != 0 {
		t.Fatalf("allowed origins = %#v, want empty override", cfg.Security.AllowedOrigins)
	}
}

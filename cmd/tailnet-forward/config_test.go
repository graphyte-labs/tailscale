package main

import (
	"strings"
	"testing"
)

func TestParseForward(t *testing.T) {
	tests := []struct {
		value   string
		want    forward
		wantErr string
	}{
		{value: "5432:192.0.2.10:5432", want: forward{listenPort: 5432, host: "192.0.2.10", port: 5432, target: "192.0.2.10:5432"}},
		{value: "8080:[fd7a::1]:80", want: forward{listenPort: 8080, host: "fd7a::1", port: 80, target: "[fd7a::1]:80"}},
		{value: "5432", wantErr: "missing target"},
		{value: "5432:192.0.2.10", wantErr: "target"},
		{value: "x:192.0.2.10:5432", wantErr: "listen port"},
		{value: "0:192.0.2.10:5432", wantErr: "listen port"},
		{value: "70000:192.0.2.10:5432", wantErr: "listen port"},
		{value: "5432:192.0.2.10:y", wantErr: "target port"},
		{value: "5432::5432", wantErr: "empty target host"},
	}
	for _, tt := range tests {
		got, err := parseForward("TCP_FORWARD_1", tt.value)
		if tt.wantErr != "" {
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("parseForward(%q) error = %v, want it to mention %q", tt.value, err, tt.wantErr)
			}
			continue
		}
		if err != nil {
			t.Errorf("parseForward(%q): %v", tt.value, err)
			continue
		}
		tt.want.name = "TCP_FORWARD_1"
		if got != tt.want {
			t.Errorf("parseForward(%q) = %+v, want %+v", tt.value, got, tt.want)
		}
	}
}

func TestLoadForwards(t *testing.T) {
	forwards, err := loadForwards([]string{
		"TCP_FORWARD_2=6379:192.0.2.20:6379",
		"TCP_FORWARD_1=5432:192.0.2.10:5432",
		"TS_SERVE_CONFIG=/tmp/serve.json",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(forwards) != 2 || forwards[0].name != "TCP_FORWARD_1" {
		t.Errorf("forwards not sorted by name: %+v", forwards)
	}
}

func TestLoadForwardsReportsAllErrors(t *testing.T) {
	_, err := loadForwards([]string{
		"TCP_FORWARD_1=5432:192.0.2.10:5432",
		"TCP_FORWARD_2=5432:192.0.2.11:5432",
		"TCP_FORWARD_3=bad",
	})
	if err == nil {
		t.Fatal("want an error")
	}
	for _, want := range []string{"already used by TCP_FORWARD_1", "TCP_FORWARD_3"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not mention %q", err, want)
		}
	}
}

func TestLoadForwardsEmpty(t *testing.T) {
	forwards, err := loadForwards(nil)
	if err != nil || len(forwards) != 0 {
		t.Errorf("empty env: forwards=%+v err=%v", forwards, err)
	}
}

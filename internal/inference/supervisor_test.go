package inference

import (
	"testing"
)

func TestLoopbackHost(t *testing.T) {
	tests := []struct {
		host string
		want bool
	}{
		{"127.0.0.1", true},
		{"::1", true},
		{"localhost", true},
		{"0.0.0.0", false},
		{"192.168.1.10", false},
		{"example.com", false},
	}
	for _, tt := range tests {
		t.Run(tt.host, func(t *testing.T) {
			if got := isLoopbackHost(tt.host); got != tt.want {
				t.Fatalf("isLoopbackHost(%q) = %v, want %v", tt.host, got, tt.want)
			}
		})
	}
}

func TestEndpointIPv6(t *testing.T) {
	s := NewSupervisor(Config{Host: "::1", Port: 8081})
	if got, want := s.Endpoint(), "http://[::1]:8081"; got != want {
		t.Fatalf("Endpoint() = %q, want %q", got, want)
	}
}

func TestSupervisorDefaults(t *testing.T) {
	s := NewSupervisor(Config{})
	if s.cfg.Host != "127.0.0.1" || s.cfg.Port != 8081 || s.cfg.Threads != 2 || s.cfg.ContextSize != 2048 {
		t.Fatalf("unexpected defaults: %+v", s.cfg)
	}
}

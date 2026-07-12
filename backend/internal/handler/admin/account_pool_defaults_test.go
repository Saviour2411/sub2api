package admin

import (
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

func TestApplyOpenAIAPIKeyPoolModeDefaults(t *testing.T) {
	creds := service.ApplyGatewayPoolModeDefaults("apikey", map[string]any{"api_key": "sk-test"}, service.DefaultGatewaySettings())

	if creds["pool_mode"] != true {
		t.Fatalf("pool_mode = %#v, want true", creds["pool_mode"])
	}
	if creds["pool_mode_retry_count"] != 1 {
		t.Fatalf("pool_mode_retry_count = %#v, want 1", creds["pool_mode_retry_count"])
	}
	codes, ok := creds["pool_mode_retry_status_codes"].([]int)
	if !ok {
		t.Fatalf("pool_mode_retry_status_codes type = %T, want []int", creds["pool_mode_retry_status_codes"])
	}
	want := []int{401, 403, 429, 502, 503, 504}
	if len(codes) != len(want) {
		t.Fatalf("pool_mode_retry_status_codes = %#v, want %#v", codes, want)
	}
	for i := range want {
		if codes[i] != want[i] {
			t.Fatalf("pool_mode_retry_status_codes = %#v, want %#v", codes, want)
		}
	}
}

func TestApplyOpenAIAPIKeyPoolModeDefaults_ExplicitFalseNotOverridden(t *testing.T) {
	creds := service.ApplyGatewayPoolModeDefaults("apikey", map[string]any{"pool_mode": false}, service.DefaultGatewaySettings())

	if creds["pool_mode"] != false {
		t.Fatalf("pool_mode = %#v, want false", creds["pool_mode"])
	}
	if _, ok := creds["pool_mode_retry_count"]; ok {
		t.Fatal("did not expect retry count to be added when pool mode is explicitly false")
	}
}

package mcpgateway

import (
	"encoding/json"
	"fmt"
	"os"
	"testing"
)

func newBenchGateway(b *testing.B, auth AuthMethod, tokens []string, rpmLimit int) *Gateway {
	b.Helper()
	dir, err := os.MkdirTemp("", "mcpgw-bench-*")
	if err != nil {
		b.Fatal(err)
	}
	b.Cleanup(func() { os.RemoveAll(dir) })

	cfg := GatewayConfig{
		Name:    "bench",
		Version: "1.0",
		Enabled: true,
		Auth:    AuthConfig{Method: auth, Tokens: tokens},
		RateLimit: RateLimitConfig{
			RequestsPerMinute: rpmLimit,
			Burst:             rpmLimit / 10,
		},
		Validation: []ValidationRule{
			{
				ToolName:    "run_code",
				Required:    []string{"language", "code"},
				AllowedArgs: []string{"language", "code", "timeout"},
				MaxPayload:  65536,
			},
		},
	}
	gw, err := NewGateway(dir, cfg)
	if err != nil {
		b.Fatal(err)
	}
	return gw
}

// BenchmarkProcessRequest_AuthNone measures the gateway hot-path with no authentication.
func BenchmarkProcessRequest_AuthNone(b *testing.B) {
	gw := newBenchGateway(b, AuthNone, nil, 100000)
	req := GatewayRequest{
		ClientID:   "client-1",
		RemoteAddr: "127.0.0.1:9000",
		Method:     "tools/call",
		ToolName:   "run_code",
		Args:       map[string]interface{}{"language": "python", "code": "print('hi')"},
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		gw.ProcessRequest(req)
	}
}

// BenchmarkProcessRequest_TokenAuth measures request processing with token authentication.
func BenchmarkProcessRequest_TokenAuth(b *testing.B) {
	tokens := make([]string, 10)
	for i := range tokens {
		tokens[i] = fmt.Sprintf("token-%04d", i)
	}
	gw := newBenchGateway(b, AuthToken, tokens, 100000)
	req := GatewayRequest{
		ClientID:   "client-1",
		RemoteAddr: "127.0.0.1:9000",
		Token:      tokens[9], // worst-case: last token in slice
		Method:     "tools/call",
		ToolName:   "run_code",
		Args:       map[string]interface{}{"language": "python", "code": "print('hi')"},
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		gw.ProcessRequest(req)
	}
}

// BenchmarkProcessRequest_AuthFail measures the cost of a rejected request.
func BenchmarkProcessRequest_AuthFail(b *testing.B) {
	gw := newBenchGateway(b, AuthToken, []string{"valid-token"}, 100000)
	req := GatewayRequest{
		ClientID:   "attacker",
		RemoteAddr: "10.0.0.1:1234",
		Token:      "wrong-token",
		Method:     "tools/call",
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		gw.ProcessRequest(req)
	}
}

// BenchmarkProcessRequest_Parallel measures concurrency under parallel load.
func BenchmarkProcessRequest_Parallel(b *testing.B) {
	gw := newBenchGateway(b, AuthNone, nil, 100000000)
	req := GatewayRequest{
		ClientID:   "client-parallel",
		RemoteAddr: "127.0.0.1:9000",
		Method:     "tools/call",
		ToolName:   "run_code",
		Args:       map[string]interface{}{"language": "go", "code": "fmt.Println()"},
	}
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			gw.ProcessRequest(req)
		}
	})
}

// BenchmarkProcessRequest_MultiClient measures per-client rate-limit map overhead.
func BenchmarkProcessRequest_MultiClient(b *testing.B) {
	gw := newBenchGateway(b, AuthNone, nil, 100000)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req := GatewayRequest{
			ClientID:   fmt.Sprintf("client-%d", i%1000),
			RemoteAddr: "127.0.0.1:9000",
			Method:     "tools/list",
		}
		gw.ProcessRequest(req)
	}
}

// BenchmarkGetAudit measures audit log filtering performance.
func BenchmarkGetAudit(b *testing.B) {
	gw := newBenchGateway(b, AuthNone, nil, 100000)
	// Pre-populate the audit log
	req := GatewayRequest{
		ClientID: "client-audit",
		Method:   "tools/call",
	}
	for i := 0; i < 500; i++ {
		gw.ProcessRequest(req)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		gw.GetAudit("client-audit", "ok", 50)
	}
}

// BenchmarkStats measures aggregate stats computation.
func BenchmarkStats(b *testing.B) {
	gw := newBenchGateway(b, AuthNone, nil, 100000)
	for i := 0; i < 200; i++ {
		gw.ProcessRequest(GatewayRequest{
			ClientID: fmt.Sprintf("c%d", i%20),
			Method:   "tools/call",
		})
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		gw.Stats()
	}
}

// BenchmarkAuthenticate_TokenSlice measures token-slice linear-scan cost.
func BenchmarkAuthenticate_TokenSlice(b *testing.B) {
	sizes := []int{1, 10, 100}
	for _, n := range sizes {
		tokens := make([]string, n)
		for i := range tokens {
			tokens[i] = fmt.Sprintf("tok-%d", i)
		}
		gw := newBenchGateway(b, AuthToken, tokens, 100000)
		req := GatewayRequest{Token: tokens[n-1], ClientID: "c", Method: "m"} // worst-case
		b.Run(fmt.Sprintf("tokens=%d", n), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				gw.authenticate(req) //nolint:errcheck
			}
		})
	}
}

// BenchmarkValidate measures schema validation hot-path.
func BenchmarkValidate(b *testing.B) {
	gw := newBenchGateway(b, AuthNone, nil, 0)
	payload, _ := json.Marshal(map[string]string{"language": "python", "code": "x=1"})
	req := GatewayRequest{
		ClientID: "c",
		Method:   "tools/call",
		ToolName: "run_code",
		Args:     map[string]interface{}{"language": "python", "code": "x=1"},
		Payload:  payload,
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		gw.validate(req) //nolint:errcheck
	}
}

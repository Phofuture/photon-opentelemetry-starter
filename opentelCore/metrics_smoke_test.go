package opentelCore

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"
)

// 驗證雙軌 metric：Prometheus /metrics 端點可拉取、自訂 counter 有出現、shutdown 會關閉 server。
func TestPrometheusPullSmoke(t *testing.T) {
	// 找一個空閒 port
	ln, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Fatal(err)
	}
	port := fmt.Sprintf("%d", ln.Addr().(*net.TCPAddr).Port)
	_ = ln.Close()

	path := "/metrics"
	config := &Config{}
	config.OpenTel.ServiceName = "smoke-test"
	config.OpenTel.MetricPort = &port
	config.OpenTel.MetricPath = &path
	serviceName = config.OpenTel.ServiceName

	// 不 dial collector（OTLP 那軌走 http endpoint 到不存在的位址也不影響 pull 軌）
	collectorUseGRPC = false
	collectorEndpoint = "http://127.0.0.1:1" // 不可達，僅驗證失敗不影響 prometheus 軌

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	startMetricService(ctx, config)

	if provider == nil {
		t.Fatal("provider not initialized")
	}
	AddMeterCount("smoke_total", "smoke test counter", "1", 7)

	// 等 server 起來後抓 /metrics
	var body string
	for i := 0; i < 20; i++ {
		time.Sleep(100 * time.Millisecond)
		resp, err := http.Get("http://127.0.0.1:" + port + path)
		if err != nil {
			continue
		}
		b, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		body = string(b)
		break
	}
	if body == "" {
		t.Fatal("failed to scrape /metrics")
	}
	// otel prometheus exporter 的命名轉換：smoke_total + unit "1" → smoke_ratio_total（與 v1.x 相同）
	if !strings.Contains(body, "smoke_ratio_total") {
		t.Fatalf("custom counter not found in exposition:\n%s", body[:min(len(body), 2000)])
	}
	t.Logf("scrape ok, custom counter present; exposition size=%d bytes", len(body))

	if err := Shutdown(context.Background()); err != nil {
		t.Fatal(err)
	}
	// server 應已關閉
	if _, err := http.Get("http://127.0.0.1:" + port + path); err == nil {
		t.Fatal("metric server still serving after shutdown")
	}
}

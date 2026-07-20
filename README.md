# photon-opentelemetry-starter

Photon 共用 OpenTelemetry starter：以 blank import 自動註冊 trace / metric / log 初始化。

```go
import _ "github.com/Phofuture/photon-opentelemetry-starter"
```

## 設定（config）

```yaml
openTel:
  url: grpc://otel-collector:4317   # 必填。grpc:// → 三 signal 共用一條 grpc conn；否則走 http endpoint
  serviceName: my-service           # 必填
  environment: uat                  # 選填 → resource.deployment.environment（dev / uat / prod）
  serviceVersion: 1.2.3             # 選填 → resource.service.version（建議帶 git commit / 版號）
  sampleRatio: 0.1                  # 選填 → trace 取樣率 0.0~1.0；未設維持全採
  metricPort: "9090"                # 選填 → 有設才啟動 Prometheus /metrics 拉取端點（SRE 監控用）
  metricPath: /metrics              # 選填 → 拉取路徑，未設預設 /metrics
log:
  level: info
```

## 架構

```
                     ┌─ TracerProvider  (BatchSpanProcessor → otlptrace)
 collector conn ─────┼─ MeterProvider   (PeriodicReader     → otlpmetric)  ─┐ 同一個 provider
 (openTel.url 共用)   │                  (prometheus reader  → /metrics)    ─┘ 雙 reader
                     └─ LoggerProvider  (BatchLogProcessor  → otlplog)
   + 共用 Resource(service.name / service.version / deployment.environment)
```

- **trace / metric / log 全走 OTLP push** 到同一個 collector（`openTel.url`）。
- **Metric 雙軌並存**：OTLP push（SigNoz）之外，有設 `openTel.metricPort` 就同時啟動
  Prometheus `/metrics` 拉取端點。雙 reader 掛在同一個 MeterProvider，push / pull 資料一致；
  metric 名稱與 exposition 格式與 v1.0.x 相同，SRE 既有的 Prometheus job / dashboard 不用改。
- 關機時自動 flush 三個 signal 並關閉 `/metrics` 端點。

## v1.1.1 升級注意（自 v1.1.0 / v1.0.x）

- **Trace / log 改走 OTLP push**：log 不再每筆開一個 `name="log"` span，改送真正的 OTLP logs。
  ⚠️ 若下游有 dashboard / alert 建在舊 log-span 上會失效，需改用 SigNoz logs 檢索。
- **Metric 新增 OTLP push，Prometheus 拉取式保留**：`openTel.metricPort` / `metricPath`
  語意不變（有設 `metricPort` 才開 `/metrics`），未設定則只走 OTLP push。
- 下游靠 blank import 自動註冊，升級只需 bump 版本。

## Roadmap / 優化項

見 [docs/optimization-todo.md](docs/optimization-todo.md) —— trace/metric/log 三訊號的已知問題與優化 checklist。

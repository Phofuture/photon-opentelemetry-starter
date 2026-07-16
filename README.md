# photon-opentelemetry-starter

Photon 共用 OpenTelemetry starter：以 blank import 自動註冊 trace / metric / log 初始化。

## 設定（config）

```yaml
openTel:
  url: grpc://otel-collector:4317   # grpc:// → 三 signal 共用一條 grpc conn；否則走 http endpoint
  serviceName: my-service
  environment: uat                  # 選填 → resource.deployment.environment
  serviceVersion: 1.2.3             # 選填 → resource.service.version（建議帶 git commit / 版號）
  sampleRatio: 0.1                  # 選填 → trace 取樣率 0.0~1.0；未設維持全採
log:
  level: info
```

trace / metric / log 三個 signal 全部走 **OTLP push** 到同一個 collector（`url`），
starter 不再曝露 Prometheus `/metrics` 端點。

## v2.0.0 breaking change（自 v1.x 升級）

- **Metric 改 OTLP push，移除 Prometheus 拉取式**：不再有 `/metrics` HTTP 端點。
  依賴 scrape 的舊監控（Prometheus job / k8s ServiceMonitor）需改為由 collector 接收 OTLP。
- **`openTel.metricPort` / `openTel.metricPath` 作廢**：欄位保留（舊 yaml 不會解析失敗）但無作用，
  啟動會印 deprecation warning，請從 config 移除。
- **Log 改真正的 OTLP logs**：不再每筆 log 開一個 `name="log"` span。
  ⚠️ 若下游有 dashboard / alert 建在這些 log-span 上會失效，需改用 SigNoz logs 檢索。
- 下游靠 blank import `_ ".../photon-opentelemetry-starter"` 自動註冊，升級只需 bump 版本。

## Roadmap / 優化項

見 [docs/optimization-todo.md](docs/optimization-todo.md) —— trace/metric/log 三訊號的已知問題與優化 checklist。
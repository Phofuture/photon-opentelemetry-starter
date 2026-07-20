# photon-opentelemetry-starter 優化 TODO

> 最後更新：2026-07-16
> 對象版本：`v1.0.1`(main）→ ~~目標 `v2.0.0`（breaking）~~ → 定版 `v1.1.1`
> （2026-07-20：Prometheus 拉取式恢復並存後不再走 v2 major bump；
> 　module path 無 `/v2` 後綴，v2.0.0 tag 對 Go 工具鏈本為無效版本，僅存在本地、未推上 origin，已刪除）
>
> 起因：slot-game UAT 查找中發現「app 的 metric / log 在 SigNoz 上看不到，只有 trace 進得去」，
> 追到根因都在本 starter。
>
> 狀態：☐ 未開始　◐ 進行中　☑ 完成

---

## 已鎖定決策（2026-07-16，2026-07-20 修訂第 2、3 點）

1. **全面 OTLP push**：trace / metric / log 三個 signal 全走同一個 collector（OTLP grpc），starter 不再依賴外部 scrape。
2. ~~**Prometheus 拉取式：全砍**~~ → **改為與 OTLP push 雙軌並存**（2026-07-20，SRE 要求保留 scrape 監控）。
   同一個 MeterProvider 掛雙 reader：PeriodicReader（OTLP push → SigNoz）+ prometheus exporter（`/metrics` 拉取）。
   有設 `metricPort` 才開端點，語意同 v1.x；metric 名稱 / exposition 格式與 v1.x 相同，SRE 既有 dashboard 不受影響。
3. **config 相容**：`openTel.url` 沿用；`metricPort` / `metricPath` ~~作廢為 no-op~~ → **恢復有效**（見第 2 點）。
4. ~~**這是 breaking change**：bump `v2.0.0`~~ → **定版 `v1.1.1`**（2026-07-20：Prometheus 拉取式恢復、
   config 欄位語意不變後，對下游唯一的行為變化是 log-span → OTLP logs）。
   仍需通知所有下游（slot-game 等靠 blank import `_ ".../photon-opentelemetry-starter"` 自動註冊）。

---

## 目標架構

```
                     ┌─ TracerProvider  (BatchSpanProcessor  → otlptracegrpc)
 collector conn ─────┼─ MeterProvider   (PeriodicReader      → otlpmetricgrpc)
 (grpc://…, 共用)     └─ LoggerProvider  (BatchLogProcessor   → otlploggrpc)
   + 共用 Resource(service.name / service.version / deployment.environment / service.instance.id)
```

一條到 collector 的 grpc 連線 + 一份共用 Resource，餵給三個 signal。對外 API（`AddMeterCount`、logger 的 `Info/Error/...`）簽名全部保持不變，對下游透明。

---

## 執行順序（依此重排）

### ☑ 階段 0　地基：共用 grpc conn + 共用 Resource
（原 P1-6 併入）

- **位置**：`opentelCore/init.go`（`connectGrpc` 目前只給 trace 用）、resource 只放 `ServiceNameKey`。
- **做法**：抽出單一 `grpc.ClientConn`（沿用 `openTel.url` 的 `grpc://` 解析），供三個 exporter 共用；
  建一份共用 `Resource`：`service.name` + `service.version`（git commit / version.txt，下游 actuator 已在讀）
  + `deployment.environment`（dev/uat/prod）+ `service.instance.id`。皆低基數、高價值。
- **驗收**：三個 provider 都拿到同一 conn 與同一 resource；SigNoz 上 trace 帶 env + version。

### ☑ 階段 1　Metric 改 OTLP push（原 P2-7 + P2-8 + P0-4）

- **位置**：`opentelCore/meterics.go` `configureMetrics()`、`startMetricService()`；`shutdown.go`。
- **影響（現況）**：
  1. 只有 Prometheus 拉取，下游 SigNoz 沒 scrape → **app metric 全看不到**（查找痛點根源之一）。
  2. MeterProvider 無 resource → metric 不帶 service.name。
  3. `provider` 為 nil 時 `AddMeterCount` → `provider.Meter()` **nil-panic**（下游 slot-game 已被迫 `best_effort.go` 用 `recover()` 擋）。
- **做法**：
  - 砍 `prometheus.New()` + `promhttp` HTTP server。
  - `metric.NewMeterProvider(WithReader(PeriodicReader(otlpmetricgrpc)), WithResource(...))`。
  - `provider == nil` 時 `AddMeterCount` / `AddMeterUpDownCount` 走 no-op（不再把地雷丟給 caller）。
  - `shutdown.go` 補 `provider.Shutdown(ctx)` flush 尾端 metric。
  - `metricPort` / `metricPath` 作廢：保留欄位、印 deprecation warning。
- **驗收**：SigNoz `list_metrics` 撈得到 app metric、帶 `service.name`；metric 關閉時呼叫不 panic；關機 flush 完整。

### ☑ 階段 2　Log 改真正的 OTLP logs（原 P0-1 + P0-2 + P2-9 dead handler）

- **位置**：`opentelLog/log.go` `log()`（`NewTrace(ctx,"log")` + 塞 span 屬性）、`opentelLog/logHandler.go`（`Handle()` 回 nil 死碼）。
- **影響（現況）**：
  1. SigNoz 沒有 logs signal（查找痛點根源，無法用 logs API 檢索）。
  2. **span 爆量**：每個 INFO/DEBUG/WARN 都變一個 `name="log"` span。
  3. 屬性 key 動態 `log.params.<任意key>`、值全 `fmt.Sprintf("%+v")` → **無限 attribute cardinality**，打爆欄式儲存。
  4. ERROR 分支 `stake[:len(stake)-1]`：無 caller 幀時 `stake=""` → `[:-1]` **panic**。
- **做法**：`sdk/log.LoggerProvider` + `otlploggrpc`（走階段 0 的共用 conn）；把 `logHandler.go` 的 no-op 換成
  真的 `slog.Handler`，record → OTel log record，並從 `ctx` 的 span context 帶 `trace_id` / `span_id` 做關聯。
  舊的 log-as-span code 整段移除，P0-2 的 panic 隨之消失。
- **驗收**：SigNoz `search_logs` 撈得到 log、帶 `service.name` resource 與 `trace_id`；**不再產生 `name="log"` span**。

### ☑ 階段 3　Sampler config 化（原 P1-5）

- **位置**：`opentelCore/init.go` `WithSampler(AlwaysSample())`（寫死）。
- **影響**：prod 全採，成本高。
- **做法**：`ParentBased(TraceIDRatioBased(ratio))`，新增 `openTel.sampleRatio`（給預設值，非全採）；
  錯誤 / 慢請求保留交給 collector tail sampling。
- **驗收**：可用 config 調取樣率；預設非全採。

### ☑ 階段 4　trace 健壯性 + semconv bump（原 P0-3 + P2-9 semconv）

- **位置**：`opentelCore/init.go` `initTraceExporter`（`NewBatchSpanProcessor(nil)` + 靜默吞錯）；`semconv/v1.4.0`。
- **影響**：exporter 失敗仍建 processor，可能 nil-panic / 靜默無 trace；semconv 過舊。
  （備註：`connectGrpc` 用 `grpc.NewClient` 是 lazy dial，collector 不通時 err 通常仍 nil，故 nil 路徑比想像窄，但仍該修。）
- **做法**：錯誤往上傳、fail fast 或明確降級（no-op provider）；`Start` 檢查 `initTraceExporter` 回傳；semconv bump 到近版。
- **驗收**：collector 不通時啟動有明確錯誤、不 panic；語意慣例對齊新版 SigNoz；無死碼。

---

## 風險 / 相容備忘

- **init 順序**：logger 走 `log.RegisterInitAction`、otel core 走 `core.RegisterCoreDependency`（見 `starter.go`）。
  LoggerProvider（core/otel 資源）必須在 log handler 開始寫之前就緒；若框架 log-init 先跑，handler 會拿到 nil provider。
  **接線前需實測註冊順序**（與 slot-game openfeign init 順序雷同類）。
- **`name="log"` span 消失**：改完後這些 span 不再產生。若下游有任何 dashboard / alert 建在這些 log-span 上，會一起壞掉——比 API breaking 更隱蔽，需先盤點。
- **下游自動註冊**：blank import 自動生效，改 API / config 結構要 bump 版本並通知。
- **P0-4 修好後**：下游 slot-game `best_effort.go` 那圈 `recover()` 可移除（非必須）。

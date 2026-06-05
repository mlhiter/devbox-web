# Devbox RW 层存储监控计划书

## 背景

当前 Devbox 已有 CPU、内存等基础监控能力，但缺少对 Devbox 读写层存储，也就是 RW layer 的用量监控。RW 层数据实际落在节点本地 containerd snapshotter 后端存储中，底层可能对应节点上的 LV 或 thin device。由于这是节点本地资源，中心化 controller 不适合直接跨节点查询。

线上监控体系使用 VictoriaMetrics 持久化时序数据，因此本方案采用 commit-worker 节点本地采集，暴露 Prometheus 格式 metrics，再通过 VictoriaMetrics Operator 的 `VMServiceScrape` 接入 vmagent，由 VictoriaMetrics 负责持久化和查询。

## 目标

- 在每个 Devbox 节点本地采集 Devbox RW 层存储用量。
- 复用现有 commit-worker DaemonSet，不新增独立采集组件。
- 暴露 Prometheus 格式 metrics，供 VictoriaMetrics 采集。
- 通过 `VMServiceScrape` 接入线上 vmagent。
- 支持按 namespace、devbox、contentID、node、snapshotter 查询。
- 输出已用字节数、存储限制、使用率、最近采集时间和采集错误。
- 本阶段只做后端/controller 与部署清单改造，不修改前端代码。

## 非目标

- 不修改前端页面和前端 API。
- 不新增数据库表或本地持久化文件。
- 不把高频用量写回 Devbox CR status。
- 不直接执行 `lvs`、`lvdisplay` 等 LVM 命令解析 LV。
- 不在本阶段新增告警规则和前端展示逻辑。

## 总体架构

```text
Devbox CR
  |
  | status.contentID
  | status.commitRecords[contentID].node
  | status.commitRecords[contentID].snapshotter
  v
commit-worker DaemonSet
  |
  | 每个 Devbox 节点本地运行
  | 通过 NODE_NAME 确认当前节点
  v
本机 containerd
  |
  | namespace: k8s.io
  | snapshotter: devbox / stargz
  | snapshot label: containerd.io/snapshot/devbox-content-id
  v
snapshotter Usage()
  |
  v
commit-worker /metrics
  |
  v
Service: commit-worker-metrics-service
  |
  v
VMServiceScrape
  |
  v
vmagent
  |
  v
VictoriaMetrics
```

## 采集设计

### 采集位置

采集逻辑运行在 `commit-worker` 中。原因如下：

- `commit-worker` 是 DaemonSet，每个 Devbox 节点都有一个实例。
- `commit-worker` 通过 `NODE_NAME` 能知道自己所在节点。
- `commit-worker` 已经挂载本机 containerd socket 和 containerd 数据目录。
- RW 层存储是节点本地资源，由本节点采集最直接。

### 节点识别

DaemonSet Pod 通过 Downward API 注入节点名：

```yaml
env:
  - name: NODE_NAME
    valueFrom:
      fieldRef:
        fieldPath: spec.nodeName
```

采集器只处理当前节点拥有的 Devbox：

```text
Devbox.status.commitRecords[Devbox.status.contentID].node == NODE_NAME
```

这样每个 commit-worker 只采本节点数据，不跨节点访问其它节点的 snapshot 或 LV。

### Devbox 目标筛选

采集器会 list Devbox CR，并筛选满足以下条件的对象：

- Devbox 未被删除。
- Devbox 当前处于可观测状态：
  - `spec.state == Running`
  - 或 `status.state == Running`
  - 或 `status.state == Pending`
- Devbox 有当前 `status.contentID`。
- 当前 contentID 对应的 commit record 存在。
- commit record 中记录的 node 等于当前 commit-worker 所在节点。

每个采集目标包含：

```text
namespace
devbox
contentID
node
snapshotter
storageLimit
```

### snapshot 定位

已有 commit 流程会把 Devbox label 转换为 containerd snapshot label：

```text
devbox.sealos.io/content-id
```

转换为：

```text
containerd.io/snapshot/devbox-content-id
```

采集器在本机 containerd snapshotter 中遍历 snapshot，找到：

```text
containerd.io/snapshot/devbox-content-id == Devbox.status.contentID
```

找到 snapshot 后调用 containerd snapshotter API：

```go
snapshotService.Usage(ctx, snapshotName)
```

如果底层 snapshotter 使用 LV 或 thin device，具体映射由 snapshotter 内部负责，采集器不直接解析 LV 名称。

## 指标设计

新增指标如下：

```text
devbox_rw_storage_used_bytes
devbox_rw_storage_limit_bytes
devbox_rw_storage_usage_ratio
devbox_rw_storage_last_collect_timestamp_seconds
devbox_rw_storage_collect_errors_total
```

### 指标字段

`devbox_rw_storage_used_bytes`

- 类型：Gauge
- 含义：当前 Devbox RW 层已使用字节数
- labels：`namespace`、`devbox`、`content_id`、`node`、`snapshotter`

`devbox_rw_storage_limit_bytes`

- 类型：Gauge
- 含义：Devbox 配置的 RW 层存储限制，单位字节
- labels：`namespace`、`devbox`、`content_id`、`node`、`snapshotter`

`devbox_rw_storage_usage_ratio`

- 类型：Gauge
- 含义：RW 层存储使用率
- 计算：`used_bytes / limit_bytes`
- labels：`namespace`、`devbox`、`content_id`、`node`、`snapshotter`

`devbox_rw_storage_last_collect_timestamp_seconds`

- 类型：Gauge
- 含义：最近一次成功采集时间
- 单位：Unix timestamp seconds
- labels：`namespace`、`devbox`、`content_id`、`node`、`snapshotter`

`devbox_rw_storage_collect_errors_total`

- 类型：Counter
- 含义：采集错误计数
- labels：`node`、`reason`

## VictoriaMetrics 持久化设计

采集器本身不做数据落盘，只在进程内维护 Prometheus Gauge 和 Counter。历史数据由 VictoriaMetrics 持久化。

```text
Devbox CR + 本机 containerd snapshotter 状态
        |
        | 周期采集
        v
commit-worker 进程内 metrics
        |
        | vmagent scrape
        v
VictoriaMetrics
```

数据持久化边界：

| 数据 | 是否持久化 | 持久化位置 |
| --- | --- | --- |
| Devbox 当前 contentID | 是 | Kubernetes Devbox CR status |
| Devbox commit record | 是 | Kubernetes Devbox CR status |
| RW 层实际数据 | 是 | 节点本机 containerd snapshotter / 底层存储 |
| 当前采集值 | 否 | commit-worker 进程内 metrics |
| 历史监控曲线 | 是 | VictoriaMetrics |
| 采集错误历史 | 是 | VictoriaMetrics |

不把用量写回 Devbox CR status 的原因：

- 避免高频写 apiserver 和 etcd。
- 避免 Devbox CR resourceVersion 高频变化。
- 避免 controller watch/reconcile 噪声增加。
- 监控类时间序列更适合由 VictoriaMetrics 承担。

## 部署设计

### commit-worker metrics

commit-worker 需要开启 metrics endpoint：

```text
--metrics-bind-address=:8443
```

并暴露端口：

```yaml
ports:
  - containerPort: 8443
    name: https
    protocol: TCP
```

### metrics Service

新增 commit-worker metrics Service：

```yaml
apiVersion: v1
kind: Service
metadata:
  name: commit-worker-metrics-service
spec:
  ports:
    - name: https
      port: 8443
      targetPort: 8443
  selector:
    control-plane: commit-worker
```

如果使用 kustomize `namePrefix: devbox-`，最终名称为：

```text
devbox-commit-worker-metrics-service
```

### VMServiceScrape

线上环境使用 VictoriaMetrics Operator，因此采集规则使用 `VMServiceScrape`：

```yaml
apiVersion: operator.victoriametrics.com/v1beta1
kind: VMServiceScrape
metadata:
  name: commit-worker-metrics-monitor
  namespace: devbox-system
spec:
  endpoints:
    - path: /metrics
      port: https
      scheme: https
      bearerTokenFile: /var/run/secrets/kubernetes.io/serviceaccount/token
      tlsConfig:
        insecureSkipVerify: true
  selector:
    matchLabels:
      control-plane: commit-worker
```

controller-manager metrics 同样使用 `VMServiceScrape` 接入。

### RBAC 注意事项

controller-runtime secure metrics 默认需要认证鉴权。vmagent 抓取 `/metrics` 时，其 ServiceAccount 需要有以下权限：

```yaml
rules:
  - nonResourceURLs:
      - /metrics
    verbs:
      - get
```

当前 controller 已有 `metrics-reader` ClusterRole。上线前需要确认 VictoriaMetrics vmagent 使用的 ServiceAccount 是否已绑定该权限。如果线上 vmagent 使用固定 ServiceAccount，建议在环境级监控栈中统一绑定，避免在 Devbox controller 中硬编码 vmagent 的 release 名称和 namespace。

## PromQL / MetricsQL 查询建议

查询某个 Devbox 当前 RW 层使用率：

```promql
avg(devbox_rw_storage_usage_ratio{namespace="<namespace>",devbox="<devboxName>"})
```

前端如果展示百分比：

```text
rwStoragePercent = ratio * 100
```

查询某个 Devbox 已使用字节数：

```promql
avg(devbox_rw_storage_used_bytes{namespace="<namespace>",devbox="<devboxName>"})
```

查询某个 Devbox 存储限制：

```promql
avg(devbox_rw_storage_limit_bytes{namespace="<namespace>",devbox="<devboxName>"})
```

查询某个节点上的所有 Devbox RW 层用量：

```promql
devbox_rw_storage_used_bytes{node="<nodeName>"}
```

查询采集错误：

```promql
sum by (node, reason) (
  increase(devbox_rw_storage_collect_errors_total[5m])
)
```

## 前端适配建议

本阶段不修改前端代码。后续前端或 API agent 可按以下字段映射：

| 前端字段 | 来源指标 | 说明 |
| --- | --- | --- |
| `rwStorage` | `devbox_rw_storage_usage_ratio * 100` | RW 层使用百分比 |
| `rwStorageUsedBytes` | `devbox_rw_storage_used_bytes` | RW 层已使用字节数 |
| `rwStorageLimitBytes` | `devbox_rw_storage_limit_bytes` | RW 层存储限制字节数 |

如果 `rwStorageLimitBytes` 缺失或为 `0`，前端可以回退使用 Devbox spec 中的 storage limit 做展示。

## 验证计划

### 本地验证

```bash
cd v2/controller
go test ./...
```

### 部署清单验证

生成部署清单：

```bash
cd v2/controller
make pre-deploy
```

确认清单包含：

```text
kind: VMServiceScrape
name: devbox-controller-manager-metrics-monitor
name: devbox-commit-worker-metrics-monitor
name: devbox-commit-worker-metrics-service
```

确认清单不包含：

```text
kind: ServiceMonitor
apiVersion: monitoring.coreos.com/v1
```

### 集群验证

```bash
kubectl rollout status deployment/devbox-controller-manager -n devbox-system
kubectl rollout status daemonset/devbox-commit-worker -n devbox-system
kubectl get vmservicescrape -n devbox-system
kubectl get svc devbox-commit-worker-metrics-service -n devbox-system
kubectl get endpoints devbox-commit-worker-metrics-service -n devbox-system
```

vmagent 接入后，在 VictoriaMetrics 查询：

```promql
devbox_rw_storage_used_bytes
devbox_rw_storage_limit_bytes
devbox_rw_storage_usage_ratio
devbox_rw_storage_last_collect_timestamp_seconds
devbox_rw_storage_collect_errors_total
```

## 风险与注意事项

- 线上集群必须安装 VictoriaMetrics Operator CRD：`vmservicescrapes.operator.victoriametrics.com`。
- vmagent 必须能发现 Devbox namespace 下的 `VMServiceScrape`。
- vmagent ServiceAccount 需要有 `/metrics` 的 `get` 权限。
- commit-worker 必须能访问本机 containerd socket。
- 如果当前没有 Devbox 实例，RW 层业务指标可能暂时为空。
- 如果 Devbox 没有 storage limit，`limit_bytes` 和 `usage_ratio` 会退化为 `0`。

## 阶段计划

| 阶段 | 内容 | 状态 |
| --- | --- | --- |
| 阶段 1 | 后端采集器实现 | 已完成 |
| 阶段 2 | commit-worker metrics 暴露 | 已完成 |
| 阶段 3 | VictoriaMetrics VMServiceScrape 接入清单 | 已完成 |
| 阶段 4 | controller 单元测试 | 已完成 |
| 阶段 5 | dev 集群 controller-only 部署 | 已完成 |
| 阶段 6 | 线上 vmagent RBAC 和采集验证 | 待上线验证 |
| 阶段 7 | 前端/API 适配 | 待单独排期 |
| 阶段 8 | 告警规则 | 待单独排期 |

## 结论

本方案通过 commit-worker DaemonSet 在节点本地采集 Devbox RW 层存储用量，避免中心化 controller 跨节点访问本地 LV 或 snapshot。采集结果以 Prometheus 格式暴露，由 VictoriaMetrics vmagent 通过 `VMServiceScrape` 抓取，并由 VictoriaMetrics 持久化。

当前后端改造和部署清单已按 VictoriaMetrics 方向调整，后续上线重点是确认线上 VMServiceScrape CRD、vmagent 发现规则和 `/metrics` RBAC。

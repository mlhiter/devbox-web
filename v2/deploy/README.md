# DevBox v2 部署梳理

## 文档范围

这份文档用于梳理当前仓库里 DevBox v2 的部署方式，覆盖下面几部分：

- `v2/controller`
- `v2/server`
- `v2/frontend`
- `devbox snapshotter` 和 `stargz` 两条运行路径

这份文档暂时不覆盖：

- 节点侧 containerd runtime handler / snapshotter 插件的安装过程

## 仓库里的部署入口

当前仓库里和 v2 部署直接相关的入口如下：

| 路径 | 作用 |
| --- | --- |
| `v2/controller/deploy/Kubefile` | controller 的 Sealos 打包入口 |
| `v2/controller/deploy/manifests/deploy.yaml` | controller 主清单，包含 namespace、CRD、controller RBAC、Service、DaemonSet、RuntimeClass |
| `v2/controller/deploy/manifests/rbac.yaml` | `devbox-system` 下补充的 RoleBinding |
| `v2/server/deploy/devbox-api.yaml` | `devbox-server` 的 namespace、配置、RBAC、Deployment、Service |
| `v2/frontend/deploy/Kubefile` | frontend 的 Sealos 打包入口，调用 `install.sh` 安装 Helm chart |
| `v2/frontend/deploy/charts/devbox-v2-frontend` | frontend Helm chart，包含 Secret、Deployment、Service、Ingress、App CR |

`v2/deploy` 在补这份文档之前是空目录，所以这份 `README.md` 现在可以视为 v2 的总部署说明。

## 整体部署模型

当前 v2 的部署关系可以拆成三层来看：

1. 节点运行时层

   DevBox 节点需要提前具备 containerd runtime handler 和 snapshotter，这部分当前仓库没有安装清单。

2. 控制面层

   `v2/controller` 以特权 DaemonSet 的方式部署在 DevBox 节点上，负责监听 `Devbox` CRD，并驱动 DevBox 的创建、恢复、暂停、提交、清理等流程。

3. API 层

   `v2/server` 以 Deployment 的方式部署，向外提供 REST API，包括 create、pause、resume、destroy、exec、文件上传下载，以及 gateway 反向代理能力。

4. 前端层

   `v2/frontend` 以 Helm chart 部署到 `devbox-frontend` namespace，负责页面、Next.js route handlers、模板仓库、监控查询、AppLaunchpad 发布入口和 DevBox 高级配置 UI。

## 部署前提

在直接应用仓库里的 manifest 之前，建议先确认下面这些前置条件已经满足。

### 1. DevBox 节点已经打标

controller DaemonSet 和默认 DevBox 调度都依赖这个节点标签：

```yaml
devbox.sealos.io/node: ""
```

如果节点没有这个标签，会出现两个直接后果：

- controller DaemonSet 不会调度到该节点
- `v2/server` 默认创建出来的 DevBox 也不会调度到该节点

### 2. 节点侧已经提供 runtime handler

仓库里会创建两个 RuntimeClass：

| RuntimeClass | handler | snapshotter |
| --- | --- | --- |
| `devbox-runtime` | `devbox-runc` | `devbox` |
| `devbox-stargz-runtime` | `devbox-stargz-runc` | `stargz` |

需要注意的是：

- 这两个 RuntimeClass 对象是由 controller manifest 创建出来的
- 但真正的 runtime handler 并不是这个仓库安装的

也就是说，节点本身必须已经支持：

- runtime handler `devbox-runc`
- runtime handler `devbox-stargz-runc`
- snapshotter `devbox`
- snapshotter `stargz`

如果这些 runtime handler 或 snapshotter 没准备好，RuntimeClass 虽然能创建成功，但 DevBox Pod 真正启动时还是会失败。

### 3. controller 依赖宿主机上的 containerd 目录

controller DaemonSet 会挂载这些 hostPath：

- `/run/containerd`
- `/var/run/containerd`
- `/var/lib/containerd`
- `/var/lib/containerd-stargz-grpc`
- `/etc/containerd/certs.d`
- `/tmp`

这说明当前 controller 的设计是强绑定 containerd 节点环境的，而且要求特权运行。

### 4. controller 的镜像仓库默认值是环境相关的

controller 二进制默认使用下面这些参数：

- `--registry-addr=sealos.hub:5000`
- `--registry-user=admin`
- `--registry-password=passw0rd`

而仓库里的 `v2/controller/deploy/manifests/deploy.yaml` 并没有显式覆盖这些参数。

所以如果你们的镜像仓库不是这套默认值，部署前需要先 patch DaemonSet 的 args。

### 5. `devbox-server` manifest 里有环境占位值

`v2/server/deploy/devbox-api.yaml` 里目前放的是一份示例部署配置，应用前至少要检查下面这些值：

- JWT signing key
- `ssh.host`
- `ssh.port`
- `gateway.domain`
- `gateway.pathPrefix`
- `devbox.createDefaults.image`
- `devbox.createDefaults.runtimeClassName`
- `devbox-server` 镜像地址

## v2 里 snapshotter 的路由方式

从代码实现看，v2 controller 已经支持 `devbox` 和 `stargz` 两条路径。

### RuntimeClass 到 snapshotter 的映射

当前映射关系是根据 `RuntimeClass.handler` 推导出来的：

- `devbox-runtime` -> `devbox-runc` -> snapshotter `devbox`
- `devbox-stargz-runtime` -> `devbox-stargz-runc` -> snapshotter `stargz`

controller 启动时也会同时初始化两套 committer：

- 一套给 `devbox` snapshotter
- 一套给 `stargz` snapshotter

所以从 controller 侧的镜像拉取、提交、恢复逻辑看，这两条路径都已经被接入了。

### `v2/server` 当前默认走哪条路径

`v2/server` 默认创建出来的 DevBox 规格里，会写入：

```yaml
spec:
  runtimeClassName: devbox-runtime
```

这意味着 API server 默认走 `devbox snapshotter`。如果要切到 `stargz`，可以在 server 配置里设置：

```yaml
devbox:
  createDefaults:
    runtimeClassName: devbox-stargz-runtime
```

### 这件事在落地上的含义

- 如果只部署 `v2/controller`，你可以通过直接创建 `Devbox` CR 的方式使用 `devbox` 或 `stargz`
- 如果同时部署 `v2/server`，通过 `devbox.createDefaults.runtimeClassName` 控制 API 创建 DevBox 的默认 RuntimeClass
- 如果通过 `v2/frontend` 创建 DevBox，通过 `DEVBOX_RUNTIME_CLASS_NAME` 控制默认 RuntimeClass

## controller 部署

### `deploy.yaml` 里已经包含什么

`v2/controller/deploy/manifests/deploy.yaml` 当前已经包含：

- namespace `devbox-system`
- `Devbox` CRD
- controller 的 ServiceAccount、ClusterRole、ClusterRoleBinding、Service
- controller DaemonSet
- RuntimeClass `devbox-runtime`
- RuntimeClass `devbox-stargz-runtime`

`v2/controller/deploy/manifests/rbac.yaml` 另外补了一份 `devbox-system` 下的 RoleBinding。

### controller 部署特征

当前 controller manifest 有这些关键特征：

- namespace：`devbox-system`
- workload 类型：`DaemonSet`
- 镜像：`ghcr.io/sealos-apps/devbox-v2-controller:latest`
- nodeSelector：`devbox.sealos.io/node: ""`
- toleration：`devbox.sealos.io/node`
- 容器以 root + privileged 方式运行
- 会挂载 containerd 的宿主机目录

### 推荐部署命令

如果直接用仓库里的清单，推荐这样应用：

```bash
kubectl apply -f v2/controller/deploy/manifests
```

这和 `v2/controller/deploy/Kubefile` 里的行为是一致的：

```bash
kubectl apply -f manifests
```

### 部署前建议重点检查

在应用 controller manifest 前，建议至少确认：

- controller 镜像 tag 是否需要替换
- 是否需要显式补 `--registry-addr`、`--registry-user`、`--registry-password`
- 是否需要调整 `--default-base-image`
- 是否希望每个 DevBox 节点都跑一个 controller Pod

## devbox-server 部署

### `devbox-api.yaml` 里已经包含什么

`v2/server/deploy/devbox-api.yaml` 当前会创建：

- namespace `devbox-server`
- JWT Secret
- 配置 ConfigMap
- ServiceAccount
- ClusterRole 和 ClusterRoleBinding
- leader election 的 Role 和 RoleBinding
- Deployment `devbox-server`
- Service `devbox-server`

### server 暴露端口

这份示例 manifest 暴露了两个端口：

- `8090`：REST API
- `8091`：gateway reverse proxy

这份 manifest 没有包含 ingress，前端也不在本文范围内。

### 部署前建议重点检查

在应用 `v2/server/deploy/devbox-api.yaml` 前，建议重点看这几项：

- `jwt-signing.key`
- 镜像 `registry.cn-hangzhou.aliyuncs.com/lingdie/devbox-server:4.15.2`
- `ssh.host` 和 `ssh.port`
- `gateway.domain` 和 `gateway.pathPrefix`
- `devbox.createDefaults.image`
- 默认 CPU / 内存 / 存储配额

### 推荐部署命令

```bash
kubectl apply -f v2/server/deploy/devbox-api.yaml
```

## 推荐部署顺序

### 第一步：准备节点运行时

先确保 DevBox 节点已经具备：

- 节点标签 `devbox.sealos.io/node`
- runtime handler `devbox-runc`
- runtime handler `devbox-stargz-runc`
- snapshotter `devbox`
- snapshotter `stargz`

这部分当前仓库没有提供安装清单，属于节点侧前置条件。

### 第二步：部署 controller

执行：

```bash
kubectl apply -f v2/controller/deploy/manifests
```

建议验证：

```bash
kubectl -n devbox-system get ds,pod
kubectl get runtimeclass devbox-runtime devbox-stargz-runtime
kubectl get crd devboxes.devbox.sealos.io
```

### 第三步：部署 devbox-server

先按你们环境改好 `v2/server/deploy/devbox-api.yaml`，然后执行：

```bash
kubectl apply -f v2/server/deploy/devbox-api.yaml
```

建议验证：

```bash
kubectl -n devbox-server get deploy,svc,pod
kubectl -n devbox-server port-forward svc/devbox-server 8090:8090 8091:8091
curl -sS http://127.0.0.1:8090/healthz
```

期望返回：

```json
{"code":200,"message":"ok","data":{"status":"healthy"}}
```

## v2 frontend 部署

### frontend chart 里已经包含什么

`v2/frontend/deploy/charts/devbox-v2-frontend` 当前会创建：

- Secret `devbox-frontend-runtime`
- ConfigMap `devbox-frontend-config`
- Deployment `devbox-frontend`
- Service `devbox-frontend`
- Ingress `devbox-frontend`
- Ingress `devbox-challenge`
- App CR `app-system/devbox`

frontend 的 Sealos 包入口是 `v2/frontend/deploy/Kubefile`。它会复制
`install.sh`、`devbox-v2-frontend-values.yaml` 和 `charts/`，然后执行：

```bash
bash install.sh
```

### frontend 运行时配置

`install.sh` 会读取：

- `/root/.sealos/cloud/values/apps/devbox/devbox-v2-frontend-values.yaml`
- `/root/.sealos/cloud/values/global.yaml`
- `sealos-system/sealos-config`
- `sealos-system/registry-config`
- `sealos-system/devbox-config`

这些值会汇总成 Helm overrides。关键字段包括：

- `METRICS_URL`：默认是 VictoriaMetrics select endpoint `http://vmselect-vm-stack-victoria-metrics-k8s-stack.vm.svc.cluster.local:8481/select/0/prometheus`
- `STORAGE_LIMIT`：默认 `20Gi`
- `APP_LAUNCHPAD_URL`：默认 `http://applaunchpad-frontend.applaunchpad-frontend.svc.cluster.local:3000/api/v1alpha`
- `ENABLE_ADVANCED_CONFIG`：默认 `true`，控制高级 Env/ConfigMap UI
- `GPU_SCHEDULER_MODE`：默认 `native`，可按集群改为 `hami`
- `REGISTRY_USER` / `REGISTRY_PASSWORD` / `DEVBOX_DOMAIN_CHALLENGE_SECRET`：写入 `devbox-frontend-runtime` Secret，并通过 `envFrom` 注入 Deployment

`REGISTRY_USER` 和 `REGISTRY_PASSWORD` 必须来自组件 values、环境变量或
`sealos-system/registry-config`；`install.sh` 不会生成默认 registry 凭据。

### frontend 推荐检查

本地渲染检查：

```bash
cd v2/frontend/deploy
make lint
helm template devbox-v2-frontend charts/devbox-v2-frontend --namespace devbox-frontend
```

集群侧检查：

```bash
kubectl -n devbox-frontend rollout status deployment/devbox-frontend --timeout=10m
kubectl -n devbox-frontend get deploy devbox-frontend -o json | jq -r '
  .spec.template.spec.containers[]
  | select(.name=="devbox-frontend")
  | "envFrom=" + ((.envFrom // []) | map(.secretRef.name) | join(",")),
    (.env[]
      | select(.name|test("METRICS_URL|STORAGE_LIMIT|APP_LAUNCHPAD_URL|ENABLE_ADVANCED_CONFIG|GPU_SCHEDULER_MODE|REGISTRY_ADDR"))
      | .name + "=" + .value)
'
```

## 两条 snapshotter 路径怎么用

### 路径 A：`devbox snapshotter`

如果要显式指定走 `devbox`，可以在 `Devbox` CR 里写：

```yaml
spec:
  runtimeClassName: devbox-runtime
```

这也是 `v2/server` 当前默认创建 DevBox 时使用的路径。

### 路径 B：`stargz snapshotter`

如果要显式指定走 `stargz`，可以在 `Devbox` CR 里写：

```yaml
spec:
  runtimeClassName: devbox-stargz-runtime
```

这条路径 controller 已经支持；`v2/server` 和 `v2/frontend` 都可以通过部署配置切换默认 RuntimeClass。

如果你现在就要用 `stargz`，现实可行的方式有两个：

1. 不走 `v2/server`，直接创建 `Devbox` CR
2. 将 `v2/server` 的 `devbox.createDefaults.runtimeClassName` 或 `v2/frontend` 的 `DEVBOX_RUNTIME_CLASS_NAME` 设置为 `devbox-stargz-runtime`

## 当前部署上的缺口

结合仓库现状，当前最明显的部署缺口有这几个：

1. `v2/deploy` 下还没有统一的总 manifest，controller、server 和 frontend 还是分开部署的。
2. 仓库里没有 `devbox snapshotter`、`stargz snapshotter` 以及 runtime handler 的节点侧安装清单。
3. RuntimeClass 选择现在已经可配置，但运行时节点仍必须提前安装匹配的 runtime handler 和 snapshotter。
4. `v2/server/deploy/devbox-api.yaml` 仍然更像一份环境示例，不适合完全不改直接上生产。

## 实际落地建议

如果按当前仓库状态走一条最短路径，建议是：

1. 先在 DevBox 节点上准备好两套 runtime handler 和两套 snapshotter。
2. 应用 `v2/controller/deploy/manifests`。
3. 按环境修改后应用 `v2/server/deploy/devbox-api.yaml`。
4. 用 `v2/frontend/deploy/install.sh` 或对应 Sealos 包安装 frontend chart。
5. 把 `devbox-runtime` 视为当前 API 创建流量的默认路径。
6. 把 `devbox-stargz-runtime` 视为 controller 已支持、但暂时需要手动 CR 或补 server 能力才能走通的路径。

# DevBox 272 修复与页面验证用例

## 基本信息

- 测试目标：验证 DevBox v1 在 Sealos Pro v5.1.2-rc5 环境中的部署修复、页面业务流程和离线镜像结果。
- 目标主机：`192.168.12.161`
- PVE ID：`272`
- 测试分支：`codex/devbox-deploy-adjust-272`
- 修复提交：`94cca3a fix(deploy): support legacy Sealos tools helpers`
- 修复文件：`v1/deploy/install.sh`
- 测试账号：`admin`
- 测试 DevBox：`codex-go-272`
- 发布版本：`v1-272`
- 转换模板：`codex-go-272-template`
- 应用管理发布应用：`codex-go-272-release-vhajyi`

## 修复说明

原始 `sealos run ghcr.io/sealos-apps/devbox-v1-cluster:sha-9e43a42` 在 rc5 环境中失败，原因是目标环境的 `/root/.sealos/cloud/scripts/tools.sh` 缺少较新的 helper 函数。修复在 `v1/deploy/install.sh` 中补齐兼容 fallback：

- `fetch_configmap_data_key`
- `read_jwt_internal`
- `read_cert_tls_reject_unauthorized`
- `read_yaml_file_path`

同时根据自签证书环境把 `NODE_TLS_REJECT_UNAUTHORIZED` 写为 `0`，解决页面“转换模板”时前端请求被自签证书阻断的问题。

## 目录说明

- `cases/`：按测试点拆分的中文用例。
- `screenshots/`：每个用例引用的精选截图。
- `results/`：远端命令、Kubernetes 资源和离线镜像结果日志。

## 用例索引

| 用例 | 名称 | 结果 | 关键截图 | 关键结果文件 |
|---|---|---|---|---|
| TC-01 | 管理中心导入 DevBox 模板 Excel | PASS | `screenshots/tc01-template-precheck.png`, `screenshots/tc01-template-import-done.png` | - |
| TC-02 | 页面创建 Go DevBox | PASS | `screenshots/tc02-devbox-created-list.png` | `results/devbox-created-resource-check.txt` |
| TC-03 | 普通停机后再启动 | PASS | `screenshots/tc03-normal-stop-dialog.png`, `screenshots/tc03-normal-stop-after-submit.png`, `screenshots/tc03-start-after-normal-stop.png` | `results/final-health-after-v3-and-ui.txt` |
| TC-04 | 冷停机后再启动 | PASS | `screenshots/tc04-cold-stop-dialog.png`, `screenshots/tc04-cold-stop-after-submit.png`, `screenshots/tc04-running-after-cold-start.png` | `results/final-health-after-v3-and-ui.txt` |
| TC-05 | 发布 DevBox 版本 | PASS | `screenshots/tc05-release-dialog-filled.png`, `screenshots/tc05-release-success.png` | `results/devbox-release-v1-272-watch.txt` |
| TC-06 | 发布版本转换为模板 | PASS | `screenshots/tc06-template-submit-result.png`, `screenshots/tc06-current-browse-template-page.png`, `screenshots/tc06-current-my-template.png` | `results/final-health-after-v3-and-ui.txt` |
| TC-07 | 发布版本上线到应用管理 | PASS | `screenshots/tc07-app-management-prefilled.png`, `screenshots/tc07-app-management-confirm-dialog.png`, `screenshots/tc07-app-management-running.png` | `results/app-management-publish-check.txt` |
| TC-08 | DevBox 修复包部署与 worker 初始化 | PASS | - | `results/devbox-sha-9e43a42-sealos-run.log`, `results/devbox-cluster-patched-v3-sealos-run.log`, `results/devbox-worker-start-after-patched-run.log` |
| TC-09 | 最终健康与离线镜像检查 | PASS | - | `results/final-health-after-v3-and-ui.txt`, `results/final-devbox-offline-image-check-after-v3-and-ui.txt` |

## 总体结果

所有用例均通过。当前远端状态：

- `devbox-system` 下 `devbox-controller-manager`、`devbox-frontend`、`devbox-service` 均 Ready。
- 旧 namespace `devbox-frontend` 不存在。
- DevBox Release `codex-go-272-v1-272` 状态为 `Success`。
- AppLaunchpad 应用 `codex-go-272-release-vhajyi` 状态为 `1/1 Running`。
- DevBox 范围离线镜像检查 `SCOPED_VIOLATIONS` 为空。


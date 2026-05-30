# TC-08 DevBox 修复包部署与 worker 初始化

## 测试目的

验证原始 DevBox cluster 包失败原因明确，修复后的包可以部署成功，并且按要求执行 worker 初始化脚本。

## 前置条件

- Sealos Pro 基础环境已安装。
- 目标机可通过 SSH 访问。

## 测试流程

1. 在目标机执行原始命令：
   `sealos run ghcr.io/sealos-apps/devbox-v1-cri-shim-patch:sha-9e43a42 && sealos run ghcr.io/sealos-apps/devbox-v1-cluster:sha-9e43a42`
2. 记录失败日志。
3. 修复 `v1/deploy/install.sh` 中缺失 helper 的兼容逻辑。
4. 构建并运行修复后的 cluster 包。
5. 执行：
   `cd /var/lib/sealos/data/default/rootfs/devbox-v1/ && bash devbox-worker-start.sh false`
6. 检查 Helm release、namespace 和服务状态。

## 预期结果

- 原始包失败原因指向缺失 helper。
- 修复包 `sealos run` 成功。
- Helm release `devbox-v1` 状态为 deployed。
- worker 初始化输出 `init cri shim success`。

## 实际结果

PASS。原始包失败日志包含 `fetch_configmap_data_key not found`，修复后 `devbox-v1` 升级成功，worker 初始化成功。

## 结果文件

- `../results/devbox-sha-9e43a42-sealos-run.log`
- `../results/devbox-cluster-patched-v3-sealos-run.log`
- `../results/devbox-worker-start-after-patched-run.log`


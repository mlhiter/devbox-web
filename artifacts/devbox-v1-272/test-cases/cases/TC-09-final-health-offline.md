# TC-09 最终健康与离线镜像检查

## 测试目的

验证全部页面操作完成后，集群健康、DevBox 相关资源和离线镜像状态符合预期。

## 前置条件

- TC-01 至 TC-08 已通过。

## 测试流程

1. 检查节点状态。
2. 检查旧 namespace `devbox-frontend` 是否仍存在。
3. 检查 `devbox-system` 下 Deployment 状态。
4. 检查 DevBox CR、DevBoxRelease 和应用管理资源。
5. 检查非 Running/非 Succeeded Pod。
6. 检查 DevBox 范围镜像是否只使用允许前缀：
   - `sealos.hub:5000/`
   - `hub.192.168.12.161.nip.io/`

## 预期结果

- 节点 Ready。
- 旧 namespace `devbox-frontend` 不存在。
- DevBox 相关 Deployment Ready。
- DevBoxRelease 成功。
- AppLaunchpad 应用 Running。
- DevBox 范围离线镜像无违规项。

## 实际结果

PASS。最终健康检查和 DevBox scoped 离线镜像检查均通过，`SCOPED_VIOLATIONS` 为空。

## 结果文件

- `../results/final-health-after-v3-and-ui.txt`
- `../results/final-devbox-offline-image-check-after-v3-and-ui.txt`


# MinerX Kind 测试计划

## 概述

本文档详细说明了如何使用 Kind (Kubernetes in Docker) 在本地环境中测试 MinerX 项目的三个核心控制器：Chain Controller、Miner Controller 和 MinerSet Controller。

## 测试目标

1. **Chain Controller**: 验证 ConfigMap 和 Miner 资源的创建与管理逻辑
2. **Miner Controller**: 验证 Pod 生命周期管理、健康检查和状态同步
3. **MinerSet Controller**: 验证副本扩缩容、孤儿领养和状态更新

## 环境要求

### 软件依赖

- **Go**: >= 1.21
- **Docker**: >= 20.10
- **Kind**: >= v0.20.0
- **kubectl**: >= 1.27

### 系统要求

- **操作系统**: macOS, Linux, Windows (WSL2)
- **内存**: 至少 8GB RAM
- **CPU**: 至少 4 核心
- **磁盘**: 至少 20GB 可用空间

## 快速开始

### 1. 设置测试环境

```bash
cd /Users/mervyn/go/src/github/minerx
./test/kind/setup.sh
```

此脚本将：
- 检查并安装 Kind
- 创建 Kind 集群（配置：test/kind/kind-config.yaml）
- 构建 minerx manager Docker 镜像
- 加载镜像到 Kind 集群
- 安装 CRDs
- 部署 minerx manager
- 创建测试命名空间

### 2. 运行测试

```bash
# 运行所有 E2E 测试
make test-e2e

# 或手动运行测试
cd test/e2e
go test -v -ginkgo.v
```

### 3. 使用部署脚本

```bash
cd test/kind

# 显示当前资源状态
./deploy.sh status

# 运行完整测试场景
./deploy.sh scenario

# 清理所有测试资源
./deploy.sh cleanup
```

## 测试用例详情

### Chain Controller 测试

#### TC-001: 基础创建测试
**测试文件**: `test/e2e/chain_test.go`
**测试场景**: `When creating a Chain`

**验证点**:
- [ ] 创建 Chain 后自动生成 ConfigMap
- [ ] ConfigMap 标签包含 `chain.onex.io/name=<chain-name>`
- [ ] ConfigMap 有正确的 OwnerReference
- [ ] 创建 Chain 后自动生成 Miner（Genesis Miner）
- [ ] Miner 名称等于 Chain 名称
- [ ] Miner 标签包含 `chain.onex.io/name=<chain-name>`
- [ ] Miner 继承 Chain 的 MinerType 配置
- [ ] Miner 有正确的 OwnerReference

**预期结果**:
```yaml
Chain Status:
  Conditions:
    - type: ConfigMapsCreated
      status: "True"
      reason: Created
    - type: MinersCreated
      status: "True"
      reason: Created
  ConfigMapRef:
    name: "<chain-name>-*"
  MinerRef:
    name: "<chain-name>"
```

#### TC-002: 状态同步测试
**测试场景**: `When updating a Chain`

**验证点**:
- [ ] `ObservedGeneration` 正确设置为 `metadata.generation`
- [ ] `ConfigMapRef` 正确引用创建的 ConfigMap
- [ ] `MinerRef` 正确引用创建的 Miner
- [ ] Conditions 正确标记为 True

#### TC-003: 重协调测试
**测试场景**: `should not recreate resources on reconcile`

**验证点**:
- [ ] 重复 Reconcile 不创建额外的 ConfigMap
- [ ] 重复 Reconcile 不创建额外的 Miner
- [ ] 资源计数保持不变

#### TC-004: 级联删除测试
**测试场景**: `When deleting a Chain`

**验证点**:
- [ ] 删除 Chain 后 ConfigMap 自动删除（OwnerReference 机制）
- [ ] 删除 Chain 后 Miner 自动删除（OwnerReference 机制）
- [ ] Finalizer 正确移除
- [ ] 所有依赖资源清理完成

### Miner Controller 测试

#### TM-001: Pod 创建测试
**测试文件**: `test/e2e/miner_test.go`
**测试场景**: `When creating a Miner`

**验证点**:
- [ ] 创建 Miner 后自动创建 Pod
- [ ] Pod 名称与 Miner 名称相同
- [ ] Pod 标签包含 `app: miner`
- [ ] Pod 标签包含 `miner.onex.io/name=<miner-name>`
- [ ] Pod 标签包含 `chain.onex.io/name=<chain-name>`
- [ ] Pod 注解包含 `miner.onex.io/name=<miner-name>`
- [ ] Pod OwnerReference 指向 Miner
- [ ] Pod RestartPolicy 继承 Miner 配置

**镜像策略验证**:

| MinerType | 预期镜像 |
|-----------|----------|
| small | `nginx:alpine` |
| medium | `nginx` |
| large | `redis:alpine` |
| unknown | `busybox` |

#### TM-002: 状态同步测试
**测试场景**: `should update Miner status correctly`

**验证点**:
- [ ] `PodRef` 正确设置
- [ ] `Phase` 初始设置为 `Provisioning`
- [ ] `LastUpdated` 时间戳正确设置
- [ ] Conditions 正确标记

#### TM-003: Pod 就绪测试
**测试场景**: `When Pod becomes Ready`

**验证点**:
- [ ] Pod Ready 时 Miner Phase 更新为 `Running`
- [ ] `InfrastructureReadyCondition` 设为 True
- [ ] `BootstrapReadyCondition` 设为 True
- [ ] `MinerPodHealthyCondition` 设为 True
- [ ] `Addresses` 字段包含 Pod IPs

**Phase 状态转换**:

| Pod Phase | Pod Ready | Miner Phase | MinerPodHealthyCondition |
|----------|-----------|-------------|------------------------|
| Pending | - | Provisioning | False (Provisioning) |
| Running | False | Provisioning | False |
| Running | True | Running | True |
| Failed | - | Failed | False |
| - | Deleting | Deleting | False |

#### TM-004: Pod 故障测试
**测试场景**: `When Pod fails`

**验证点**:
- [ ] Pod 失败时 Miner Phase 更新为 `Failed`
- [ ] `FailureReason` 字段正确设置
- [ ] Conditions 正确反映失败状态

#### TM-005: 删除重试测试
**测试场景**: `When deleting a Miner`

**验证点**:
- [ ] 删除 Miner 时 Pod 被删除
- [ ] 支持 `podDeletionTimeout` 配置
- [ ] 删除失败时进行重试
- [ ] 超时后标记删除失败

### MinerSet Controller 测试

#### TMS-001: 副本创建测试
**测试文件**: `test/e2e/minerset_test.go`
**测试场景**: `When creating a MinerSet`

**验证点**:
- [ ] 创建指定数量的 Miner
- [ ] 所有 Miner 都有 `minerset.onex.io/name` 标签
- [ ] 所有 Miner 都有 `chain.onex.io/name` 标签
- [ ] 所有 Miner 的 OwnerReference 指向 MinerSet
- [ ] Miner 名称使用 GenerateName（格式：`<minerset-name>-`）
- [ ] Status.Replicas 正确统计 Miner 数量
- [ ] Status.FullyLabeledReplicas 正确统计完全匹配标签的 Miner

#### TMS-002: 状态更新测试
**测试场景**: `should update MinerSet status correctly`

**验证点**:
- [ ] `Replicas` 等于实际 Miner 数量
- [ ] `FullyLabeledReplicas` 等于标签完全匹配的 Miner 数量
- [ ] `ReadyReplicas` 等于 Phase=Running 的 Miner 数量
- [ ] `AvailableReplicas` 等于 Ready 且 Generation 匹配的 Miner 数量
- [ ] `ObservedGeneration` 正确设置
- [ ] `MinersCreatedCondition` 设为 True
- [ ] `ResizedCondition` 设为 True

#### TMS-003: 扩容测试
**测试场景**: `When scaling up a MinerSet`

**验证点**:
- [ ] 增加 replicas 时创建额外的 Miner
- [ ] 创建数量等于 diff
- [ ] `ResizedCondition` 临时设为 False（Creating reason）
- [ ] 所有 Miner 就绪后 `ResizedCondition` 设为 True
- [ ] Status.Replicas 正确更新

#### TMS-004: 缩容测试
**测试场景**: `When scaling down a MinerSet`

**验证点**:
- [ ] 减少 replicas 时删除 Miner
- [ ] 删除数量等于 diff
- [ ] `ResizedCondition` 临时设为 False（Deleting reason）
- [ ] 所有剩余 Miner 就绪后 `ResizedCondition` 设为 True
- [ ] Status.Replicas 正确更新

**删除策略验证**:

| DeletePolicy | 预期行为 |
|-------------|----------|
| Random | 随机选择副本删除 |
| Newest | 删除最新创建的副本 |
| Oldest | 删除最旧的副本 |

#### TMS-005: 孤儿领养测试
**测试场景**: `When adopting orphan Miners`

**验证点**:
- [ ] 无 OwnerReference 的孤儿 Miner 被自动领养
- [ ] Selector 匹配检查正确
- [ ] Miner 的 OwnerReference 更新为 MinerSet
- [ ] MinerSet Status.Replicas 包含领养的 Miner
- [ ] 领养的 Miner 标签包含 `minerset.onex.io/name`

#### TMS-006: 删除测试
**测试场景**: `When deleting a MinerSet`

**验证点**:
- [ ] 删除 MinerSet 时所有管理的 Miner 被删除
- [ ] 不删除不属于 MinerSet 的 Miner
- [ ] Finalizer 正确移除

#### TMS-007: 控制器最终一致性测试
**测试场景**: 验证达到期望状态后不再重复操作

**验证点**:
- [ ] 所有 Miner Ready 后不重复创建/删除
- [ ] Status 字段保持稳定
- [ ] 重复 Reconcile 不产生副作用

## 集成测试场景

### IT-001: 协同工作测试
**目标**: 验证 Chain 和 MinerSet 创建的 Miner 能独立工作

**步骤**:
1. 创建 Chain（自动创建 Genesis Miner）
2. 创建 MinerSet（创建多个 Miner）
3. 验证两者都能正常运行
4. 验证各自管理的 Pod 正常

### IT-002: 故障恢复测试
**目标**: 验证控制器在 Pod 故障时的恢复能力

**步骤**:
1. 创建 MinerSet（replicas: 3）
2. 手动删除其中一个 Pod
3. 验证控制器检测到 Pod 缺失
4. 验证重新创建 Pod
5. 验证最终恢复到期望状态

### IT-003: 完整生命周期测试
**目标**: 验证资源从创建到删除的完整生命周期

**步骤**:
1. 创建 Chain
2. 创建 MinerSet
3. 验证所有资源正常运行
4. 缩容 MinerSet
5. 删除 MinerSet
6. 删除 Chain
7. 验证所有资源清理完成

## 调试指南

### 查看控制器日志

```bash
# Chain Controller
kubectl logs -n minerx-system -l control-plane=controller-manager -c manager | grep chain

# Miner Controller
kubectl logs -n minerx-system -l control-plane=controller-manager -c manager | grep miner

# MinerSet Controller
kubectl logs -n minerx-system -l control-plane=controller-manager -c manager | grep minerset
```

### 查看资源事件

```bash
# Chain 事件
kubectl describe chain <chain-name>

# Miner 事件
kubectl describe miner <miner-name>

# MinerSet 事件
kubectl describe minerset <minerset-name>

# Pod 事件
kubectl describe pod <pod-name>
```

### 查看资源状态

```bash
# 查看 Chain 详细状态
kubectl get chain <chain-name> -o yaml

# 查看 Miner 详细状态
kubectl get miner <miner-name> -o yaml

# 查看 MinerSet 详细状态
kubectl get minerset <minerset-name> -o yaml
```

## 清理环境

### 清理测试资源

```bash
cd test/kind
./deploy.sh cleanup
```

### 删除 Kind 集群

```bash
kind delete cluster --name minerx-test
```

## 故障排查

### 常见问题

**Q1: Kind 集群创建失败**
- 检查 Docker 是否运行
- 检查端口是否被占用（30080, 30443）
- 检查 Kind 版本（要求 >= v0.20.0）

**Q2: 镜像加载失败**
- 检查镜像是否构建成功：`docker images | grep minerx`
- 手动加载镜像：`kind load docker-image minerx/manager:latest --name minerx-test`

**Q3: 控制器 Pod 处于 CrashLoopBackOff**
- 查看控制器日志：`kubectl logs -n minerx-system -l control-plane=controller-manager`
- 检查 CRD 是否安装：`kubectl get crd | grep onex.io`

**Q4: Miner Pod 无法启动**
- 查看描述：`kubectl describe pod <pod-name>`
- 检查镜像是否在集群中可用
- 查看控制器日志中的错误信息

**Q5: 测试超时**
- 增加超时时间（修改测试中的 Eventually 超时参数）
- 检查集群资源是否充足
- 重启 Kind 集群

## 测试清单

完成所有测试后，请使用以下清单确认：

### Chain Controller
- [ ] TC-001: 基础创建测试
- [ ] TC-002: 状态同步测试
- [ ] TC-003: 重协调测试
- [ ] TC-004: 级联删除测试

### Miner Controller
- [ ] TM-001: Pod 创建测试
- [ ] TM-002: 状态同步测试
- [ ] TM-003: Pod 就绪测试
- [ ] TM-004: Pod 故障测试
- [ ] TM-005: 删除重试测试

### MinerSet Controller
- [ ] TMS-001: 副本创建测试
- [ ] TMS-002: 状态更新测试
- [ ] TMS-003: 扩容测试
- [ ] TMS-004: 缩容测试
- [ ] TMS-005: 孤儿领养测试
- [ ] TMS-006: 删除测试
- [ ] TMS-007: 控制器最终一致性测试

### 集成测试
- [ ] IT-001: 协同工作测试
- [ ] IT-002: 故障恢复测试
- [ ] IT-003: 完整生命周期测试

## 附录

### 文件结构

```
minerx/
├── test/
│   ├── kind/
│   │   ├── kind-config.yaml      # Kind 集群配置
│   │   ├── setup.sh              # 环境初始化脚本
│   │   └── deploy.sh            # 部署和测试脚本
│   └── e2e/
│       ├── chain_test.go         # Chain Controller E2E 测试
│       ├── miner_test.go         # Miner Controller E2E 测试
│       └── minerset_test.go      # MinerSet Controller E2E 测试
```

### 相关文档

- MinerX README.md: 项目概览和使用说明
- API 文档: `pkg/apis/apps/v1alpha1/` 目录下的类型定义
- Controller 实现: `internal/controller/` 目录下的控制器代码

### 参考资料

- Kind 文档: https://kind.sigs.k8s.io/
- Kubebuilder 文档: https://book.kubebuilder.io/
- Kubernetes Operator 最佳实践: https://kubernetes.io/docs/concepts/extend-kubernetes/operator/

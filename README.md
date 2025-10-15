# Nginx Operator

一个 Kubernetes Operator，用于管理 Nginx 部署集群。支持通过修改自定义资源（CR）来动态更新 Nginx 配置，并自动重启相关容器。

## 功能特性

- ✨ 通过 CRD 管理 Nginx 集群部署
- 🔧 支持动态修改 Nginx 配置文件（nginx.conf）
- 🔄 配置变更后自动触发 Pod 滚动更新
- 📊 实时状态监控和报告
- 🎯 支持配置多副本部署

## 架构说明

该 Operator 包含以下核心组件：

1. **NginxCluster CRD**：定义 Nginx 集群的期望状态
2. **Controller**：监听 CRD 变化，协调实际状态与期望状态
3. **ConfigMap**：存储 Nginx 配置文件
4. **Deployment**：管理 Nginx Pod 副本
5. **Service**：提供服务访问入口

### 工作流程

1. 用户创建或更新 `NginxCluster` 资源
2. Controller 检测到变化，计算配置文件的哈希值
3. 如果配置发生变化，更新 ConfigMap 中的 nginx.conf
4. 通过修改 Deployment 的 Pod Template 注解触发滚动更新
5. Kubernetes 自动执行滚动更新，新配置生效

## 快速开始

### 前置要求

- Go 1.21+
- Kubernetes 集群（1.25+）
- kubectl 配置正确
- Docker（用于构建镜像）

### 安装 CRD

```bash
make install
```

### 本地运行 Operator

```bash
# 运行 controller（会连接到当前 kubectl 配置的集群）
make run
```

### 部署到集群

```bash
# 构建 Docker 镜像
make docker-build IMG=your-registry/nginx-operator:latest

# 推送镜像
make docker-push IMG=your-registry/nginx-operator:latest

# 部署到集群
make deploy IMG=your-registry/nginx-operator:latest
```

## 使用示例

### 创建 Nginx 集群

创建一个示例 `NginxCluster` 资源：

```yaml
apiVersion: nginx.example.com/v1
kind: NginxCluster
metadata:
  name: my-nginx
  namespace: default
spec:
  replicas: 3
  image: nginx:1.25
  nginxConf: |
    events {
        worker_connections 1024;
    }

    http {
        include       /etc/nginx/mime.types;
        default_type  application/octet-stream;

        sendfile        on;
        keepalive_timeout  65;

        server {
            listen       80;
            server_name  localhost;

            location / {
                root   /usr/share/nginx/html;
                index  index.html index.htm;
            }

            location /health {
                access_log off;
                return 200 "healthy\n";
                add_header Content-Type text/plain;
            }
        }
    }
```

应用配置：

```bash
kubectl apply -f config/samples/nginx_v1_nginxcluster.yaml
```

### 查看 Nginx 集群状态

```bash
# 查看所有 NginxCluster 资源
kubectl get nginxclusters

# 查看详细信息
kubectl describe nginxcluster my-nginx

# 查看 Pod 状态
kubectl get pods -l cluster=my-nginx
```

### 更新 Nginx 配置

修改 `NginxCluster` 资源中的 `nginxConf` 字段：

```bash
kubectl edit nginxcluster my-nginx
```

或者使用 patch 命令：

```bash
kubectl patch nginxcluster my-nginx --type='json' -p='[{
  "op": "replace",
  "path": "/spec/nginxConf",
  "value": "events {\n    worker_connections 2048;\n}\n\nhttp {\n    server {\n        listen 80;\n        location / {\n            return 200 \"Hello from updated config!\";\n        }\n    }\n}\n"
}]'
```

Operator 会自动检测配置变化并触发 Pod 滚动更新。

### 扩缩容

```bash
# 扩容到 5 个副本
kubectl patch nginxcluster my-nginx --type='merge' -p '{"spec":{"replicas":5}}'

# 缩容到 2 个副本
kubectl patch nginxcluster my-nginx --type='merge' -p '{"spec":{"replicas":2}}'
```

### 删除 Nginx 集群

```bash
kubectl delete nginxcluster my-nginx
```

## 开发指南

### 项目结构

```
operator/
├── api/v1/                      # CRD 定义
│   ├── nginxcluster_types.go   # NginxCluster 类型定义
│   └── groupversion_info.go    # API 组版本信息
├── controllers/                 # Controller 实现
│   └── nginxcluster_controller.go
├── config/                      # Kubernetes 配置文件
│   ├── crd/                    # CRD YAML 定义
│   ├── rbac/                   # RBAC 权限配置
│   ├── manager/                # Operator 部署配置
│   ├── samples/                # 示例 CR
│   └── default/                # Kustomize 默认配置
├── main.go                      # 入口文件
├── Dockerfile                   # 容器镜像构建文件
├── Makefile                     # 构建和部署命令
└── README.md                    # 项目文档
```

### 构建和测试

```bash
# 格式化代码
make fmt

# 代码检查
make vet

# 运行测试
make test

# 生成代码和 manifests
make generate manifests

# 构建二进制文件
make build
```

### 调试

```bash
# 查看 controller 日志（本地运行时）
# 日志会输出到控制台

# 查看 controller 日志（集群部署时）
kubectl logs -n nginx-operator-system deployment/nginx-operator-controller-manager -f
```

## API 参考

### NginxClusterSpec

| 字段 | 类型 | 描述 | 默认值 |
|------|------|------|--------|
| `replicas` | int32 | Nginx 实例副本数（最小值：1） | 1 |
| `image` | string | 使用的 Nginx 镜像 | nginx:latest |
| `nginxConf` | string | Nginx 配置文件内容 | 默认配置 |

### NginxClusterStatus

| 字段 | 类型 | 描述 |
|------|------|------|
| `replicas` | int32 | 当前副本数 |
| `readyReplicas` | int32 | 就绪副本数 |
| `configHash` | string | 当前配置的哈希值 |
| `lastUpdateTime` | Time | 最后更新时间 |

## 常见问题

### Q: 配置更新后，Pod 多久会重启？
A: Operator 检测到配置变化后会立即触发滚动更新，具体完成时间取决于集群资源和 Pod 数量。

### Q: 如何验证配置是否生效？
A: 可以通过以下方式验证：
```bash
# 查看 ConfigMap
kubectl get configmap my-nginx-nginx-config -o yaml

# 进入 Pod 查看配置文件
kubectl exec -it <pod-name> -- cat /etc/nginx/nginx.conf

# 查看 NginxCluster 状态中的 configHash
kubectl get nginxcluster my-nginx -o jsonpath='{.status.configHash}'
```

### Q: 支持哪些 Nginx 配置？
A: 支持完整的 nginx.conf 配置内容。需要注意的是，配置需要是有效的 Nginx 配置格式，否则 Nginx 容器会启动失败。

### Q: 如何回滚配置？
A: 修改 `NginxCluster` 资源的 `nginxConf` 字段为之前的配置内容即可，Operator 会自动触发滚动更新。

## 清理

```bash
# 删除所有 NginxCluster 实例
kubectl delete nginxclusters --all

# 卸载 CRD
make uninstall

# 卸载 Operator（如果部署到集群）
make undeploy
```

## 贡献

欢迎提交 Issue 和 Pull Request！

## 许可证

Apache License 2.0

## 技术栈

- **语言**: Go 1.21
- **框架**: Kubebuilder v3
- **运行时**: controller-runtime v0.16.3
- **K8s 版本**: 1.28+

## 相关资源

- [Kubebuilder 文档](https://book.kubebuilder.io/)
- [Kubernetes Operator 模式](https://kubernetes.io/docs/concepts/extend-kubernetes/operator/)
- [Nginx 官方文档](https://nginx.org/en/docs/)



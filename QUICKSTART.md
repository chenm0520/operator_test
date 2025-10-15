# 快速开始指南

本指南将帮助你在 5 分钟内运行 Nginx Operator。

## 步骤 1: 准备环境

确保你有：
- 可访问的 Kubernetes 集群
- kubectl 已配置
- Go 1.21+ (用于本地运行)

## 步骤 2: 安装 CRD

```bash
# 克隆项目后，进入项目目录
cd nginx-operator

# 安装 CRD 到集群
make install
```

验证 CRD 安装成功：
```bash
kubectl get crd nginxclusters.nginx.example.com
```

## 步骤 3: 运行 Operator

### 方式 A: 本地运行（推荐用于开发）

```bash
# 下载依赖
go mod download

# 运行 controller
make run
```

保持终端打开，controller 将开始监听集群中的 NginxCluster 资源。

### 方式 B: 部署到集群（推荐用于生产）

```bash
# 构建并推送镜像（替换为你的镜像仓库）
make docker-build docker-push IMG=<your-registry>/nginx-operator:v1.0.0

# 部署到集群
make deploy IMG=<your-registry>/nginx-operator:v1.0.0
```

验证部署：
```bash
kubectl get deployment -n nginx-operator-system
```

## 步骤 4: 创建第一个 Nginx 集群

打开新终端，创建一个简单的 Nginx 集群：

```bash
kubectl apply -f config/samples/nginx_v1_nginxcluster_simple.yaml
```

或者创建一个自定义配置的集群：
```bash
kubectl apply -f config/samples/nginx_v1_nginxcluster.yaml
```

## 步骤 5: 验证部署

```bash
# 查看 NginxCluster 资源
kubectl get nginxclusters

# 查看创建的 Pods
kubectl get pods -l app=nginx

# 查看详细信息
kubectl describe nginxcluster simple-nginx

# 查看 Service
kubectl get svc simple-nginx
```

你应该看到类似输出：
```
NAME           REPLICAS   READY   IMAGE        AGE
simple-nginx   2          2       nginx:1.25   1m
```

## 步骤 6: 测试访问

```bash
# 端口转发到本地（在新终端中运行）
kubectl port-forward svc/simple-nginx 8080:80

# 在浏览器访问 http://localhost:8080
# 或使用 curl 测试
curl http://localhost:8080
```

## 步骤 7: 更新配置

编辑 Nginx 配置：

```bash
kubectl edit nginxcluster simple-nginx
```

修改 `spec.nginxConf` 字段，保存后退出。观察 Operator 日志，你会看到：
1. ConfigMap 被更新
2. Deployment 触发滚动更新
3. 新的 Pods 启动，旧的 Pods 逐步停止

或者使用 patch 命令快速更新副本数：
```bash
kubectl patch nginxcluster simple-nginx --type='merge' -p '{"spec":{"replicas":3}}'
```

## 步骤 8: 清理

```bash
# 删除 Nginx 集群
kubectl delete nginxcluster simple-nginx

# 如果需要完全卸载
make uninstall  # 删除 CRD
make undeploy   # 删除 Operator（如果部署到集群）
```

## 下一步

- 查看 [README.md](README.md) 了解更多详细功能
- 尝试修改示例配置文件
- 查看 [开发指南](#) 了解如何自定义 Operator

## 故障排查

### Operator 无法启动
```bash
# 检查 RBAC 权限
kubectl get clusterrole manager-role

# 查看日志
kubectl logs -n nginx-operator-system deployment/nginx-operator-controller-manager
```

### Pods 启动失败
```bash
# 检查 Pod 事件
kubectl describe pod <pod-name>

# 检查 ConfigMap
kubectl get configmap simple-nginx-nginx-config -o yaml

# 验证 Nginx 配置是否有效
kubectl exec <pod-name> -- nginx -t
```

### 配置未生效
```bash
# 检查 configHash 是否更新
kubectl get nginxcluster simple-nginx -o jsonpath='{.status.configHash}'

# 检查 Pod 注解
kubectl get pods -l app=nginx -o jsonpath='{.items[0].metadata.annotations}'
```

需要帮助？查看完整文档或提交 Issue。



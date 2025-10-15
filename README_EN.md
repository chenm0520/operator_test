# Nginx Operator

A Kubernetes Operator for managing Nginx deployment clusters. It supports dynamically updating Nginx configurations by modifying custom resources (CR) and automatically restarting related containers.

## Features

- ✨ Manage Nginx cluster deployments through CRD
- 🔧 Support dynamic modification of Nginx configuration files (nginx.conf)
- 🔄 Automatically trigger pod rolling updates after configuration changes
- 📊 Real-time status monitoring and reporting
- 🎯 Support multi-replica deployments

## Architecture

This Operator contains the following core components:

1. **NginxCluster CRD**: Defines the desired state of the Nginx cluster
2. **Controller**: Watches CRD changes and reconciles actual state with desired state
3. **ConfigMap**: Stores Nginx configuration files
4. **Deployment**: Manages Nginx pod replicas
5. **Service**: Provides service access endpoint

### Workflow

1. User creates or updates `NginxCluster` resource
2. Controller detects the change and calculates configuration hash
3. If configuration changed, updates nginx.conf in ConfigMap
4. Triggers rolling update by modifying Deployment pod template annotations
5. Kubernetes automatically performs rolling update with new configuration

## Quick Start

### Prerequisites

- Go 1.21+
- Kubernetes cluster (1.25+)
- kubectl configured correctly
- Docker (for building images)

### Install CRDs

```bash
make install
```

### Run Operator Locally

```bash
# Run controller (connects to cluster in current kubectl config)
make run
```

### Deploy to Cluster

```bash
# Build Docker image
make docker-build IMG=your-registry/nginx-operator:latest

# Push image
make docker-push IMG=your-registry/nginx-operator:latest

# Deploy to cluster
make deploy IMG=your-registry/nginx-operator:latest
```

## Usage Examples

### Create Nginx Cluster

Create an example `NginxCluster` resource:

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

Apply the configuration:

```bash
kubectl apply -f config/samples/nginx_v1_nginxcluster.yaml
```

### View Nginx Cluster Status

```bash
# List all NginxCluster resources
kubectl get nginxclusters

# View detailed information
kubectl describe nginxcluster my-nginx

# View pod status
kubectl get pods -l cluster=my-nginx
```

### Update Nginx Configuration

Modify the `nginxConf` field in `NginxCluster` resource:

```bash
kubectl edit nginxcluster my-nginx
```

Or use patch command:

```bash
kubectl patch nginxcluster my-nginx --type='json' -p='[{
  "op": "replace",
  "path": "/spec/nginxConf",
  "value": "events {\n    worker_connections 2048;\n}\n\nhttp {\n    server {\n        listen 80;\n        location / {\n            return 200 \"Hello from updated config!\";\n        }\n    }\n}\n"
}]'
```

The Operator will automatically detect configuration changes and trigger pod rolling updates.

### Scale

```bash
# Scale up to 5 replicas
kubectl patch nginxcluster my-nginx --type='merge' -p '{"spec":{"replicas":5}}'

# Scale down to 2 replicas
kubectl patch nginxcluster my-nginx --type='merge' -p '{"spec":{"replicas":2}}'
```

### Delete Nginx Cluster

```bash
kubectl delete nginxcluster my-nginx
```

## Development Guide

### Project Structure

```
operator/
├── api/v1/                      # CRD definitions
│   ├── nginxcluster_types.go   # NginxCluster type definition
│   └── groupversion_info.go    # API group version info
├── controllers/                 # Controller implementation
│   └── nginxcluster_controller.go
├── config/                      # Kubernetes configuration files
│   ├── crd/                    # CRD YAML definitions
│   ├── rbac/                   # RBAC permission configs
│   ├── manager/                # Operator deployment config
│   ├── samples/                # Sample CRs
│   └── default/                # Kustomize default config
├── main.go                      # Entry point
├── Dockerfile                   # Container image build file
├── Makefile                     # Build and deploy commands
└── README.md                    # Project documentation
```

### Build and Test

```bash
# Format code
make fmt

# Code lint
make vet

# Run tests
make test

# Generate code and manifests
make generate manifests

# Build binary
make build
```

## API Reference

### NginxClusterSpec

| Field | Type | Description | Default |
|-------|------|-------------|---------|
| `replicas` | int32 | Number of Nginx replicas (minimum: 1) | 1 |
| `image` | string | Nginx image to use | nginx:latest |
| `nginxConf` | string | Nginx configuration file content | Default config |

### NginxClusterStatus

| Field | Type | Description |
|-------|------|-------------|
| `replicas` | int32 | Current replica count |
| `readyReplicas` | int32 | Ready replica count |
| `configHash` | string | Hash of current configuration |
| `lastUpdateTime` | Time | Last update timestamp |

## License

Apache License 2.0

## Tech Stack

- **Language**: Go 1.21
- **Framework**: Kubebuilder v3
- **Runtime**: controller-runtime v0.16.3
- **K8s Version**: 1.28+



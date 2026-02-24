# Kustomize Deployment for RSS Serve

This directory contains Kustomize configurations for deploying the RSS Serve application to Kubernetes.

## Structure

```
kustomize/
├── base/                  # Base Kubernetes manifests
│   ├── deployment.yaml    # Deployment configuration
│   ├── service.yaml       # Service configuration
│   ├── ingress.yaml       # Ingress configuration
│   ├── kustomization.yaml # Base kustomization
│   └── .env.secrets.template # Template for secrets
└── overlays/
    └── production/        # Production environment overlay
        └── kustomization.yaml
```

## Setup Instructions

### 0. Namespace Setup

The application will be deployed to the `rss-serve` namespace, which will be automatically created by Kustomize.

### 1. Database Configuration

The application expects the database URL to be in an existing Kubernetes secret called `homelab-app` with a key `uri`. Ensure this secret exists in the `rss-serve` namespace before deploying.

```bash
# Example of how the secret should look:
kubectl create secret generic homelab-app -n rss-serve \
  --from-literal=uri='postgres://user:password@host:port/database'
```

### 2. JWT Secret Configuration

The application requires a JWT secret for token signing. This should be created as a SealedSecret.

#### Option A: Generate a new sealed secret

```bash
# Install kubeseal if you haven't already
# brew install kubeseal (macOS) or see https://github.com/bitnami-labs/sealed-secrets

# Run the helper script
cd kustomize/base
./create-sealed-secret.sh

# Replace the jwt-secret.yaml file with the generated sealed secret
cp jwt-secret-sealed.yaml jwt-secret.yaml
rm jwt-secret-sealed.yaml
```

#### Option B: Use existing JWT secret

If you already have a JWT secret, create a sealed secret:

```bash
# Create a temporary secret file
cat > jwt-secret-temp.yaml <<EOF
apiVersion: v1
kind: Secret
metadata:
  name: rss-serve-jwt
  namespace: rss-serve
  labels:
    app: rss-serve
type: Opaque
data:
  JWT_SECRET: $(echo -n "your-existing-jwt-secret" | base64)
EOF

# Seal it
kubeseal --format yaml < jwt-secret-temp.yaml > kustomize/base/jwt-secret.yaml
rm jwt-secret-temp.yaml
```

#### Option B: Use existing JWT secret

If you already have a JWT secret, create a sealed secret:

```bash
# Create a temporary secret file
cat > jwt-secret-temp.yaml <<EOF
apiVersion: v1
kind: Secret
metadata:
  name: rss-serve-jwt
  namespace: rss-serve
type: Opaque
data:
  JWT_SECRET: $(echo -n "your-existing-jwt-secret" | base64)
EOF

# Seal it
kubeseal --format yaml < jwt-secret-temp.yaml > kustomize/base/jwt-secret.yaml
rm jwt-secret-temp.yaml
```

### 2. Deploy to Kubernetes

#### For production:
```bash
kubectl apply -k kustomize/overlays/production
```

#### For development/testing:
```bash
kubectl apply -k kustomize/base
```

### 3. Update Image Version

To deploy a specific version, update the image tag in the production overlay:

```yaml
# kustomize/overlays/production/kustomization.yaml
images:
  - name: ghcr.io/hugis/rss-serve
    newTag: v1.2.3  # Change this to your desired version
```

## Configuration Details

### Service
- Exposes port 80 internally, routes to container port 3000
- Annotated for Cloudflare Tunnel with hostname `feed.hugis.dev`

### Ingress
- Uses Cloudflare Tunnel ingress class
- Routes `feed.hugis.dev` to the service

### Deployment
- Runs 1 replica in base, 2 replicas in production
- Includes liveness and readiness probes
- Resource limits: 500m CPU, 512Mi memory

### Environment Variables
- `PORT`: Set to 3000 via ConfigMap (`rss-serve-config`)
- `DATABASE_URL`: Read from existing `homelab-app` secret (key: `uri`)
- `JWT_SECRET`: Read from `rss-serve-jwt` sealed secret (key: `JWT_SECRET`)

## Updating from GitHub Releases

When you create a GitHub release, the GitHub Action will automatically build and push a Docker image tagged with the release version. You can then update your deployment by changing the image tag in the production overlay.
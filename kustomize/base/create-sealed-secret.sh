#!/bin/bash

# Script to create a sealed secret for JWT_SECRET
# Requires: kubeseal (https://github.com/bitnami-labs/sealed-secrets)

# Generate a random JWT secret (32 bytes)
JWT_SECRET=$(openssl rand -hex 32)

echo "Generated JWT Secret: $JWT_SECRET"
echo "This secret will be used for JWT token signing"

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
  JWT_SECRET: $(echo -n "$JWT_SECRET" | base64)
EOF

# Seal the secret
echo "Creating sealed secret..."
kubeseal --format yaml < jwt-secret-temp.yaml > jwt-secret-sealed.yaml

# Clean up
rm jwt-secret-temp.yaml

echo "Sealed secret created: jwt-secret-sealed.yaml"
echo "Replace kustomize/base/jwt-secret.yaml with this sealed secret file"
echo "Don't forget to commit the sealed secret to git!"

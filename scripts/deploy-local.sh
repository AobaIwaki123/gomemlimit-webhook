#!/usr/bin/env bash
set -euo pipefail

# Deploy script for gomemlimit-webhook into local Kubernetes cluster

NAMESPACE="gomemlimit-system"
SERVICE_NAME="gomemlimit-webhook"
SECRET_NAME="gomemlimit-webhook-tls"
TMP_DIR=$(mktemp -d)
trap 'rm -rf "${TMP_DIR}"' EXIT

echo "=== 1. Preparing namespace: ${NAMESPACE} ==="
kubectl get namespace "${NAMESPACE}" >/dev/null 2>&1 || kubectl create namespace "${NAMESPACE}"

echo "=== 2. Generating self-signed TLS certificates ==="
# Generate CA
openssl genrsa -out "${TMP_DIR}/ca.key" 2048 2>/dev/null
openssl req -x509 -new -nodes -key "${TMP_DIR}/ca.key" -subj "/CN=gomemlimit-webhook-ca" -days 3650 -out "${TMP_DIR}/ca.crt" 2>/dev/null

# Generate Server Cert with SAN
cat <<EOF > "${TMP_DIR}/server.conf"
[req]
req_extensions = v3_req
distinguished_name = req_distinguished_name
[req_distinguished_name]
[v3_req]
basicConstraints = CA:FALSE
keyUsage = nonRepudiation, digitalSignature, keyEncipherment
extendedKeyUsage = serverAuth
subjectAltName = @alt_names
[alt_names]
DNS.1 = ${SERVICE_NAME}
DNS.2 = ${SERVICE_NAME}.${NAMESPACE}
DNS.3 = ${SERVICE_NAME}.${NAMESPACE}.svc
DNS.4 = ${SERVICE_NAME}.${NAMESPACE}.svc.cluster.local
EOF

openssl genrsa -out "${TMP_DIR}/tls.key" 2048 2>/dev/null
openssl req -new -key "${TMP_DIR}/tls.key" -subj "/CN=${SERVICE_NAME}.${NAMESPACE}.svc" -out "${TMP_DIR}/server.csr" -config "${TMP_DIR}/server.conf" 2>/dev/null
openssl x509 -req -in "${TMP_DIR}/server.csr" -CA "${TMP_DIR}/ca.crt" -CAkey "${TMP_DIR}/ca.key" -CAcreateserial -out "${TMP_DIR}/tls.crt" -days 3650 -extensions v3_req -extfile "${TMP_DIR}/server.conf" 2>/dev/null

echo "=== 3. Creating TLS Secret in Kubernetes ==="
kubectl -n "${NAMESPACE}" delete secret "${SECRET_NAME}" --ignore-not-found=true
kubectl -n "${NAMESPACE}" create secret tls "${SECRET_NAME}" \
  --cert="${TMP_DIR}/tls.crt" \
  --key="${TMP_DIR}/tls.key"

CA_BUNDLE=$(base64 < "${TMP_DIR}/ca.crt" | tr -d '\n')

echo "=== 4. Applying Manifests with CA Bundle ==="
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
MANIFEST_FILE="${SCRIPT_DIR}/../manifests/webhook.yaml"

sed "s|\${CA_BUNDLE}|${CA_BUNDLE}|g" "${MANIFEST_FILE}" | kubectl apply -f -

echo "=== 5. Deployment Completed ==="
echo "To verify rollout status: kubectl rollout status deployment/${SERVICE_NAME} -n ${NAMESPACE}"

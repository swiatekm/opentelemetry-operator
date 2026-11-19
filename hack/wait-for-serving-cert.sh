#!/usr/bin/env bash
# Waits for cert-manager to issue the operator's webhook serving certificate.
#
# cert-manager sometimes creates two private keys for the certificate at once
# and fails the first certificate request with "CSR not signed by referenced
# private key". It then retries only after a backoff of an hour or more, so the
# operator pod cannot mount the certificate secret and never starts. If the
# certificate is not ready in time, renew it once to request it again now.
set -euo pipefail

# Not read from the environment: chainsaw sets NAMESPACE to the test namespace
# when a test runs make deploy.
NAMESPACE=opentelemetry-operator-system
CERTIFICATE=opentelemetry-operator-serving-cert
CMCTL="${CMCTL:-cmctl}"

if kubectl wait --for=condition=Ready "certificate/${CERTIFICATE}" -n "${NAMESPACE}" --timeout=60s; then
  exit 0
fi

echo "Certificate ${CERTIFICATE} is not ready, renewing it" >&2
kubectl describe "certificate/${CERTIFICATE}" -n "${NAMESPACE}" >&2 || true
kubectl get certificaterequests -n "${NAMESPACE}" >&2 || true
"${CMCTL}" renew "${CERTIFICATE}" -n "${NAMESPACE}"
kubectl wait --for=condition=Ready "certificate/${CERTIFICATE}" -n "${NAMESPACE}" --timeout=120s

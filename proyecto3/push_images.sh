#!/bin/bash
set -e

REGISTRY="34.59.251.34:5000"

echo "=== 1/5 Subiendo go-dep1-client ==="
docker buildx build --output type=image,oci-mediatypes=true,push=true \
  -t ${REGISTRY}/go-dep1-client:v1 -f go_deployment1/Dockerfile.grpc_client .

echo "=== 2/5 Subiendo go-dep2-grpc ==="
docker buildx build --output type=image,oci-mediatypes=true,push=true \
  -t ${REGISTRY}/go-dep2-grpc:v1 -f go_deployment2/Dockerfile.grpc_server .

echo "=== 3/5 Subiendo go-dep2-writer ==="
docker buildx build --output type=image,oci-mediatypes=true,push=true \
  -t ${REGISTRY}/go-dep2-writer:v1 -f go_deployment2/Dockerfile.rabbitmq_writer .

echo "=== 4/5 Subiendo consumer-go ==="
docker buildx build --output type=image,oci-mediatypes=true,push=true \
  -t ${REGISTRY}/consumer-go:v1 -f consumer_go/Dockerfile .

echo "=== 5/5 Subiendo api-rust ==="
docker buildx build --output type=image,oci-mediatypes=true,push=true \
  -t ${REGISTRY}/api-rust:v1 -f api_rust/Dockerfile api_rust/

echo "=== TODAS LAS IMÁGENES PUBLICADAS EN ZOT CON ÉXITO ==="

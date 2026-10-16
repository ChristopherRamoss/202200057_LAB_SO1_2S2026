#!/usr/bin/env bash
# Construye y sube a Zot las imágenes de Go.
# Uso: ./scripts/build-push.sh [etiqueta]     (por defecto: v1)
set -e
REGISTRY=zot.local:5000/mumnk8s
TAG=${1:-v1}

build() {
  name=$1
  dir=$2
  echo ">>> $name"
  docker build -f "$dir/Dockerfile" -t "$REGISTRY/$name:$TAG" .
  docker push "$REGISTRY/$name:$TAG"
}

build go-api          go-deployment1/api
build grpc-client     go-deployment1/grpc-client
build grpc-server     go-deployment2/grpc-server
build rabbit-writer   go-deployment2/rabbit-writer
build consumer        go-consumer

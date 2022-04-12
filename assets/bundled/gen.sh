#!/bin/bash

SRC=$(realpath $(cd -P "$(dirname "${BASH_SOURCE[0]}")" && pwd))

# grab google protobuf definitions
mkdir -p $SRC/google/protobuf
for i in descriptor duration empty timestamp; do
  wget -O $SRC/google/protobuf/$i.proto https://raw.githubusercontent.com/protocolbuffers/protobuf/master/src/google/protobuf/$i.proto
done

# grab google api definitions
mkdir -p $SRC/google/api
for i in annotations http; do
  wget -O $SRC/google/api/$i.proto https://raw.githubusercontent.com/googleapis/googleapis/master/google/api/$i.proto
done

# grab grpc-gateway (protoc-gen-openapiv2) definitions
mkdir -p $SRC/protoc-gen-openapiv2/options
for i in annotations openapiv2; do
  wget -O $SRC/protoc-gen-openapiv2/options/$i.proto https://raw.githubusercontent.com/grpc-ecosystem/grpc-gateway/master/protoc-gen-openapiv2/options/$i.proto
done

# grab xo definitions
wget -O $SRC/xo/xo.proto https://raw.githubusercontent.com/xo/ecosystem/master/proto/xo/xo.proto

#!/bin/bash

set -e

PROTO_DIR=./proto
GEN_DIR=./shared/proto

rm -rf ${GEN_DIR}
mkdir -p ${GEN_DIR}

protoc --proto_path=${PROTO_DIR} \
       --go_out=${GEN_DIR} \
       --go-grpc_out=${GEN_DIR} \
       $(find ${PROTO_DIR} -name '*.proto')

echo "Protobuf files generated"
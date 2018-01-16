#!/bin/bash

VERSION=3.5.1

set -e

[[ -f $HOME/protobuf-$VERSION/bin/protoc ]] && exit 0

wget https://github.com/google/protobuf/releases/download/v$VERSION/protoc-$VERSION-linux-x86_64.zip
echo $PWD
mkdir -p $HOME/protobuf-$VERSION
unzip -q -d $HOME/protobuf-$VERSION protoc-$VERSION-linux-x86_64.zip

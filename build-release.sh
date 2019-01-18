#!/bin/bash

VER=$1
BUILD=$2

if [ -z "$VER" ]; then
  echo "usage: $0 <VER>"
  exit 1
fi

PLATFORM=$(uname|sed -e 's/_.*//'|tr '[:upper:]' '[:lower:]'|sed -e 's/^\(msys\|mingw\).*/windows/')
TAG=$VER
SRC=$(realpath $(cd -P "$( dirname "${BASH_SOURCE[0]}" )" && pwd ))
NAME=$(basename $SRC)
EXT=tar.bz2

if [ -z "$BUILD" ]; then
  BUILD=$SRC/build
fi

DIR=$BUILD/$PLATFORM/$VER
BIN=$DIR/$NAME

TAGS=""

case $PLATFORM in
  windows)
    EXT=zip
    BIN=$BIN.exe
  ;;

  linux|darwin)
  ;;
esac

OUT=$DIR/gunk-$VER-$PLATFORM-amd64.$EXT

echo "PLATFORM: $PLATFORM"
echo "VER: $VER"
echo "DIR: $DIR"
echo "BIN: $BIN"
echo "OUT: $OUT"
echo "TAGS: $TAGS"

set -e

if [ -d $DIR ]; then
  echo "removing $DIR"
  rm -rf $DIR
fi

mkdir -p $DIR

pushd $SRC &> /dev/null

go build \
  -tags "$TAGS" \
  -ldflags="-s -w -X main.version=$VER" \
  -o $BIN

echo -n "checking gunk version: "
BUILT_VER=$($BIN version)
if [ "$BUILT_VER" != "gunk $VER" ]; then
  echo -e "\n\nerror: expected gunk version to report 'gunk $VER', got: '$BUILT_VER'"
  exit 1
fi
echo "$BUILT_VER"

case $PLATFORM in
  linux|windows|darwin)
    echo "stripping $BIN"
    strip $BIN
  ;;
esac

case $PLATFORM in
  linux|windows|darwin)
    echo "packing $BIN"
    upx -q -q $BIN
  ;;
esac

echo "compressing $OUT"
case $EXT in
  tar.bz2)
    tar -C $DIR -cjf $OUT $(basename $BIN)
  ;;
  zip)
    zip $OUT -j $BIN
  ;;
esac

du -sh $OUT

popd &> /dev/null

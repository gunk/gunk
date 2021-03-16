#!/bin/bash

SRC=$(realpath $(cd -P "$(dirname "${BASH_SOURCE[0]}")" && pwd))

set -e

PKGPATH=$GOPATH/src/github.com/grpc-ecosystem/grpc-gateway/internal/httprule

pushd $PKGPATH &> /dev/null
git clean -f -x -d
git reset --hard
git pull
popd &> /dev/null

rm -f $SRC/*.go
FILES=$(ls $PKGPATH/*.go|grep -v test)
cp $FILES $SRC/

GEN=$(cat << END
package httprule

//go:generate ./gen.sh
END
)
echo "$GEN" > $SRC/gen.go

#!/bin/bash
set -ex
this_dir=$(dirname "$script")
if [[ "$this_dir" == "." ]]
then
    this_dir=$(pwd)
fi

# g++ treats /e/xxx as <currentDrive>:/e/xxx
# therefore we go with mixed mode names
this_dir=$(cygpath -m "$this_dir")

export CGO_CFLAGS="-DIBM=1 -O2"
export CGO_CXXFLAGS="-std=c++20 -DIBM=1 -O2 -I${this_dir}/SDK/CHeaders/XPLM"
export CGO_LDFLAGS="-L${this_dir}/SDK/Libraries/Win -lXPLM_64 -static-libgcc -static -lstdc++"
export GOOS=windows
export GOARCH=amd64
export CGO_ENABLED=1
export CC=gcc
export CXX=g++
go build --buildmode c-shared -o build/XA-snow/win.xpl \
		-ldflags="-X github.com/xairline/xa-snow/services.VERSION=${VERSION}" main.go

XPL_DIR=/e/X-Plane-12-test
if [ -d $XPL_DIR/Resources/plugins/XA-snow ]
then
  cp build/XA-snow/win.xpl $XPL_DIR/Resources/plugins/XA-snow/.
fi

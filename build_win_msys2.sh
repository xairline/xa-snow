#!/bin/bash
set -ex
this_dir=$(dirname "$script")
if [[ "$this_dir" == "." ]]
then
    this_dir=$(pwd)
fi

export CGO_CFLAGS="-DIBM=1 -O2"
# for -I an absolute path starting with a / does not work for whatever reasons
# as all c++ code is in services we need the ..
export CGO_CXXFLAGS="-std=c++20 -DIBM=1 -O2 -I../SDK/CHeaders/XPLM"
export CGO_LDFLAGS="-L"${this_dir}"/SDK/Libraries/Win -lXPLM_64 -static-libgcc -static -lstdc++"
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

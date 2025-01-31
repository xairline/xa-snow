#!/bin/bash
set -ex
this_dir=$(dirname "$script")
if [[ "$this_dir" == "." ]]
then
    this_dir=$(pwd)
fi

export CGO_CFLAGS="-DLIN=1 -O2"
export CGO_CXXFLAGS="-std=c++20 -DLIN=1 -O2 -I${this_dir}/SDK/CHeaders/XPLM"
export CGO_LDFLAGS="-shared -rdynamic -nodefaultlibs"
export CGO_ENABLED=1
go build --buildmode c-shared -o build/XA-snow/lin.xpl \
		-ldflags="-X github.com/xairline/xa-snow/services.VERSION=${VERSION}" main.go

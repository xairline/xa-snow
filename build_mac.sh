#!/bin/bash
set -ex
this_dir=$(dirname "$script")
if [[ "$this_dir" == "." ]]
then
    this_dir=$(pwd)
fi

export CGO_CFLAGS="-DAPL=1 -O2"
export CGO_CXXFLAGS="-std=c++20 -DAPL=1 -O2 -I${this_dir}/SDK/CHeaders/XPLM"
export CGO_LDFLAGS="-F/System/Library/Frameworks/ -F${this_dir}/SDK/Libraries/Mac -framework XPLM"
export GOOS=darwin
export GOARCH=arm64
export CGO_ENABLED=1
go build --buildmode c-shared -o build/XA-snow/mac_arm.xpl \
		-ldflags="-X github.com/xairline/xa-snow/services.VERSION=${VERSION}" main.go

export GOARCH=amd64
export CGO_ENABLED=1
go build --buildmode c-shared -o build/XA-snow/mac_x86.xpl \
    -ldflags="-X github.com/xairline/xa-snow/services.VERSION=${VERSION}" main.go

lipo build/XA-snow/mac_arm.xpl build/XA-snow/mac_x86.xpl -create -output build/XA-snow/mac.xpl

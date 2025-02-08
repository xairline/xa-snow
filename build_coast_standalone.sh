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

export CGO_CFLAGS="-DIBM=1 -DLOCAL_DEBUGSTRING -O2 -Iservices/"
export CGO_CXXFLAGS="-std=c++20 -DIBM=1 -DLOCAL_DEBUGSTRING -Wall -O2 -Iservices -I${this_dir}/SDK/CHeaders/XPLM"
export CGO_LDFLAGS="-L${this_dir}/SDK/Libraries/Win -lXPLM_64 -l:libpng.a -l:libz.a"
export GOOS=windows
export GOARCH=amd64
export CGO_ENABLED=1
export CC=gcc
export CXX=g++
go build coast_standalone.go

mkfile_path := $(abspath $(lastword $(MAKEFILE_LIST)))
current_dir := $(notdir $(patsubst %/,%,$(dir $(mkfile_path))))

clean:
	rm -r dist || true || rm ~/X-Plane\ 12/Resources/plugins/XA-snow/mac.xpl
mac:
	GOOS=darwin \
	GOARCH=arm64 \
	CGO_ENABLED=1 \
	CGO_CFLAGS="-DAPL=1 -DIBM=0 -DLIN=0 -O2 -g" \
	CGO_CXXFLAGS="-std=c++20 -DAPL=1 -O2 -I${CURDIR}/SDK/CHeaders/XPLM" \
	CGO_LDFLAGS="-F/System/Library/Frameworks/ -F${CURDIR}/Libraries/Mac -framework XPLM" \
	go build -buildmode c-shared -o build/XA-snow/mac_arm.xpl \
		-ldflags="-X github.com/xairline/xa-snow/services.VERSION=${VERSION}"  main.go
	GOOS=darwin \
	GOARCH=amd64 \
	CGO_ENABLED=1 \
	CGO_CFLAGS="-DAPL=1 -DIBM=0 -DLIN=0 -O2 -g" \
	CGO_CXXFLAGS="-std=c++20 -DAPL=1 -O2 -I${CURDIR}/SDK/CHeaders/XPLM" \
	CGO_LDFLAGS="-F/System/Library/Frameworks/ -F${CURDIR}/Libraries/Mac -framework XPLM" \
	go build -buildmode c-shared -o build/XA-snow/mac_amd.xpl \
		-ldflags="-X github.com/xairline/xa-snow/services.VERSION=${VERSION}" main.go
	lipo build/XA-snow/mac_arm.xpl build/XA-snow/mac_amd.xpl -create -output build/XA-snow/mac.xpl
dev:
	GOOS=darwin \
	GOARCH=arm64 \
	CGO_ENABLED=1 \
	CGO_CFLAGS="-DAPL=1 -DIBM=0 -DLIN=0 -O2 -g" \
	CGO_LDFLAGS="-F/System/Library/Frameworks/ -F${CURDIR}/Libraries/Mac -framework XPLM" \
	go build -buildmode c-shared -o ~/X-Plane\ 12/Resources/plugins/XA-snow/mac.xpl \
		-ldflags="-X github.com/xairline/xa-snow/services.VERSION=${VERSION}" main.go
win:
	CGO_CFLAGS="-DIBM=1 -static -O2 -g" \
	CGO_CXXFLAGS="-std=c++20 -DIBM=1 -O2 -I${CURDIR}/SDK/CHeaders/XPLM" \
	CGO_LDFLAGS="-L${CURDIR}/Libraries/Win -lXPLM_64 -static-libgcc -static-libstdc++ -Wl,--exclude-libs,ALL" \
	GOOS=windows \
	GOARCH=amd64 \
	CGO_ENABLED=1 \
	CC=x86_64-w64-mingw32-gcc \
	CXX=x86_64-w64-mingw32-g++ \
	go build --buildmode c-shared -o build/XA-snow/win.xpl \
		-ldflags="-X github.com/xairline/xa-snow/services.VERSION=${VERSION}"  main.go
lin:
	GOOS=linux \
	GOARCH=amd64 \
	CGO_ENABLED=1 \
	CC=/usr/local/bin/x86_64-linux-musl-cc \
	CGO_CFLAGS="-DLIN=1 -O2 -g" \
	CGO_CXXFLAGS="-std=c++20 -DLIN=1 -O2 -I${CURDIR}/SDK/CHeaders/XPLM" \
	CGO_LDFLAGS="-shared -rdynamic -nodefaultlibs" \
	go build -buildmode c-shared -o build/XA-snow/lin.xpl  \
		-ldflags="-X github.com/xairline/xa-snow/services.VERSION=${VERSION}" main.go

all: mac win lin
mac-test:
	GOOS=darwin \
	GOARCH=arm64 \
	CGO_ENABLED=1 \
	CGO_CFLAGS="-DAPL=1 -DIBM=0 -DLIN=0 -O2 -g" \
	CGO_CXXFLAGS="-std=c++20 -DAPL=1 -O2 -I${CURDIR}/SDK/CHeaders/XPLM" \
	CGO_LDFLAGS="-F/System/Library/Frameworks/ -F${CURDIR}/Libraries/Mac -framework XPLM" \
	go test -race -coverprofile=coverage.txt -covermode=atomic ./... -v

# build on Windows msys2/mingw64
PLUG_DIR=$(XPL_ROOT)/Resources/plugins/XA-snow

msys2:
	@if [ -z "$(XPL_ROOT)" ]; then echo "Environment is not setup"; exit 1; fi
	go build --buildmode c-shared -o build/XA-snow/win.xpl \
		-ldflags="-X github.com/xairline/xa-snow/services.VERSION=${VERSION}" main.go
	[ -d "$(PLUG_DIR)" ] && cp -p build/XA-snow/win.xpl "$(PLUG_DIR)/."

msys2-test:
	@if [ -z "$(XPL_ROOT)" ]; then echo "Environment is not setup"; exit 1; fi
	go test -race -coverprofile=coverage.txt -covermode=atomic ./... -v

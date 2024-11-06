#
# set up environment for msys2
#
export XPL_ROOT=/E/X-Plane-12-test

script="$BASH_SOURCE"
[ -z "$BASH_SOURCE" ] && script="$0"

this_dir=$(dirname "$script")
if [[ "$this_dir" == "." ]]
then
    this_dir=$(pwd)
fi

export CGO_CFLAGS="-DIBM=1 -O2 -g"
export CGO_LDFLAGS="-L"$this_dir"/Libraries/Win -lXPLM_64"
export GOOS=windows
export GOARCH=amd64
export CGO_ENABLED=1
export CC=gcc
export CXX=g++

# must X Plane must be in PATH in order to find XPLM_64.dll for standalone tests
if [[ ! "$PATH" =~ "$XPL_ROOT" ]]
then
    export PATH="$PATH:$XPL_ROOT/Resources/plugins"
fi

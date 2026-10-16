#!/bin/sh
#
# build.sh compiles MyTools for the platforms it ships on.
#
#	scripts/build.sh                  every target, into dist/
#	scripts/build.sh -p linux/amd64   one target
#	scripts/build.sh --host           this machine only
#	scripts/build.sh -v 1.2.3         stamp a version
#
# MyGo draws its window through purego and never imports cgo, so one machine
# cross-compiles the lot: the only thing needed is a Go toolchain.
set -eu

name=mytools
pkg=./cmd/mytools
all_targets="linux/amd64 linux/arm64 darwin/amd64 darwin/arm64 windows/amd64 windows/arm64"

out=dist
version=
platforms=
archive=1
debug=0
hostonly=0

usage() {
	cat <<'EOF'
Builds MyTools for every platform it ships on.

Usage: scripts/build.sh [options]

Options:
  -v, --version VER    version stamped into the binary (default: the git
                       tag, else git describe, else 0.1.0)
  -o, --out DIR        where the builds go (default: dist)
  -p, --platform LIST  GOOS/GOARCH targets, comma separated, e.g.
                       linux/amd64,windows/arm64, or "all" (default: all)
  -a, --host           build for this machine only
  -d, --debug          keep development features and symbols, instead of a
                       release build
  -n, --no-archive     do not pack the tar.gz archives
  -h, --help           this text

Every target gets its own directory, OUT/<os>-<arch>/, holding the binary
and the resources it reads, and, unless -n, is packed as
OUT/<name>-<version>-<os>-<arch>.tar.gz.
EOF
}

while [ "$#" -gt 0 ]; do
	case $1 in
	-v | --version)
		[ "$#" -ge 2 ] || { echo "build.sh: $1 needs a value" >&2; exit 2; }
		version=$2
		shift 2
		;;
	-o | --out)
		[ "$#" -ge 2 ] || { echo "build.sh: $1 needs a value" >&2; exit 2; }
		out=$2
		shift 2
		;;
	-p | --platform)
		[ "$#" -ge 2 ] || { echo "build.sh: $1 needs a value" >&2; exit 2; }
		platforms=$2
		shift 2
		;;
	-a | --host)
		hostonly=1
		shift
		;;
	-d | --debug)
		debug=1
		shift
		;;
	-n | --no-archive)
		archive=0
		shift
		;;
	-h | --help)
		usage
		exit 0
		;;
	*)
		echo "build.sh: unknown option: $1" >&2
		usage >&2
		exit 2
		;;
	esac
done

here=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
root=$(dirname -- "$here")

# The version the About panel shows. A build that names itself after its
# commit is more useful than one that always says 0.1.0.
if [ -z "$version" ]; then
	if version=$(git -C "$root" describe --tags --always --dirty 2>/dev/null); then
		version=${version#v}
	else
		version=0.1.0
	fi
fi

if [ "$hostonly" -eq 1 ]; then
	platforms=$(go env GOOS)/$(go env GOARCH)
elif [ -z "$platforms" ] || [ "$platforms" = all ]; then
	platforms=$all_targets
fi

# Take both the comma separated list -p documents and plain arguments.
platforms=$(printf '%s' "$platforms" | tr ',' ' ')

mkdir -p "$out"
out=$(CDPATH= cd -- "$out" && pwd)

cd "$root"

for target in $platforms; do
	case $target in
	*/*) ;;
	*)
		echo "build.sh: target must be GOOS/GOARCH, not $target" >&2
		exit 2
		;;
	esac

	os=${target%%/*}
	arch=${target#*/}
	bin=$name
	if [ "$os" = windows ]; then
		bin=$name.exe
	fi

	echo "==> $target"
	dir=$out/$os-$arch
	rm -rf "$dir"
	mkdir -p "$dir"

	ldflags="-X main.version=$version"
	tags=
	if [ "$debug" -eq 0 ]; then
		ldflags="-s -w $ldflags"
		tags="-tags mygo_noinspector"
	fi
	if [ "$os" = windows ]; then
		# Without this the app opens a console window beside itself.
		ldflags="-H=windowsgui $ldflags"
	fi

	# shellcheck disable=SC2086 # $tags is a flag list, empty when debug.
	GOOS=$os GOARCH=$arch CGO_ENABLED=0 \
		go build -trimpath $tags -ldflags "$ldflags" -o "$dir/$bin" "$pkg"

	# At run time MyGo reads resources next to the executable.
	if [ -d resources ]; then
		cp -R resources "$dir/"
	fi

	if [ "$archive" -eq 1 ]; then
		tar -czf "$out/$name-$version-$os-$arch.tar.gz" -C "$out" "$os-$arch"
	fi
done

echo "mytools $version built for: $platforms"
echo "in $out"

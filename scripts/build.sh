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
and the resources it reads — on macOS a double-clickable <name>.app bundle
instead of a bare executable — and, unless -n, is packed as
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
	exe=$dir/$bin

	# Finder runs a bare executable in a Terminal window; only a bundle
	# opens as an application. So on macOS the build goes into
	# <name>.app/Contents/MacOS, and its resources into Contents/Resources,
	# which is where MyGo looks for them there.
	app=
	if [ "$os" = darwin ]; then
		app=$dir/$name.app
		mkdir -p "$app/Contents/MacOS" "$app/Contents/Resources"
		exe=$app/Contents/MacOS/$name
	fi

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
		go build -trimpath $tags -ldflags "$ldflags" -o "$exe" "$pkg"

	# At run time MyGo reads resources next to the executable, or from
	# Contents/Resources in a bundle.
	if [ -d resources ]; then
		if [ -n "$app" ]; then
			cp -R resources/. "$app/Contents/Resources/"
		else
			cp -R resources "$dir/"
		fi
	fi

	if [ -n "$app" ]; then
		cat >"$app/Contents/Info.plist" <<EOF
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>CFBundleName</key><string>$name</string>
	<key>CFBundleDisplayName</key><string>$name</string>
	<key>CFBundleExecutable</key><string>$name</string>
	<key>CFBundleIdentifier</key><string>com.yinshuwei.$name</string>
	<key>CFBundleIconFile</key><string>AppIcon</string>
	<key>CFBundleVersion</key><string>$version</string>
	<key>CFBundleShortVersionString</key><string>$version</string>
	<key>CFBundlePackageType</key><string>APPL</string>
	<key>CFBundleInfoDictionaryVersion</key><string>6.0</string>
	<key>LSMinimumSystemVersion</key><string>12.0</string>
	<key>NSHighResolutionCapable</key><true/>
	<key>NSPrincipalClass</key><string>NSApplication</string>
</dict>
</plist>
EOF
		printf 'APPL????' >"$app/Contents/PkgInfo"

		# The icon the bundle names, as an .icns. Both tools ship with
		# macOS; a cross build on another system keeps icon.png and the
		# default icon.
		if [ -f resources/icon.png ] &&
			command -v iconutil >/dev/null 2>&1 &&
			command -v sips >/dev/null 2>&1; then
			iconset=$(mktemp -d)/AppIcon.iconset
			mkdir -p "$iconset"
			for size in 16 32 128 256 512; do
				dbl=$((size * 2))
				sips -z "$size" "$size" resources/icon.png \
					--out "$iconset/icon_${size}x${size}.png" >/dev/null
				sips -z "$dbl" "$dbl" resources/icon.png \
					--out "$iconset/icon_${size}x${size}@2x.png" >/dev/null
			done
			iconutil -c icns "$iconset" -o "$app/Contents/Resources/AppIcon.icns" ||
				echo "build.sh: iconutil failed, keeping the default icon" >&2
			rm -rf "$(dirname "$iconset")"
		fi

		# An ad hoc signature is what lets the bundle start on Apple
		# silicon without a Developer ID. To ship it, sign with an
		# identity instead (see `go tool mygo build -sign`).
		if command -v codesign >/dev/null 2>&1; then
			codesign --force --sign - "$app" >/dev/null 2>&1 ||
				echo "build.sh: codesign failed, the bundle is unsigned" >&2
		fi
	fi

	if [ "$archive" -eq 1 ]; then
		tar -czf "$out/$name-$version-$os-$arch.tar.gz" -C "$out" "$os-$arch"
	fi
done

echo "mytools $version built for: $platforms"
echo "in $out"

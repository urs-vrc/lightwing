#!/bin/sh
# Based on Deno installer: Copyright 2019 the Deno authors. All rights reserved. MIT license.
# TODO(everyone): Keep this script simple and easily auditable.

# Script will install the latest Encore release by default.
# If a version is provided, it will install the specified version.
# Example: curl -L https://encore.dev/install.sh | bash -s -- 1.50.0

set -e

version=$1

case $(uname -sm) in
	"Darwin x86_64") target="darwin_amd64" ;;
	"Darwin arm64")  target="darwin_arm64" ;;
	"Darwin arm64")  target="darwin_arm64" ;;
	"Linux aarch64") target="linux_arm64"  ;;
	"Linux arm64")   target="linux_arm64"  ;;
	*) target="linux_amd64" ;;
esac

if [ -z "$version" ]; then
  encore_uri=$(curl -sSf -N "https://encore.dev/api/releases?target=${target}&show=url")
  if [ ! "$encore_uri" ]; then
    echo "Error: Unable to determine latest Encore release." 1>&2
    exit 1
  fi
else
  encore_uri="https://d2f391esomvqpi.cloudfront.net/encore-${version}-${target}.tar.gz"
fi

encore_install="${ENCORE_INSTALL:-$HOME/.encore}"

bin_dir="$encore_install/bin"
exe="$bin_dir/encore"
tar="$encore_install/encore.tar.gz"

if [ ! -d "$bin_dir" ]; then
	mkdir -p "$bin_dir"
fi

curl --fail --location --progress-bar --output "$tar" "$encore_uri"
cd "$encore_install"

# If encore-go already exists, delete it.
# Merging multiple Go releases into the same directory
# leads to very difficult-to-understand fatal errors.
if [ -d "./encore-go" ]; then
	rm -rf "./encore-go"
fi

# Same goes for runtime
if [ -d "./runtimes" ]; then
	rm -rf "./runtimes"
fi

tar -C "$encore_install" -xzf "$tar"
chmod +x "$bin_dir"/*
rm "$tar"

"$exe" version

echo "Encore was installed successfully to $exe"
if command -v encore >/dev/null; then
	echo "Run 'encore --help' to get started"
else
	case $SHELL in
	/bin/zsh) shell_profile=".zshrc" ;;
	*) shell_profile=".bash_profile" ;;
	esac
	echo "Manually add the directory to your \$HOME/$shell_profile (or similar)"
	echo "  export ENCORE_INSTALL=\"$encore_install\""
	echo "  export PATH=\"\$ENCORE_INSTALL/bin:\$PATH\""
	echo "Run '$exe --help' to get started"
fi

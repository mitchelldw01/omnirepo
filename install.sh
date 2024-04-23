#!/usr/bin/env bash

set -e

GITHUB_REPO="mitchelldw01/omnirepo"
INSTALL_PATH="/usr/local/bin"
RELEASE_TAG=$(curl -s https://api.github.com/repos/$GITHUB_REPO/releases/latest | grep '"tag_name":' | awk -F '"' '{print $4}')

OS=$(uname | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)

case $ARCH in
    arm64)
        ARCH="arm64"
        ;;
    aarch64)
        ARCH="arm64"
        ;;
    x86_64)
        ARCH="x86_64"
        ;;
    i686)
        ARCH="i386"
        ;;
    *)
        echo "Unsupported architecture: $ARCH"
        exit 1
        ;;
esac

if [[ "$OS" != "darwin" && "$OS" != "linux" ]]; then
    echo "Unsupported OS: $OS"
    exit 1
fi

FILENAME="omnirepo_${OS}_${ARCH}.tar.gz"
BINARY_URL="https://github.com/$GITHUB_REPO/releases/download/$RELEASE_TAG/$FILENAME"

TEMP_DIR=$(mktemp -d)
cd "$TEMP_DIR"

curl -L -o "$FILENAME" "$BINARY_URL"
tar -zxf "$FILENAME"
sudo mv omni "$INSTALL_PATH"

cd - > /dev/null
rm -r "$TEMP_DIR"
printf '\033[1;32mSuccess!\033[0m Run '"'"'omni --help'"'"' to verify installation.\n'

#!/bin/bash
# Build xxk-proxy for OpenWrt with UPX compression
# Usage: ./build.sh [target_arch]
#   target_arch: mipsle, mips, arm, x86_64 (default: mipsle)

set -e

TARGET_ARCH="${1:-mipsle}"
BUILD_DIR="$(cd "$(dirname "$0")" && pwd)"
SRC_DIR="$BUILD_DIR/package/xxk-proxy/src"
OUTPUT_DIR="$BUILD_DIR/output"

echo "============================================"
echo "Building xxk-proxy for OpenWrt"
echo "Target architecture: $TARGET_ARCH"
echo "============================================"

# Set Go environment variables based on target architecture
case "$TARGET_ARCH" in
    mips)
        GOARCH=mips
        GOOS=linux
        GOMIPS=softfloat
        ;;
    mips-be)
        GOARCH=mips
        GOOS=linux
        GOMIPS=hardfloat
        ;;
    arm)
        GOARCH=arm
        GOOS=linux
        GOARM=7
        ;;
    x86_64)
        GOARCH=amd64
        GOOS=linux
        ;;
    *)
        echo "Unknown architecture: $TARGET_ARCH"
        echo "Supported: mipsle, mips, arm, x86_64"
        exit 1
        ;;
esac

echo "GOARCH=$GOARCH"
echo "GOOS=$GOOS"
echo "GOMIPS=$GOMIPS"
echo "GOARM=$GOARM"

# Create output directory
mkdir -p "$OUTPUT_DIR"

# Build
cd "$SRC_DIR"
export GOARCH GOOS GOMIPS GOARM
export GOPROXY=https://goproxy.cn,direct
export CGO_ENABLED=0

echo ""
echo "Compiling..."
go build -trimpath -ldflags="-s -w" -o xxk-proxy .

# Check result
if [ ! -f "$SRC_DIR/xxk-proxy" ]; then
    echo "Build failed!"
    exit 1
fi

RAW_SIZE=$(stat -c%s "$SRC_DIR/xxk-proxy" 2>/dev/null || echo "unknown")
echo "Raw binary size: $RAW_SIZE bytes"

# UPX compression
if command -v upx &> /dev/null; then
    echo ""
    echo "Compressing with UPX..."
    upx --best --lzma "$SRC_DIR/xxk-proxy" -o "$OUTPUT_DIR/xxk-proxy-$TARGET_ARCH" 2>&1
else
    echo ""
    echo "UPX not found, copying uncompressed binary..."
    cp "$SRC_DIR/xxk-proxy" "$OUTPUT_DIR/xxk-proxy-$TARGET_ARCH"
fi

# Check result
if [ -f "$OUTPUT_DIR/xxk-proxy-$TARGET_ARCH" ]; then
    FINAL_SIZE=$(stat -c%s "$OUTPUT_DIR/xxk-proxy-$TARGET_ARCH" 2>/dev/null || echo "unknown")
    echo ""
    echo "Build successful!"
    echo "Binary: $OUTPUT_DIR/xxk-proxy-$TARGET_ARCH"
    echo "Final size: $FINAL_SIZE bytes"
    
    # Show binary info
    file "$OUTPUT_DIR/xxk-proxy-$TARGET_ARCH"
else
    echo "Build failed!"
    exit 1
fi

echo ""
echo "Done. Binary is ready for OpenWrt."
echo ""
echo "To install on OpenWrt:"
echo "  1. Copy the binary to your OpenWrt device:"
echo "     scp $OUTPUT_DIR/xxk-proxy-$TARGET_ARCH root@openwrt:/usr/bin/xxk-proxy"
echo ""
echo "  2. Copy init script and config:"
echo "     scp $BUILD_DIR/package/xxk-proxy/files/xxk-proxy.init root@openwrt:/etc/init.d/xxk-proxy"
echo "     scp $BUILD_DIR/package/xxk-proxy/files/xxk-proxy.config root@openwrt:/etc/config/xxk-proxy"
echo ""
echo "  3. Enable and start the service:"
echo "     chmod +x /usr/bin/xxk-proxy /etc/init.d/xxk-proxy"
echo "     /etc/init.d/xxk-proxy enable"
echo "     /etc/init.d/xxk-proxy start"
#!/bin/bash

# CI/CD 构建脚本
# 用法: ./build_ci.sh <frontend_dist_dir> <version> <platform> <arch>
# 示例: ./build_ci.sh /path/to/dist 1.0.0 linux amd64

set -e

FRONTEND_DIST_DIR="$1"
VERSION="$2"
PLATFORM="$3"
ARCH="$4"

if [ -z "$FRONTEND_DIST_DIR" ] || [ -z "$VERSION" ] || [ -z "$PLATFORM" ] || [ -z "$ARCH" ]; then
    echo "Usage: $0 <frontend_dist_dir> <version> <platform> <arch>"
    echo "Example: $0 /path/to/dist 1.0.0 linux amd64"
    exit 1
fi

echo "=========================================="
echo "CI/CD Build Script"
echo "=========================================="
echo "Frontend dist directory: $FRONTEND_DIST_DIR"
echo "Version: $VERSION"
echo "Platform: $PLATFORM"
echo "Architecture: $ARCH"
echo "=========================================="

# 获取脚本所在目录（backend 根目录）
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$SCRIPT_DIR"

echo "Backend directory: $SCRIPT_DIR"

# 复制前端构建文件到 public 目录
echo ""
echo "Copying frontend build files to public directory..."
# 清空 public 目录（如果存在）以确保干净的状态
if [ -d "public" ]; then
    rm -rf public/*
fi
mkdir -p public
if [ -d "$FRONTEND_DIST_DIR" ] && [ "$(ls -A "$FRONTEND_DIST_DIR" 2>/dev/null)" ]; then
    cp -r "$FRONTEND_DIST_DIR"/* public/
    echo "Frontend files copied successfully"
else
    echo "Error: Frontend dist directory is empty or does not exist: $FRONTEND_DIST_DIR"
    exit 1
fi

# 验证 public 目录
echo ""
echo "Verifying public directory..."
if [ ! -d "public" ]; then
    echo "Error: public directory not found"
    exit 1
fi

if [ ! -f "public/index.html" ]; then
    echo "Error: public/index.html not found"
    exit 1
fi

FILE_COUNT=$(find public -type f | wc -l)
echo "Found $FILE_COUNT files in public directory"

if [ "$FILE_COUNT" -eq 0 ]; then
    echo "Error: public directory is empty"
    exit 1
fi

echo "Public directory verification passed"

# 更新版本号
if [ -f "config/app.go" ]; then
    echo ""
    echo "Updating version in config/app.go..."
    if [[ "$OSTYPE" == "darwin"* ]]; then
        # macOS
        sed -i '' "s/\"version\": \".*\"/\"version\": \"$VERSION\"/" config/app.go
    else
        # Linux
        sed -i "s/\"version\": \".*\"/\"version\": \"$VERSION\"/" config/app.go
    fi
    echo "Version updated to: $VERSION"
fi

# 设置构建环境变量
export GOOS="$PLATFORM"
export GOARCH="$ARCH"
export CGO_ENABLED=0

# 确定输出文件名
if [ "$PLATFORM" = "windows" ]; then
    BINARY_NAME="dashboard-${PLATFORM}-${ARCH}.exe"
    OUTPUT_NAME="dashboard.exe"
    ARCHIVE_NAME="dashboard-${PLATFORM}-${ARCH}.zip"
    ARCHIVE_CMD="zip"
else
    BINARY_NAME="dashboard-${PLATFORM}-${ARCH}"
    OUTPUT_NAME="dashboard"
    ARCHIVE_NAME="dashboard-${PLATFORM}-${ARCH}.tar.gz"
    ARCHIVE_CMD="tar -czf"
fi

# 构建
echo ""
echo "Building backend with production tag..."
echo "GOOS=$GOOS GOARCH=$GOARCH CGO_ENABLED=$CGO_ENABLED"
echo "Command: go build -tags production -ldflags \"-s -w\" -trimpath -o $OUTPUT_NAME ."

go build -tags production -ldflags "-s -w" -trimpath -o "$OUTPUT_NAME" .

if [ ! -f "$OUTPUT_NAME" ]; then
    echo "Error: Build failed - $OUTPUT_NAME not found"
    exit 1
fi

echo "Build successful!"

# 重命名二进制文件
if [ -f "$BINARY_NAME" ]; then
    rm "$BINARY_NAME"
fi
mv "$OUTPUT_NAME" "$BINARY_NAME"
echo "Binary renamed to: $BINARY_NAME"

# 打包
echo ""
echo "Creating archive..."
if [ -f "$ARCHIVE_NAME" ]; then
    rm "$ARCHIVE_NAME"
fi

if [ "$PLATFORM" = "windows" ]; then
    if command -v zip &> /dev/null; then
        zip "$ARCHIVE_NAME" "$BINARY_NAME"
    elif command -v 7z &> /dev/null; then
        7z a "$ARCHIVE_NAME" "$BINARY_NAME"
    else
        echo "Error: zip or 7z command not found"
        exit 1
    fi
else
    tar -czf "$ARCHIVE_NAME" -C . "$BINARY_NAME"
fi

if [ ! -f "$ARCHIVE_NAME" ]; then
    echo "Error: Failed to create archive"
    exit 1
fi

echo "Archive created: $ARCHIVE_NAME"

# 生成 SHA256
echo ""
echo "Generating SHA256 checksum..."
SHA256_FILE="${ARCHIVE_NAME}.sha256"
if command -v sha256sum &> /dev/null; then
    sha256sum "$ARCHIVE_NAME" > "$SHA256_FILE"
elif command -v shasum &> /dev/null; then
    shasum -a 256 "$ARCHIVE_NAME" > "$SHA256_FILE"
else
    echo "Warning: sha256sum or shasum not found, skipping checksum generation"
fi

if [ -f "$SHA256_FILE" ]; then
    echo "SHA256 file generated: $SHA256_FILE"
fi

echo ""
echo "=========================================="
echo "Build completed successfully!"
echo "=========================================="
echo "Files created:"
echo "  - $BINARY_NAME"
echo "  - $ARCHIVE_NAME"
if [ -f "$SHA256_FILE" ]; then
    echo "  - $SHA256_FILE"
fi
echo "=========================================="

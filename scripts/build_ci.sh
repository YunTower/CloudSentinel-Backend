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

# 详细列出 public 目录结构
echo ""
echo "Public directory structure:"
find public -type f | head -20
echo "..."

# 验证关键文件
REQUIRED_FILES=("public/index.html")
for file in "${REQUIRED_FILES[@]}"; do
    if [ ! -f "$file" ]; then
        echo "Error: Required file not found: $file"
        exit 1
    fi
done

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

# 构建前再次验证 public 目录
echo ""
echo "=========================================="
echo "Final verification before build"
echo "=========================================="
echo "Current directory: $(pwd)"
echo "Public directory exists: $([ -d "public" ] && echo "YES" || echo "NO")"

if [ ! -d "public" ]; then
    echo "ERROR: public directory does not exist!"
    exit 1
fi

echo "Public directory absolute path: $(cd public && pwd)"
echo "File count in public: $(find public -type f | wc -l)"
echo "Directory count in public: $(find public -type d | wc -l)"

if [ ! -f "public/index.html" ]; then
    echo "ERROR: public/index.html not found!"
    exit 1
fi

echo "Sample files in public:"
find public -type f | head -5

# 验证 embed.go 文件存在且包含正确的 build tag
if [ ! -f "embed.go" ]; then
    echo "ERROR: embed.go not found in current directory!"
    exit 1
fi

if ! grep -q "//go:build production" embed.go; then
    echo "ERROR: embed.go does not have production build tag!"
    exit 1
fi

if ! grep -q "//go:embed public" embed.go; then
    echo "ERROR: embed.go does not have //go:embed public directive!"
    exit 1
fi

echo "✓ embed.go validation passed"

# 验证 build tag 会被应用
echo ""
echo "Verifying build configuration..."
echo "Checking which files will be included with production tag:"
INCLUDED_FILES=$(go list -tags production -f '{{range .GoFiles}}{{.}} {{end}}' . 2>/dev/null)
echo "$INCLUDED_FILES"

if echo "$INCLUDED_FILES" | grep -q "embed.go"; then
    echo "✓ embed.go will be included in build with production tag"
else
    echo "✗ ERROR: embed.go will NOT be included in build!"
    echo "This means the build will use embed_dev.go instead (empty PublicFiles)"
    echo "Checking without tags..."
    WITHOUT_TAG=$(go list -f '{{range .GoFiles}}{{.}} {{end}}' . 2>/dev/null)
    if echo "$WITHOUT_TAG" | grep -q "embed.go"; then
        echo "✗ ERROR: embed.go is included WITHOUT production tag (this is wrong!)"
        echo "Files without tag: $WITHOUT_TAG"
    fi
    echo ""
    echo "This will cause 404 errors at runtime because PublicFiles will be empty!"
    exit 1
fi

# 检查 embed_dev.go 是否会被排除
if echo "$INCLUDED_FILES" | grep -q "embed_dev.go"; then
    echo "✗ ERROR: embed_dev.go is included with production tag (this is wrong!)"
    exit 1
else
    echo "✓ embed_dev.go correctly excluded with production tag"
fi

# 构建
echo ""
echo "=========================================="
echo "Building backend"
echo "=========================================="
echo "GOOS=$GOOS"
echo "GOARCH=$GOARCH"
echo "CGO_ENABLED=$CGO_ENABLED"
echo "Build tags: production"
echo "Command: go build -tags production -ldflags \"-s -w\" -trimpath -o $OUTPUT_NAME ."
echo ""

# 执行构建，捕获所有输出并显示
echo "Starting build..."
BUILD_OUTPUT=$(go build -tags production -ldflags "-s -w" -trimpath -o "$OUTPUT_NAME" . 2>&1)
BUILD_EXIT_CODE=$?

if [ $BUILD_EXIT_CODE -ne 0 ]; then
    echo ""
    echo "=========================================="
    echo "ERROR: Build failed!"
    echo "=========================================="
    echo "$BUILD_OUTPUT"
    echo ""
    echo "This could mean:"
    echo "  1. public directory is empty or missing files"
    echo "  2. embed.go validation failed (check embed.go init() function)"
    echo "  3. Go compilation error"
    echo ""
    echo "Checking public directory one more time..."
    if [ -d "public" ]; then
        echo "Public directory exists with $(find public -type f | wc -l) files"
        ls -la public/ | head -10
    else
        echo "Public directory does NOT exist!"
    fi
    exit 1
fi

# 显示构建输出（如果有警告或信息）
if [ -n "$BUILD_OUTPUT" ]; then
    echo "Build output:"
    echo "$BUILD_OUTPUT"
fi

if [ ! -f "$OUTPUT_NAME" ]; then
    echo "Error: Build failed - $OUTPUT_NAME not found"
    exit 1
fi

echo "Build successful!"

# 验证构建结果
BINARY_SIZE=$(stat -f%z "$OUTPUT_NAME" 2>/dev/null || stat -c%s "$OUTPUT_NAME" 2>/dev/null || echo "0")
echo ""
echo "Binary size: $BINARY_SIZE bytes ($(numfmt --to=iec-i --suffix=B $BINARY_SIZE 2>/dev/null || echo "N/A"))"

# 尝试使用 strings 命令检查二进制文件中是否包含前端文件的内容
echo ""
echo "Verifying embedded files in binary..."
if command -v strings &> /dev/null; then
    # 检查是否包含 index.html 的典型内容
    if strings "$OUTPUT_NAME" | grep -q "<!DOCTYPE html" || strings "$OUTPUT_NAME" | grep -q "<html"; then
        echo "✓ Found HTML content in binary (frontend likely embedded)"
    else
        echo "✗ WARNING: No HTML content found in binary"
    fi
    
    # 检查是否包含前端资源路径
    if strings "$OUTPUT_NAME" | grep -q "assets/" || strings "$OUTPUT_NAME" | grep -q "/assets/"; then
        echo "✓ Found assets path in binary"
    else
        echo "✗ WARNING: No assets path found in binary"
    fi
else
    echo "strings command not available, skipping content verification"
fi

# 检查文件大小是否合理
if [ "$BINARY_SIZE" -lt 5000000 ]; then
    echo ""
    echo "⚠️  WARNING: Binary size is very small ($BINARY_SIZE bytes)"
    echo "   This suggests frontend files may NOT be embedded"
    echo "   Expected size should be > 5MB if frontend is embedded"
    echo ""
    echo "   Possible causes:"
    echo "   1. embed.go was not included in build (wrong build tag?)"
    echo "   2. public directory was empty during build"
    echo "   3. Go embed failed silently"
else
    echo "✓ Binary size looks reasonable for embedded frontend"
fi

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
# SHA256 文件名应该与工作流中期望的一致（不包含 .tar.gz 或 .zip 后缀）
SHA256_FILE="dashboard-${PLATFORM}-${ARCH}.sha256"

if command -v sha256sum &> /dev/null; then
    sha256sum "$ARCHIVE_NAME" > "$SHA256_FILE"
    echo "SHA256 file generated: $SHA256_FILE"
elif command -v shasum &> /dev/null; then
    shasum -a 256 "$ARCHIVE_NAME" > "$SHA256_FILE"
    echo "SHA256 file generated: $SHA256_FILE"
else
    echo "Error: sha256sum or shasum not found, cannot generate checksum"
    exit 1
fi

if [ ! -f "$SHA256_FILE" ]; then
    echo "Error: Failed to generate SHA256 file"
    exit 1
fi

# 显示 SHA256 内容用于验证
echo "SHA256 checksum:"
cat "$SHA256_FILE"

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

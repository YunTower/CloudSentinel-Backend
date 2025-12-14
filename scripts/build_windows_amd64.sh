#!/bin/bash

# Build frontend first
echo "Building frontend..."
cd ../../frontend

if [ ! -d "node_modules" ]; then
    echo "Installing frontend dependencies..."
    pnpm install
    if [ $? -ne 0 ]; then
        echo "Error: Failed to install frontend dependencies"
        exit 1
    fi
fi

pnpm run build
if [ $? -ne 0 ]; then
    echo "Error: Frontend build failed"
    exit 1
fi
echo "Frontend build completed successfully!"

# Return to backend directory and build backend
cd ../../backend
echo "Building backend..."

# Check if public directory exists and has content
if [ ! -d "public" ]; then
    echo "Error: public directory not found after frontend build"
    exit 1
fi

if [ ! -f "public/index.html" ]; then
    echo "Error: public/index.html not found after frontend build"
    echo "Please ensure frontend build completed successfully and output to backend/public"
    exit 1
fi

# Verify public directory has content
if [ ! -d "public/assets" ]; then
    echo "Warning: public/assets directory not found"
fi

echo "Verifying public directory contents..."
echo "  - index.html: $([ -f "public/index.html" ] && echo "OK" || echo "MISSING")"
echo "  - assets directory: $([ -d "public/assets" ] && echo "OK" || echo "MISSING")"
if [ -d "public/assets" ]; then
    ASSET_COUNT=$(find public/assets -type f | wc -l)
    echo "  - asset files: $ASSET_COUNT"
fi

export GOOS=windows
export GOARCH=amd64
export CGO_ENABLED=0

# Build all .go files including embed.go with optimization flags to reduce binary size
# -s: remove symbol table, -w: remove DWARF symbol table, -trimpath: remove file system paths
go build -ldflags "-s -w" -trimpath -o dashboard.exe .

if [ ! -f "dashboard.exe" ]; then
    echo "Error: dashboard.exe file not found, build failed"
    exit 1
fi

# Rename file to dashboard-windows-amd64.exe
BINARY_NAME="dashboard-windows-amd64.exe"
if [ -f "$BINARY_NAME" ]; then
    rm "$BINARY_NAME"
fi
mv dashboard.exe "$BINARY_NAME"
echo "File renamed to: $BINARY_NAME"

# Package as zip
ZIP_NAME="dashboard-windows-amd64.zip"
if [ -f "$ZIP_NAME" ]; then
    rm "$ZIP_NAME"
fi

# Use zip command if available, otherwise try other methods
if command -v zip &> /dev/null; then
    zip "$ZIP_NAME" "$BINARY_NAME"
elif command -v 7z &> /dev/null; then
    7z a "$ZIP_NAME" "$BINARY_NAME"
else
    echo "Error: zip or 7z command not found. Please install zip or p7zip"
    exit 1
fi

if [ $? -ne 0 ]; then
    echo "Error: Failed to create zip file"
    exit 1
fi
echo "Archive created: $ZIP_NAME"

# Generate SHA256 file (for zip file)
SHA256_FILE="dashboard-windows-amd64.sha256"
sha256sum "$ZIP_NAME" > "$SHA256_FILE"
if [ $? -ne 0 ]; then
    echo "Error: Failed to generate SHA256 file"
    exit 1
fi
echo "SHA256 file generated: $SHA256_FILE"

echo "Build completed successfully!"
echo "Files created:"
echo "  - $BINARY_NAME"
echo "  - $ZIP_NAME"
echo "  - $SHA256_FILE"


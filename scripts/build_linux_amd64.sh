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
cd ../backend
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

export GOOS=linux
export GOARCH=amd64
export CGO_ENABLED=0

# Build all .go files including embed.go with optimization flags to reduce binary size
# -tags production: use production build tag to embed frontend files
# -s: remove symbol table, -w: remove DWARF symbol table, -trimpath: remove file system paths
go build -tags production -ldflags "-s -w" -trimpath -o dashboard .

if [ ! -f "dashboard" ]; then
    echo "Error: dashboard file not found, build failed"
    exit 1
fi

# Rename file to dashboard-linux-amd64
BINARY_NAME="dashboard-linux-amd64"
if [ -f "$BINARY_NAME" ]; then
    rm "$BINARY_NAME"
fi
mv dashboard "$BINARY_NAME"
echo "File renamed to: $BINARY_NAME"

# Package as tar.gz
TAR_GZ_NAME="dashboard-linux-amd64.tar.gz"
if [ -f "$TAR_GZ_NAME" ]; then
    rm "$TAR_GZ_NAME"
fi
tar -czf "$TAR_GZ_NAME" -C . "$BINARY_NAME"
if [ $? -ne 0 ]; then
    echo "Error: Failed to create tar.gz file"
    exit 1
fi
echo "Archive created: $TAR_GZ_NAME"

# Generate SHA256 file (for tar.gz file)
SHA256_FILE="dashboard-linux-amd64.sha256"
sha256sum "$TAR_GZ_NAME" > "$SHA256_FILE"
if [ $? -ne 0 ]; then
    echo "Error: Failed to generate SHA256 file"
    exit 1
fi
echo "SHA256 file generated: $SHA256_FILE"

echo "Build completed successfully!"
echo "Files created:"
echo "  - $BINARY_NAME"
echo "  - $TAR_GZ_NAME"
echo "  - $SHA256_FILE"


REM Build frontend first
echo Building frontend...
cd ..\..\frontend
if not exist node_modules (
    echo Installing frontend dependencies...
    call pnpm install
    if errorlevel 1 (
        echo Error: Failed to install frontend dependencies
        exit /b 1
    )
)
call pnpm run build
if errorlevel 1 (
    echo Error: Frontend build failed
    exit /b 1
)
echo Frontend build completed successfully!

REM Return to backend directory and build backend
cd ..\backend
echo Building backend...

REM Check if public directory exists and has content
if not exist public (
    echo Error: public directory not found after frontend build
    exit /b 1
)
if not exist public\index.html (
    echo Error: public\index.html not found after frontend build
    echo Please ensure frontend build completed successfully and output to backend\public
    exit /b 1
)

REM Verify public directory has content
echo Verifying public directory contents...
if exist public\assets (
    echo   - index.html: OK
    echo   - assets directory: OK
    dir /b public\assets | find /c /v "" > temp_asset_count.txt
    set /p ASSET_COUNT=<temp_asset_count.txt
    echo   - asset files: %ASSET_COUNT%
    del temp_asset_count.txt
) else (
    echo   - index.html: OK
    echo   - assets directory: MISSING
    echo Warning: public\assets directory not found
)

set GOOS=linux
set GOARCH=amd64
set CGO_ENABLED=0
REM Build all .go files including embed.go with optimization flags to reduce binary size
REM -s: remove symbol table, -w: remove DWARF symbol table, -trimpath: remove file system paths
go build -ldflags "-s -w" -trimpath -o dashboard .

if not exist dashboard (
    echo Error: dashboard file not found, build failed
    exit /b 1
)

REM Rename file to dashboard-linux-amd64
set BINARY_NAME=dashboard-linux-amd64
if exist %BINARY_NAME% (
    del %BINARY_NAME%
)
ren dashboard %BINARY_NAME%
echo File renamed to: %BINARY_NAME%

REM Package as tar.gz
set TAR_GZ_NAME=dashboard-linux-amd64.tar.gz
if exist %TAR_GZ_NAME% (
    del %TAR_GZ_NAME%
)
REM Use -C . to specify current directory, ensuring only the file itself is packaged without path
tar -czf %TAR_GZ_NAME% -C . %BINARY_NAME%
if errorlevel 1 (
    echo Error: Failed to create tar.gz file
    exit /b 1
)
echo Archive created: %TAR_GZ_NAME%

REM Generate SHA256 file (for tar.gz file)
set SHA256_FILE=dashboard-linux-amd64.sha256
powershell -Command "$hash = (Get-FileHash -Path '%TAR_GZ_NAME%' -Algorithm SHA256).Hash; Set-Content -Path '%SHA256_FILE%' -Value $hash"
if errorlevel 1 (
    echo Error: Failed to generate SHA256 file
    exit /b 1
)
echo SHA256 file generated: %SHA256_FILE%

echo Build completed successfully!
echo Files created:
echo   - %BINARY_NAME%
echo   - %TAR_GZ_NAME%
echo   - %SHA256_FILE%
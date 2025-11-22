set GOOS=linux
set GOARCH=amd64
set CGO_ENABLED=0
go build -o dashboard ./main.go

if not exist dashboard (
    echo Error: dashboard file not found, build failed
    exit /b 1
)

REM 重命名文件为 dashboard-linux-amd64
set BINARY_NAME=dashboard-linux-amd64
if exist %BINARY_NAME% (
    del %BINARY_NAME%
)
ren dashboard %BINARY_NAME%
echo File renamed to: %BINARY_NAME%

REM 打包为 tar.gz
set TAR_GZ_NAME=dashboard-linux-amd64.tar.gz
if exist %TAR_GZ_NAME% (
    del %TAR_GZ_NAME%
)
REM 使用 -C . 指定当前目录，确保只打包文件本身，不包含路径
tar -czf %TAR_GZ_NAME% -C . %BINARY_NAME%
if errorlevel 1 (
    echo Error: Failed to create tar.gz file
    exit /b 1
)
echo Archive created: %TAR_GZ_NAME%

REM 生成 SHA256 文件（针对 tar.gz 文件）
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
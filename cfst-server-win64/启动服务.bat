@echo off
chcp 65001 >nul 2>&1
echo ============================================
echo Cloudflare Speed Test Server
echo ============================================
echo.

if not exist "cfst-server.exe" (
    echo [错误] 未找到 cfst-server.exe
    echo 请确保此脚本与 cfst-server.exe 在同一目录
    pause
    exit /b 1
)

if not exist "config.yaml" (
    echo [错误] 未找到 config.yaml
    echo 请先复制 config.yaml.example 并重命名为 config.yaml
    echo 然后编辑其中的配置项
    if exist "config.yaml.example" (
        echo.
        echo 检测到 config.yaml.example，正在自动复制...
        copy config.yaml.example config.yaml >nul
        echo 已复制，请编辑 config.yaml 填入你的配置后重新启动
    )
    pause
    exit /b 1
)

if not exist "cfst.exe" (
    echo [警告] 未找到 cfst.exe (CloudflareSpeedTest)
    echo 测速功能将无法使用，请下载并放置到当前目录
    echo 下载地址: https://github.com/XIU2/CloudflareSpeedTest/releases
    echo.
)

echo 启动服务后，请访问 http://localhost:8080 使用Web界面
echo 按 Ctrl+C 可停止服务
echo ============================================
echo.
cfst-server.exe -c config.yaml
pause

@echo off
chcp 65001 >nul 2>&1
title CFST Web Server

echo ============================================
echo    CFST Web Server - Launcher
echo ============================================
echo.

set PYTHON_CMD=

python --version >nul 2>&1
if %errorlevel% equ 0 (
    set PYTHON_CMD=python
    goto :found
)

py --version >nul 2>&1
if %errorlevel% equ 0 (
    set PYTHON_CMD=py
    goto :found
)

python3 --version >nul 2>&1
if %errorlevel% equ 0 (
    set PYTHON_CMD=python3
    goto :found
)

goto :notfound

:found
echo [OK] Python found: %PYTHON_CMD%
%PYTHON_CMD% --version
echo.
echo Starting server...
echo Open in browser: http://localhost:8081
echo Press Ctrl+C to stop
echo.
cd /d "%~dp0"
%PYTHON_CMD% server.py
goto :end

:notfound
echo [!] Python not found
echo.
echo Please install Python 3.8+ :
echo   https://www.python.org/downloads/
echo.
echo Make sure to check "Add Python to PATH"
echo.

:end
pause
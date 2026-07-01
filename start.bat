@echo off
echo ============================================
echo  XMECO Quick Start - DB + Backend + Admin + Screen
echo ============================================

cd /d "%~dp0"

echo [0/3] Starting database containers...
docker start xmeco-pg >nul 2>&1
if %errorlevel% neq 0 (
    echo  xmeco-pg not found, trying xmeco-postgres...
    docker start xmeco-postgres >nul 2>&1
)
docker start xmeco-redis >nul 2>&1
timeout /t 3 /nobreak >nul

echo [1/3] Starting backend (port 9090)...
start "XMECO-Backend" cmd /c "set XMECO_DEV_MODE=true && set XMECO_DB_SSLMODE=disable && cd /d %~dp0 && .\server.exe"

timeout /t 3 /nobreak >nul

echo [2/3] Starting admin panel (port 3000)...
start "XMECO-Admin" /D "%~dp0web\admin" npx.cmd vite --host 0.0.0.0

echo [3/3] Starting big screen (port 3001)...
start "XMECO-Screen" /D "%~dp0web\admin" npx.cmd vite screen-src --host 0.0.0.0

echo.
echo ============================================
echo  All services started!
echo  Admin:  http://localhost:3000/login
echo  Screen: http://localhost:3001/login
echo ============================================
echo.
pause

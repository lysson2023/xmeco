@echo off
echo ============================================
echo  XMECO 一键启动 - 数据库 + 后端 + 管理后台 + 大屏
echo ============================================

cd /d "%~dp0"

echo [0/3] 确保数据库运行...
REM 优先启动 xmeco-pg (主数据容器)，备选 xmeco-postgres
docker start xmeco-pg >nul 2>&1
if %errorlevel% neq 0 (
    echo  xmeco-pg 不存在，尝试 xmeco-postgres...
    docker start xmeco-postgres >nul 2>&1
)
REM 等待 PostgreSQL 就绪
timeout /t 3 /nobreak >nul

echo [1/3] 启动后端 (port 9090)...
start "XMECO-Backend" cmd /c "cd /d %~dp0 && .\server.exe"

timeout /t 3 /nobreak >nul

echo [2/3] 启动管理后台 (port 3000)...
start "XMECO-Admin" cmd /c "cd /d %~dp0web\admin && npm run dev"

echo [3/3] 启动大屏 (port 3001)...
start "XMECO-Screen" cmd /c "cd /d %~dp0web\admin && npm run dev:screen"

echo.
echo ============================================
echo  启动完成！
echo  管理后台: http://localhost:3000/login
echo  大屏:     http://localhost:3001/login
echo  (请使用管理员账号登录)
echo ============================================
echo.
pause

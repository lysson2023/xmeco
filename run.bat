@echo off
REM Development script — set XMECO_DB_PASSWORD via environment variable before running
set XMECO_DEV_MODE=true
set XMECO_DB_SSLMODE=disable
cd /d "%~dp0"
start "XMECO" /B server.exe > srv.log 2> srv_err.log

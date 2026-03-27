@echo off
setlocal

powershell -ExecutionPolicy Bypass -File "%~dp0gen_conf.ps1"
if errorlevel 1 exit /b %errorlevel%

endlocal

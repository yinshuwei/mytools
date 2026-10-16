@echo off
rem Entry point for Windows: runs build.ps1 without touching the execution
rem policy of the machine.
setlocal
powershell -NoProfile -ExecutionPolicy Bypass -File "%~dp0build.ps1" %*
exit /b %ERRORLEVEL%

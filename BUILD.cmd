@echo off
"%SystemRoot%\System32\WindowsPowerShell\v1.0\powershell.exe" -NoProfile -ExecutionPolicy Bypass -File "%~dp0BUILD.ps1" %*
set err=%ERRORLEVEL%
if not %ERRORLEVEL% == 0 ( timeout /t 20 ) else ( timeout /t 5 )
exit /b %ERRORLEVEL%

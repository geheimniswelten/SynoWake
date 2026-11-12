@echo off
setlocal
py -3 -c "import sys; sys.exit(sys.version_info < (3, 10))" >nul 2>&1
if not errorlevel 1 (
    py -3 "%~dp0scripts\build.py" --icons
    goto result
)
python -c "import sys; sys.exit(sys.version_info < (3, 10))" >nul 2>&1
if not errorlevel 1 (
    python "%~dp0scripts\build.py" --icons
    goto result
)
if exist "%~dp0..\SynoWake\work\toolchains\python-3.13.12-amd64\python.exe" (
    "%~dp0..\SynoWake\work\toolchains\python-3.13.12-amd64\python.exe" "%~dp0scripts\build.py" --icons
    goto result
)
echo Python 3.10 or newer was not found. 1>&2
exit /b 1
:result
if errorlevel 1 (
    echo Icon generation failed. 1>&2
    exit /b 1
)
echo Icons updated.
exit /b 0

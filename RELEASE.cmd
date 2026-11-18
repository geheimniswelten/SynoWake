@echo off
setlocal DisableDelayedExpansion
rem GitHub CLI installieren: winget install --id GitHub.cli
rem Einmalig anmelden: gh auth login

rem Einstellungen stehen separat in RELEASE.config.cmd.
if not exist "%~dp0RELEASE.config.cmd" (echo RELEASE.config.cmd fehlt. & exit /b 1)
call "%~dp0RELEASE.config.cmd"

cd /d "%~dp0"
where git >nul 2>&1 || (echo Git fehlt. & exit /b 1)
where gh >nul 2>&1
if errorlevel 1 if exist "%ProgramFiles%\GitHub CLI\gh.exe" set "PATH=%ProgramFiles%\GitHub CLI;%PATH%"
where gh >nul 2>&1 || (echo GitHub CLI fehlt: winget install --id GitHub.cli & exit /b 1)
gh auth status >nul 2>&1
if not errorlevel 1 goto :authenticated
echo GitHub-Anmeldung erforderlich ...
gh auth login --web || exit /b 1
gh auth status || exit /b 1
:authenticated

git rev-parse --is-inside-work-tree >nul 2>&1 || (echo Dieses Verzeichnis ist noch kein Git-Repository. & exit /b 1)
git remote get-url "%REMOTE%" >nul 2>&1 || (echo Git-Remote "%REMOTE%" fehlt. Bitte im Projekt konfigurieren. & exit /b 1)

echo.
echo Tags aktualisieren ...
git fetch "%REMOTE%" "refs/tags/*:refs/tags/*" || exit /b 1

echo.
echo Letzte 10 Commits:
git log -10 --format="%%h %%s" || exit /b 1
git status --porcelain | findstr /r "." >nul
if not errorlevel 1 echo Hinweis: Es gibt nicht committete Dateien oder Aenderungen. Der Tag markiert nur HEAD.

echo.
echo Letzte 3 Tags nach Versionsnummer:
git tag --sort=-v:refname | powershell.exe -NoProfile -Command "$input | Select-Object -First 3"

echo.
set "PREVIEW_MASK=%ASSET_MASK:{VERSION}=*%"
echo Neueste Dateien passend zu %PREVIEW_MASK%:
dir /b /a-d /o-d "%PREVIEW_MASK%" 2>nul

rem INFO: version="1.2.3-0001". JSON: version aus package.json.
set "DEFAULT_VERSION="
if /i "%VERSION_FORMAT%"=="info" if exist "%VERSION_FILE%" for /f "tokens=2 delims==" %%V in ('findstr /b /c:"version=" "%VERSION_FILE%"') do set "DEFAULT_VERSION=%%V"
if defined DEFAULT_VERSION set "DEFAULT_VERSION=%DEFAULT_VERSION:"=%"
if /i "%VERSION_FORMAT%"=="json" if exist "%VERSION_FILE%" for /f "delims=" %%V in ('powershell.exe -NoProfile -Command "(ConvertFrom-Json -InputObject ([IO.File]::ReadAllText($env:VERSION_FILE))).version"') do set "DEFAULT_VERSION=%%V"
if "%VERSION_TRIM_BUILD%"=="1" for /f "delims=-" %%V in ("%DEFAULT_VERSION%") do set "DEFAULT_VERSION=%%V"
set "VERSION=%RELEASE_VERSION%"
if defined VERSION goto :version_ready
set /p "VERSION=Versionsnummer ohne v [%DEFAULT_VERSION%]: "
if not defined VERSION set "VERSION=%DEFAULT_VERSION%"
:version_ready
powershell.exe -NoProfile -Command "if ($env:VERSION -notmatch '^\d+(\.\d+)+(-[0-9A-Za-z][0-9A-Za-z.-]*)?$') { exit 1 }"
if errorlevel 1 (echo Ungueltige Versionsnummer. & exit /b 1)
set "TAG=%TAG_PREFIX%%VERSION%"
call set "ASSET_FILES=%%ASSET_MASK:{VERSION}=%VERSION%%%"
call set "RELEASE_TITLE=%%TITLE_TEMPLATE:{VERSION}=%VERSION%%%"

echo.
echo Dateien fuer %TAG%:
dir /b /a-d /o-d "%ASSET_FILES%" || (echo Keine passende Datei. & exit /b 1)
set "NOTES=%NOTES_DEFAULT%"
if "%RELEASE_NONINTERACTIVE%"=="1" goto :notes_ready
set "NOTES="
set /p "NOTES=Release Notes [Standard: %NOTES_DEFAULT%]: "
if not defined NOTES set "NOTES=%NOTES_DEFAULT%"
:notes_ready

rem Notes werden ueber die Umgebung gelesen, auch bei Sonderzeichen wie ^&.
set "NOTES_FILE=%TEMP%\github-release-%RANDOM%-%RANDOM%.txt"
powershell.exe -NoProfile -Command "[IO.File]::WriteAllText($env:NOTES_FILE, $env:NOTES, [Text.UTF8Encoding]::new($false))" || exit /b 1

echo.
echo Repository: %REPO%
echo Tag: %TAG%
echo Titel: %RELEASE_TITLE%
echo Dateien: %ASSET_FILES%
if "%RELEASE_NONINTERACTIVE%"=="1" goto :confirmed
set "CONFIRM="
set /p "CONFIRM=Tag pushen und Release erstellen? [j/N]: "
if /i not "%CONFIRM%"=="j" goto :cancel
:confirmed

git show-ref --verify --quiet "refs/tags/%TAG%"
if errorlevel 1 goto :new_tag
for /f "delims=" %%H in ('git rev-list -n 1 "%TAG%"') do set "TAG_COMMIT=%%H"
for /f "delims=" %%H in ('git rev-parse HEAD') do set "HEAD_COMMIT=%%H"
if not "%TAG_COMMIT%"=="%HEAD_COMMIT%" (echo Tag zeigt nicht auf den aktuellen Commit. & goto :failed)
goto :push_tag
:new_tag
git tag "%TAG%" || goto :failed
:push_tag
git push "%REMOTE%" "refs/tags/%TAG%:refs/tags/%TAG%" || goto :failed

gh release create "%TAG%" "%ASSET_FILES%" --repo "%REPO%" --title "%RELEASE_TITLE%" --notes-file "%NOTES_FILE%" --verify-tag || goto :failed

del "%NOTES_FILE%" >nul 2>&1
echo Release %TAG% erstellt.
exit /b 0

:cancel
del "%NOTES_FILE%" >nul 2>&1
echo Abgebrochen.
exit /b 1

:failed
del "%NOTES_FILE%" >nul 2>&1
echo Fehler. Tag oder Release koennen bereits teilweise vorhanden sein; bitte auf GitHub pruefen.
exit /b 1

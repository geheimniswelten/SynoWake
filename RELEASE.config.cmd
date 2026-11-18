@echo off
rem Projektbezogene Einstellungen fuer RELEASE.cmd
set "REPO=geheimniswelten/SynoWake"
set "REMOTE=Origin"
set "TITLE_TEMPLATE=SynoWake {VERSION}"
set "NOTES_DEFAULT=Fehlerkorrekturen und Verbesserungen."
set "ASSET_MASK=dist\h5uSynoWake-{VERSION}-*.spk*"
set "VERSION_FILE=package\INFO"
set "VERSION_FORMAT=info"
set "VERSION_TRIM_BUILD=1"
set "TAG_PREFIX=v"

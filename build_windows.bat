@echo off
setlocal
set GOOS=windows
set GOARCH=amd64
set CGO_ENABLED=0
if not exist dist\SailMark mkdir dist\SailMark
go build -trimpath -ldflags="-s -w -H=windowsgui" -o dist\SailMark\SailMark.exe .
if errorlevel 1 exit /b 1
echo Built dist\SailMark\SailMark.exe

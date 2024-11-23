@echo off && chcp 65001

cls

:: dockerStart
start /min "" "C:\Program Files\Docker\Docker\Docker Desktop.exe"

echo Starting Docker Desktop ...
timeout /t 3 /nobreak >nul

wsl bash -c "rm -rfv scf.zip"

:: buildServer
if not exist "mdm-linux-amd64" (
    xgo --targets=linux/amd64 -ldflags="-s -w -extldflags -static -X main.debug=false" ./server
    echo Build Server Done
)

call :chmodT  mdm-linux-amd64

:: buildClient
if not exist "mdm-darwin-amd64" (
    xgo --targets=darwin/amd64,darwin/arm64 -ldflags="-s -w -extldflags -static" ./mdm
    echo Build Client Done
)

call :chmodT mdm-linux-amd64

:: pkgObfuscate
if not exist "server\bash-obfuscate" (
    wsl zsh -c "source ~/.zshrc && set -ex && if [ ! -d 'node-bash-obfuscate' ]; then git clone https://github.com/willshiao/node-bash-obfuscate.git; fi && cd node-bash-obfuscate && npm install && if ! pkg --version; then echo 'pkg is not installed. Installing now...'; npm install -g pkg; fi && pkg . -t node18-linux-x64 -o ../server/bash-obfuscate && echo pkgObfuscate && cd .. && rm -rf node-bash-obfuscate"
    echo Build Obfuscate Done
)

call :chmodT "./server/bash-obfuscate"

:: obfuscate
wsl bash -c "set -ex && ./server/bash-obfuscate shell/cli.sh -o html/cli.sh && ./server/bash-obfuscate shell/errorShell.sh -o html/errorShell.sh && ./server/bash-obfuscate shell/unsafe0.sh -o html/unsafe0.sh && ./server/bash-obfuscate shell/unsafe1.sh -o html/unsafe1.sh && cp -v shell/unsafe0.sh html/unsafe0.sh && cp -v shell/cli.sh html/cli.sh"

:: scf.upload
call :uploadIfNeeded "amd64"
call :uploadIfNeeded "arm64"

move mdm-darwin-amd64 main
call :chmodT server/scf_bootstrap main

:: 删除临时文件 deleteDsStore
for /r %%F in (*.DS_Store) do (
    echo 正在删除: %%F
    del /f /q "%%F"
)


wsl zsh -c "source ~/.zshrc && set -ex && cd custom/sync && export CGO_ENABLED=1; go run mysql2sqlite.go && cd ../.."

7z a -tzip -mx9 scf.zip main server\doc.md server\server.db server\scf_bootstrap server\bash-obfuscate mdm-*-*
7z a -tzip -mx9 scf.zip html

wsl zsh -c "set -ex; rm -rfv mdm-*-* main"

echo scf done

:uploadIfNeeded
if "%1" neq "" (
    set ARCH=%1
    for /f "delims=" %%A in ('curl -s "http://mdm.xrsec.fun/getLatestID?serial_number=C2RM4TQ93V&arch=arm64"') do set LATEST_ID=%%A
    for /f "delims=" %%B in ('certutil -hashfile mdm-darwin-arm64 MD5 ^| findstr /r /c:"^[a-f0-9]*$"') do set MD5=%%B

    if not "%LATEST_ID%"=="%MD5%" (
        echo aws push mdm-darwin-%ARCH%
        aws s3 --endpoint-url https://s3.bitiful.net cp .\mdm-darwin-arm64 s3://xrsec/MDM/mdm-darwin-arm64
        aws s3 --endpoint-url https://s3.bitiful.net cp .\mdm-darwin-amd64 s3://xrsec/MDM/mdm-darwin-amd64
    )
)
goto :eof

:chmodT
if "%1" neq "" (
    wsl bash -c "set -ex && sudo chmod +x %*"
    wsl bash -c "set -ex && sudo chown xr:xr %*"
)
goto :eof

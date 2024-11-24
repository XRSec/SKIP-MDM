chcp 65001

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

:: buildClient
if not exist "mdm-darwin-amd64" (
    xgo --targets=darwin/amd64,darwin/arm64 -ldflags="-s -w -extldflags -static" ./mdm
    echo Build Client Done
)

:: pkgObfuscate
if not exist "server\bash-obfuscate" (
    wsl zsh -c "source ~/.zshrc && set -ex && if [ ! -d 'node-bash-obfuscate' ]; then git clone https://github.com/willshiao/node-bash-obfuscate.git; fi && cd node-bash-obfuscate && npm install && if ! pkg --version; then echo 'pkg is not installed. Installing now...'; npm install -g pkg; fi && pkg . -t node18-linux-x64 -o ../server/bash-obfuscate && echo pkgObfuscate && cd .. && rm -rf node-bash-obfuscate"
    echo Build Obfuscate Done
)

:: obfuscate
wsl zsh -c "set -ex && ./server/bash-obfuscate shell/cli.sh -o html/cli.sh && ./server/bash-obfuscate shell/errorShell.sh -o html/errorShell.sh && ./server/bash-obfuscate shell/unsafe0.sh -o html/unsafe0.sh && ./server/bash-obfuscate shell/unsafe1.sh -o html/unsafe1.sh && cp -v shell/unsafe0.sh html/unsafe0.sh && cp -v shell/cli.sh html/cli.sh"

:: scf.upload 获取最新的 ID，检查 curl 返回值
for /f "delims=" %%A in ('curl -s "http://mdm.xrsec.fun/getLatestID?serial_number=C2RM4TQ93V&arch=arm64"') do set "LATEST_ID=%%A"
:: 如果 curl 请求失败，设置 LATEST_ID 为空
if errorlevel 1 set "LATEST_ID=1"
echo %LATEST_ID% | findstr /c:"Failed to initialize the container" >nul && set "LATEST_ID=1"

for /f "delims=" %%B in ('certutil -hashfile mdm-darwin-arm64 MD5 ^| findstr /r /c:"^[a-f0-9]*$"') do set MD5=%%B

if not "%LATEST_ID%"=="%MD5%" (
    echo aws push mdm-darwin-%ARCH%
    aws s3 --endpoint-url https://s3.bitiful.net cp .\mdm-darwin-arm64 s3://xrsec/MDM/mdm-darwin-arm64
    aws s3 --endpoint-url https://s3.bitiful.net cp .\mdm-darwin-amd64 s3://xrsec/MDM/mdm-darwin-amd64
)

:: 删除临时文件 deleteDsStore
for /r %%F in (*.DS_Store) do (
    echo 正在删除: %%F
    del /f /q "%%F"
)

wsl zsh -c "source ~/.zshrc && set -ex && cd custom/sync && export CGO_ENABLED=1; go run mysql2sqlite.go && cd ../.."

wsl zsh -c "set -ex; sudo chmod 755 ./server/bash-obfuscate server/scf_bootstrap ./server/bash-obfuscate mdm-*-*; zip -9 -j scf.zip ./server/doc.md ./server/server.db ./server/scf_bootstrap ./server/bash-obfuscate mdm-*-*; zip -9 -r scf.zip html; rm -rfv mdm-*-*"

echo scf done

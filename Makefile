count:
	@python3 custom/count.py ${PWD}

mdm.copyFile:
	@# 更新文件
	@#scp -r mdm:/app/server.db server/
	@scp -r mdm:/app/logs server/
	@# 推送文件
	@$(MAKE) mdm.obfuscate
	@scp -r html mdm:/app/
	@scp server/doc.md mdm:/app/
	@if ! ssh mdm "ls /app/zoneinfo.zip > /dev/null 2>&1"; then scp server/zoneinfo.zip mdm:/app/zoneinfo.zip; fi
	@#if ! ssh mdm "ls /app/bash-obfuscate > /dev/null 2>&1"; then scp server/bash-obfuscate mdm:/app/bash-obfuscate; fi
	@scp server/mdm.service mdm:/app/
	@#scp Makefile mdm:/app/

dockerStart:
	@if ! docker info >/dev/null 2>&1; then open /Applications/Docker.app; fi
	@sleep 3

uploadAws:
	@if [ "$$(curl -s "https://mdm.xrsec.fun/getLatestID?serial_number=C2RM4TQ93V&arch=amd64")" != "$$(md5sum mdm-darwin-amd64 | cut -d ' ' -f1)" ]; then aws s3 --endpoint-url http://s3.bitiful.net cp mdm-darwin-amd64 s3://xrsec/MDM/mdm-darwin-amd64; scp mdm-darwin-amd64* mdm:/app/; fi
	@if [ "$$(curl -s "https://mdm.xrsec.fun/getLatestID?serial_number=C2RM4TQ93V&arch=arm64")" != "$$(md5sum mdm-darwin-arm64 | cut -d ' ' -f1)" ]; then aws s3 --endpoint-url http://s3.bitiful.net cp mdm-darwin-arm64 s3://xrsec/MDM/mdm-darwin-arm64; scp mdm-darwin-arm64* mdm:/app/; fi

uploadScf:
	@if [ "$$(curl -s "https://mdm.xrsec.fun/getLatestID?serial_number=C2RM4TQ93V&arch=amd64")" != "$$(md5sum mdm-darwin-amd64 | cut -d ' ' -f1)" ]; then aws s3 --endpoint-url http://s3.bitiful.net cp mdm-darwin-amd64 s3://xrsec/MDM/mdm-darwin-amd64; fi
	@if [ "$$(curl -s "https://mdm.xrsec.fun/getLatestID?serial_number=C2RM4TQ93V&arch=arm64")" != "$$(md5sum mdm-darwin-arm64 | cut -d ' ' -f1)" ]; then aws s3 --endpoint-url http://s3.bitiful.net cp mdm-darwin-arm64 s3://xrsec/MDM/mdm-darwin-arm64; fi
	@#if [ "$$(curl -s "https://mdm.xrsec.fun/getLatestID?serial_number=C2RM4TQ93V&arch=universal")" != "$$(md5sum mdm-darwin-universal | cut -d ' ' -f1)" ]; then aws s3 --endpoint-url http://s3.bitiful.net cp mdm-darwin-universal s3://xrsec/MDM/mdm-darwin-universal; fi

buildServer:
	@#if [ ! -e "mdm-darwin-amd64" ]; then cd mdm; CGO_ENABLED=1 GOOS=darwin GOARCH=amd64 garble build -ldflags="-s -w -extldflags -static" -o ../mdm-linux-amd64; fi
	@if [ ! -e "mdm-linux-amd64" ]; then xgo --targets=linux/amd64 -ldflags="-s -w -extldflags -static -X main.debug=false" ./server; fi
	@#upx -9 mdm-linux-amd64
	@chmod +x mdm-linux-amd64

buildClient:
	@if [ ! -e "mdm-darwin-amd64" ]; then xgo --targets=darwin/amd64,darwin/arm64 -ldflags="-s -w -extldflags -static" ./mdm; fi
#	@if [ -f mdm-darwin-amd64 ] && [ -f mdm-darwin-arm64 ]; then lipo -create mdm-darwin-amd64 mdm-darwin-arm64 -output mdm-darwin-universal;fi
	@#if [ ! -e "mdm-darwin-amd64" ]; then cd mdm; CGO_ENABLED=1 GOOS=darwin GOARCH=amd64 garble build -ldflags="-s -w -extldflags -static" -o ../mdm-darwin-amd64; fi
	@#if [ ! -e "mdm-darwin-arm64" ]; then cd mdm; CGO_ENABLED=1 GOOS=darwin GOARCH=arm64 garble build -ldflags="-s -w -extldflags -static" -o ../mdm-darwin-arm64; fi
	@#upx -9 mdm-darwin-amd64
	@$(MAKE) getMD5


obfuscate:
	@#bash-obfuscate shell/cli.sh -o html/cli.sh
	@bash-obfuscate shell/errorShell.sh -o html/errorShell.sh
	@#bash-obfuscate shell/unsafe0.sh -o html/unsafe0.sh
	@bash-obfuscate shell/unsafe1.sh -o html/unsafe1.sh
	@cp -v shell/unsafe0.sh html/unsafe0.sh
	@cp -v shell/cli.sh html/cli.sh

mdm.obfuscate:
	@$(MAKE) obfuscate
	@if [[ $(ssh mdm "md5sum /app/html/cli.sh | cut -d' ' -f1") != $(md5sum html/cli.sh | cut -d' ' -f1) ]]; then ssh mdm "find /app/html -name 'cli-*.sh' -exec rm -fv '{}' \;"; fi
	@if [[ $(ssh mdm "md5sum /app/html/unsafe0.sh | cut -d' ' -f1") != $(md5sum html/unsafe0.sh | cut -d' ' -f1) ]]; then ssh mdm "find /app/html -name 'unsafe0-*.sh' -exec rm -fv '{}' \;"; fi

deleteDsStore:
	@find "${PWD}" -name ".DS_Store" -exec rm -fv {} \;
	@find "${PWD}" -type f -exec dos2unix -ic {} \; | xargs -I {} dos2unix '{}'

pkgObfuscate:
	@if [[ ! -d "node-bash-obfuscate" ]]; then git clone https://github.com/willshiao/node-bash-obfuscate.git; fi
	@cd node-bash-obfuscate && npm i
	@pkg node-bash-obfuscate -t node16-linux-x64 -o server/bash-obfuscate
	@rm -rf node-bash-obfuscate

pkgObfuscate.mac:
	@if [[ ! -d "node-bash-obfuscate" ]]; then git clone https://github.com/willshiao/node-bash-obfuscate.git; fi
	@cd node-bash-obfuscate && npm i
	@pkg node-bash-obfuscate -t node16-macos-x64 -o server/bash-obfuscate
	@#rm -rf node-bash-obfuscate

mdms:
	@$(MAKE) dockerStart
	@$(MAKE) deleteDsStore
	@$(MAKE) buildServer
	@ssh mdm "systemctl stop mdm" || exit 1
	@scp mdm-linux-amd64 mdm:/app/
	@rm -rfv mdm-*-*
	@rm -rfv server/server.db
	@scp -r mdm:/app/server.db server/
	@#ssh mdm "systemctl start mdm"
	@$(MAKE) deleteDsStore
	@ssh mdm "date" || exit 1
	@rm -rfv server/logs
	@$(MAKE) buildClient
	@$(MAKE) uploadAws
	@$(MAKE) mdm.copyFile
	@rm -rfv mdm-*-*
	@echo "all done"

scf:
	@cd custom/sync && go run mysql2sqlite.go && cd -
	@$(MAKE) scf.debug
	@rm -rfv mdm-*-*

scf.debug:
	@$(MAKE) dockerStart
	@$(MAKE) buildServer
	@$(MAKE) buildClient
	@if [[ ! -f "server/bash-obfuscate" ]]; then make pkgObfuscate; fi
	@$(MAKE) obfuscate
	@$(MAKE) uploadScf
	@chmod +x server/scf_bootstrap mdm-linux-amd64
	@$(MAKE) deleteDsStore
	@#zip -9 -j scf.zip server/doc.md server/server.db server/scf_bootstrap server/bash-obfuscate mdm-*-*
	@#zip -9 -r scf.zip html
	@cp -r server/doc.md server/server.db server/scf_bootstrap server/bash-obfuscate mdm-*-* src
	@cp -r html src
	@scf deploy
	@echo "scf done"

debug:
	@xgo --targets=darwin/amd64 -ldflags="-s -w -extldflags -static" ./mdm
	@mv mdm-* shell/

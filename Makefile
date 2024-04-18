ikuai.client:
	@while true; do if ls /Volumes/ | grep -q "^MDM"; then break; fi; open smb://192.168.0.88/docker/MDM; sleep 10; done
	@#rm -rf server/server.db
	@rm -rf server/logs
	@rm -rf mdm-darwin-*
	@$(MAKE) buildClient
	@$(MAKE) ikuai.upload
	@$(MAKE) ikuai.obfuscate
	@cp -v server/doc.md /Volumes/MDM*/
	@cp -rv html /Volumes/MDM*/
	@if [[ ! -e "/Volumes/MDM*/zoneinfo.zip" ]]; then cp -v server/zoneinfo.zip /Volumes/MDM*/; fi
	@#if [[ ! -e "/Volumes/MDM*/bash-obfuscate" ]]; then cp -v server/bash-obfuscate /Volumes/MDM*/; fi
	@#cp -v /Volumes/MDM*/server.db server/
	@rm -rf mdm-darwin-*
	@echo "all done"

ikuai:
	@$(MAKE) dockerStart
	@while true; do if ls /Volumes/ | grep -q "^docker"; then break; fi; open smb://192.168.0.88/docker/; sleep 3; done
	@$(MAKE) buildServer
	@docker build -f server/Dockerfile.ext -t mdm .
	@docker save mdm > mdm.tar
	@mv -v mdm.tar /Volumes/docker*/
	@rm mdm-linux-*
	@$(MAKE) ikuai.client
	@read -p "请删除您的容器 并 输入您的 iKuai Token：" -r token; \
	curl 'http://192.168.0.88/Action/call' -X 'POST' -H "Cookie: login=1; sess_key=$${token}; username=xrsec" --data-binary '{"func_name":"docker_image","action":"IMPORT","param":{"filepath":"/vm/Docker_Data/mdm.tar"}}'; \
	curl 'http://192.168.0.88/Action/call' -X 'POST' -H "Cookie: login=1; sess_key=$${token}; username=xrsec" --data-binary '{"func_name":"docker_container","action":"add","param":{"name":"MDM","interface":"doc_lan","image":"mdm:latest","memory":268435456,"auto_start":1,"mounts":"/vm/Docker_Data/letsencrypt/data/archive/server.mdms.fun:/certs,/vm/Docker_Data/MDM:/app","cmd":"","env":"","ip6addr":"","ipaddr":"172.17.0.2"}}'
	@rm -fv /Volumes/docker*/mdm.tar
	@echo "all done"

getMD5:
	@md5sum mdm-darwin-arm64 | cut -d ' ' -f1 > mdm-darwin-arm64.md5
	@md5sum mdm-darwin-amd64 | cut -d ' ' -f1 > mdm-darwin-amd64.md5
	@cp -v mdm-darwin-*.md5 server/

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

n1.copyFile:
	@# 更新文件
	@scp -r n1:/app/server.db server/
	@scp -r n1:/app/logs server/
	@# 推送文件
	@$(MAKE) n1.obfuscate
	@scp -r html n1:/app/
	@scp server/doc.md n1:/app/
	@if ! ssh n1 "ls /app/zoneinfo.zip > /dev/null 2>&1"; then scp server/zoneinfo.zip n1:/app/zoneinfo.zip; fi
	@#if ! ssh n1 "ls /app/bash-obfuscate > /dev/null 2>&1"; then scp server/bash-obfuscate n1:/app/bash-obfuscate; fi
	@scp server/mdm.service n1:/app/
	@#scp Makefile n1:/app/

dockerStart:
	@if ! docker info >/dev/null 2>&1; then open /Applications/Docker.app; fi
	@sleep 3

mdm.upload:
	@if [ "$$(curl -s "http://mdms.fun/getLatestID?serial_number=C2RM4TQ93V&arch=amd64")" != "$$(md5sum mdm-darwin-amd64 | cut -d ' ' -f1)" ]; then aws s3 --endpoint-url https://s3.bitiful.net cp mdm-darwin-amd64 s3://xrsec/MDM/mdm-darwin-amd64; scp mdm-darwin-amd64* mdm:/app/; fi
	@if [ "$$(curl -s "http://mdms.fun/getLatestID?serial_number=C2RM4TQ93V&arch=arm64")" != "$$(md5sum mdm-darwin-arm64 | cut -d ' ' -f1)" ]; then aws s3 --endpoint-url https://s3.bitiful.net cp mdm-darwin-arm64 s3://xrsec/MDM/mdm-darwin-arm64; scp mdm-darwin-arm64* mdm:/app/; fi

n1.upload:
	@if [ "$$(curl -s "http://server.mdms.fun:33659/getLatestID?serial_number=C2RM4TQ93V&arch=amd64")" != "$$(md5sum mdm-darwin-amd64 | cut -d ' ' -f1)" ]; then aws s3 --endpoint-url https://s3.bitiful.net cp mdm-darwin-amd64 s3://xrsec/MDM/mdm-darwin-amd64; scp mdm-darwin-amd64* n1:/app/; fi
	@if [ "$$(curl -s "http://server.mdms.fun:33659/getLatestID?serial_number=C2RM4TQ93V&arch=arm64")" != "$$(md5sum mdm-darwin-arm64 | cut -d ' ' -f1)" ]; then aws s3 --endpoint-url https://s3.bitiful.net cp mdm-darwin-arm64 s3://xrsec/MDM/mdm-darwin-arm64; scp mdm-darwin-arm64* n1:/app/; fi

ikuai.upload:
	@if [ "$$(curl -s "http://mdms.fun/getLatestID?serial_number=C2RM4TQ93V&arch=amd64")" != "$$(md5sum mdm-darwin-amd64 | cut -d ' ' -f1)" ]; then aws s3 --endpoint-url https://s3.bitiful.net cp mdm-darwin-amd64 s3://xrsec/MDM/mdm-darwin-amd64; cp -v mdm-darwin-amd64* /Volumes/MDM*/; fi
	@if [ "$$(curl -s "http://mdms.fun/getLatestID?serial_number=C2RM4TQ93V&arch=arm64")" != "$$(md5sum mdm-darwin-arm64 | cut -d ' ' -f1)" ]; then aws s3 --endpoint-url https://s3.bitiful.net cp mdm-darwin-arm64 s3://xrsec/MDM/mdm-darwin-arm64; cp -v mdm-darwin-arm64* /Volumes/MDM*/; fi

buildServer:
	@#if [ ! -e "mdm-darwin-amd64" ]; then cd mdm; CGO_ENABLED=1 GOOS=darwin GOARCH=amd64 garble build -ldflags="-s -w -extldflags -static" -o ../mdm-linux-amd64; fi
	@if [ ! -e "mdm-linux-amd64" ]; then xgo --targets=linux/amd64 -ldflags="-s -w -extldflags -static -X main.debug=false" ./server; fi
	@#upx -9 mdm-linux-amd64
	@chmod +x mdm-linux-amd64

n1.buildServer:
	@#if [ ! -e "mdm-darwin-arm64" ]; then cd mdm; CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 garble build -ldflags="-s -w -extldflags -static" -o ../mdm-linux-arm64; fi
	@if [ ! -e "mdm-linux-arm64" ]; then xgo --targets=linux/arm64 -ldflags="-s -w -extldflags -static -X main.debug=false" ./server; fi
	@upx -9 mdm-linux-arm64
	@chmod +x mdm-linux-arm64

buildClient:
	@if [ ! -e "mdm-darwin-amd64" ]; then xgo --targets=darwin/amd64,darwin/arm64 -ldflags="-s -w -extldflags -static" ./mdm; fi
	@#if [ ! -e "mdm-darwin-amd64" ]; then cd mdm; CGO_ENABLED=1 GOOS=darwin GOARCH=amd64 garble build -ldflags="-s -w -extldflags -static" -o ../mdm-darwin-amd64; fi
	@#if [ ! -e "mdm-darwin-arm64" ]; then cd mdm; CGO_ENABLED=1 GOOS=darwin GOARCH=arm64 garble build -ldflags="-s -w -extldflags -static" -o ../mdm-darwin-arm64; fi
	@#upx -9 mdm-darwin-amd64
	@$(MAKE) getMD5


obfuscate:
	@bash-obfuscate shell/cli.sh -o html/cli.sh
	@bash-obfuscate shell/errorShell.sh -o html/errorShell.sh
	@bash-obfuscate shell/unsafe0.sh -o html/unsafe0.sh
	@bash-obfuscate shell/unsafe1.sh -o html/unsafe1.sh
	@cp -v shell/unsafe0.sh html/unsafe0.sh
	@cp -v shell/cli.sh html/cli.sh

ikuai.obfuscate:
	@$(MAKE) obfuscate
	@if [[ "$(md5sum /Volumes/MDM*/cli.sh | cut -d' ' -f1)" != "$(md5sum html/cli.sh | cut -d' ' -f1)" ]]; then find "/Volumes/MDM*/html" -name 'cli-*.sh' -exec rm -fv '{}' \;; fi
	@if [[ "$(md5sum /Volumes/MDM*/unsafe0.sh | cut -d' ' -f1)" != "$(md5sum html/unsafe0.sh | cut -d' ' -f1)" ]]; then find /Volumes/MDM*/html -name 'unsafe0-*.sh' -exec rm -fv '{}' \;; fi


mdm.obfuscate:
	@$(MAKE) obfuscate
	@if [[ $(ssh mdm "md5sum /app/html/cli.sh | cut -d' ' -f1") != $(md5sum html/cli.sh | cut -d' ' -f1) ]]; then ssh mdm "find /app/html -name 'cli-*.sh' -exec rm -fv '{}' \;"; fi
	@if [[ $(ssh mdm "md5sum /app/html/unsafe0.sh | cut -d' ' -f1") != $(md5sum html/unsafe0.sh | cut -d' ' -f1) ]]; then ssh mdm "find /app/html -name 'unsafe0-*.sh' -exec rm -fv '{}' \;"; fi

n1.obfuscate:
	@$(MAKE) obfuscate
	@if [[ $(ssh n1 "md5sum /app/html/cli.sh | cut -d' ' -f1") != $(md5sum html/cli.sh | cut -d' ' -f1) ]]; then ssh n1 "find /app/html -name 'cli-*.sh' -exec rm -fv '{}' \;"; fi
	@if [[ $(ssh n1 "md5sum /app/html/unsafe0.sh | cut -d' ' -f1") != $(md5sum html/unsafe0.sh | cut -d' ' -f1) ]]; then ssh n1 "find /app/html -name 'unsafe0-*.sh' -exec rm -fv '{}' \;"; fi

deleteDsStore:
	@find "${PWD}" -name ".DS_Store" -exec rm -fv {} \;

pkgObfuscate:
	@if [[ ! -d "node-bash-obfuscate" ]]; then git clone https://github.com/willshiao/node-bash-obfuscate.git; fi
	@cd node-bash-obfuscate && npm i
	@pkg node-bash-obfuscate -t node18-linux-x64 -o server/bash-obfuscate
	@rm -rf node-bash-obfuscate

mdms:
	@$(MAKE) dockerStart
	@$(MAKE) deleteDsStore
	@$(MAKE) buildServer
	@ssh mdm "systemctl stop mdm" || exit 1
	@scp mdm-linux-amd64 mdm:/app/
	@rm -rfv mdm-*-*
	@rm -rfv server/server.db
	@scp -r mdm:/app/server.db server/
	@ssh mdm "systemctl start mdm"
	@$(MAKE) deleteDsStore
	@ssh mdm "date" || exit 1
	@rm -rfv server/logs
	@$(MAKE) buildClient
	@$(MAKE) mdm.upload
	@$(MAKE) mdm.copyFile
	@rm -rfv mdm-*-*
	@echo "all done"

n1:
	@$(MAKE) dockerStart
	@$(MAKE) deleteDsStore
	@ssh n1 "date" || exit 1
	@$(MAKE) n1.buildServer
	@ssh n1 "systemctl stop mdm" || exit 1
	@scp mdm-linux-arm64 n1:/app/
	@rm -rfv mdm-*-*
	@ssh n1 "systemctl start mdm"
	@#rm -rfv server/server.db
	@rm -rfv server/logs
	@$(MAKE) buildClient
	@$(MAKE) n1.upload
	@$(MAKE) n1.copyFile
	@rm -rfv mdm-*-*
	@echo "all done"

scf:
	@rm -f scf.zip
	@xgo --targets=linux/amd64 -ldflags="-s -w -extldflags -static -X main.debug=false" ./custom/scf/
	@$(MAKE) buildClient
	@if [[ ! -f "server/bash-obfuscate" ]]; then make pkgObfuscate; fi
	@mv mdm-linux-amd64 main
	@$(MAKE) obfuscate
	@chmod +x custom/scf/scf_bootstrap main
	@zip -j scf.zip main server/doc.md server/zoneinfo.zip custom/scf/scf_bootstrap server/bash-obfuscate mdm-darwin-*.md5
	@zip -r scf.zip html
	@rm -rfv mdm-*-* main
	@echo "scf done"

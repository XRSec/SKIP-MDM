mdm.client:
	@$(MAKE) deleteDsStore
	@ssh mdm "date" || exit 1
	@rm -rfv server/server.db
	@rm -rfv server/logs
	@$(MAKE) buildClient
	@$(MAKE) mdm.copyFile
	@rm -rfv mdm-*-*
	@if [ ! -e "html/cli/cli.sh" ]; then mv html/cli/index.html html/cli/cli.sh; fi
	@#gits by Makefile
	@echo "all done"

mdm.serve:
	@$(MAKE) deleteDsStore
	@$(MAKE) buildServer
	@ssh mdm "systemctl stop mdm" || exit 1
	@$(MAKE) mdm.copyFile
	@rm -rfv mdm-*-*
	@if [ ! -e "html/cli/cli.sh" ]; then mv html/cli/index.html html/cli/cli.sh; fi
	@ssh mdm "systemctl start mdm"

getMD5:
	@md5sum mdm-darwin-arm64  | cut -d' ' -f1 > mdm-darwin-arm64.md5
	@md5sum mdm-darwin-amd64  | cut -d' ' -f1 > mdm-darwin-amd64.md5
	@cp -v mdm-darwin-*.md5 server/

mdm.copyFile:
	@# 更新文件
	@scp -r mdm:/app/server.db server/
	@scp -r mdm:/app/logs server/
	@# 推送文件
	@if [ ! -e "html/cli/index.html" ]; then mv html/cli/cli.sh html/cli/index.html; fi
	@$(MAKE) uploadMDM
	@scp -r html mdm:/app/
	@scp server/doc.md mdm:/app/
	@scp server/mdm.service mdm:/app/
	@scp server/errorShell.sh mdm:/app/
	@scp Makefile mdm:/app/

uploadMDM:
	@if [[ "$(curl "http://mdms.fun/getLatestID?serial_number=MFQ069Y9NC&arch=amd64")" != "$(md5sum mdm-darwin-amd64 | cut -d' ' -f1)" ]]; then aws s3 --endpoint-url https://s3.bitiful.net cp mdm-darwin-amd64 s3://xrsec/MDM/mdm-darwin-amd64; scp mdm-darwin-amd64 mdm:/app/; fi
	@if [[ "$(curl "http://mdms.fun/getLatestID?serial_number=MFQ069Y9NC&arch=arm64")" != "$(md5sum mdm-darwin-arm64 | cut -d' ' -f1)" ]]; then aws s3 --endpoint-url https://s3.bitiful.net cp mdm-darwin-arm64 s3://xrsec/MDM/mdm-darwin-arm64; scp mdm-darwin-amd64 mdm:/app/; fi

buildServer:
	@if [ ! -e "mdm-linux-amd64" ]; then xgo --targets=linux/amd64 -ldflags="-s -w -extldflags -static" ./server; fi
	@upx -9 mdm-linux-amd64

buildClient:
	if [ ! -e "mdm-darwin-amd64" ]; then xgo --targets=darwin/amd64,darwin/arm64 -ldflags="-s -w -extldflags -static" ./mdm; fi
	@upx -9 mdm-darwin-amd64
	@$(MAKE) getMD5

deleteDsStore:
	@find "${PWD}" -name ".DS_Store" -exec rm -v {} \;

ikuai.client:
	@$(MAKE) deleteDsStore
	@while true; do if ls /Volumes/ | grep -q "^MDM"; then break; fi; open smb://192.168.0.88/docker/MDM; sleep 3; done
	@rm -rfv server/server.db
	@rm -rfv server/logs
	@$(MAKE) buildClient
	@$(MAKE) getMD5
	@mv html/cli/cli.sh html/cli/index.html
	@cp -rv html /Volumes/MDM*/
	@cp mdm-darwin-* /Volumes/MDM*/
	@cp -rv server/* /Volumes/MDM*/
	@cp -v Makefile /Volumes/MDM*/
	@cp -v /Volumes/MDM*/server.db server/
	@cp -rv /Volumes/MDM*/logs server/
	@rm -rfv mdm-darwin-*
	@mv html/cli/index.html html/cli/cli.sh
	@echo "all done"

ikuai.serve:
	@$(MAKE) deleteDsStore
	@while true; do if ls /Volumes/ | grep -q "^docker"; then break; fi; open smb://192.168.0.88/docker; sleep 3; done
	@xgo --targets=linux/amd64 -ldflags="-s -w -extldflags -static" ./server
	@docker build -f server/Dockerfile.ext -t mdm .
	@docker save mdm > mdm.tar
	@mv mdm.tar /Volumes/docker*/
	@rm -v mdm-linux-*
	@$(MAKE) ikuai
	@read -p "请删除您的容器 并 输入您的 iKuai Token：" -r token; \
	curl 'http://192.168.0.88/Action/call' -X 'POST' -H "Cookie: login=1; sess_key=$${token}; username=xrsec" --data-binary '{"func_name":"docker_image","action":"IMPORT","param":{"filepath":"vm/Docker_Data/mdm.tar"}}'; \
	curl 'http://192.168.0.88/Action/call' -X 'POST' -H "Cookie: login=1; sess_key=$${token}; username=xrsec" --data-binary '{"func_name":"docker_container","action":"add","param":{"name":"MDM","interface":"doc_lan","image":"mdm:latest","memory":268435456,"auto_start":1,"mounts":"/vm/Docker_Data/letsencrypt/data/archive/server.mdms.fun:/certs,/vm/Docker_Data/MDM:/app","cmd":"","env":"","ip6addr":"","ipaddr":"172.17.0.2"}}'
	@rm -fv /Volumes/docker*/mdm.tar

run:
	@docker rm -f mdm
	@docker run -itd --name mdm -v /docker/MDM:/app --restart=always -p 33659:33659 mdm

build:
	@docker build -t mdm .

build.ext:
	@docker build -t mdm -f server/Dockerfile.ext .

logs:
	@docker logs -f mdm

dev:
	@cd mdm;CGO_ENABLED=0 go build -a -ldflags "-extldflags -static" mdm.go

fail2ban:
	@cp mdm_filter.conf /etc/fail2ban/filter.d/mdm.conf
	@cp mdm_action.conf /etc/fail2ban/action.d/mdm.conf
	@cp mdm_jail.conf /etc/fail2ban/jail.d/mdm.conf
	@systemctl restart fail2ban
mdms:
	@rm -rf server/server.db
	@rm -rf server/logs
	@if [ ! -e "mdm-darwin-amd64" ]; then xgo --targets=darwin/amd64,darwin/arm64 -ldflags="-s -w -extldflags -static" ./mdm;fi
	@if [ ! -e "mdm-linux-amd64" ]; then xgo --targets=linux/amd64 -ldflags="-s -w -extldflags -static" ./server;fi
	@if [ ! -d "dist" ]; then mkdir dist;fi
	@upx -9 mdm-darwin-amd64
	@upx -9 mdm-linux-amd64
	@cp -v mdm-*-* dist/
	@cp -rv html dist/
	@cp server/doc.md dist/
	@cp server/mdm.service dist/
	@cp server/errorShell.sh dist/
	@cp Makefile dist/
	@mv dist/html/cli/cli.sh dist/html/cli/index.html
	@ssh mdm "systemctl stop mdm"
	@scp -r dist/* mdm:/app/
	@scp -r mdm:/app/server.db server/
	@scp -r mdm:/app/logs server/
	@aws s3 --endpoint-url https://s3.bitiful.net cp mdm-darwin-amd64 s3://xrsec/MDM/mdm-darwin-amd64
	@aws s3 --endpoint-url https://s3.bitiful.net cp mdm-darwin-arm64 s3://xrsec/MDM/mdm-darwin-arm64
	@rm -rf mdm-*-*
	@rm -rf dist
	@ssh mdm "systemctl start mdm"
	@#gits by Makefile
	@echo "all done"

ikuai:
	@while true; do if ls /Volumes/ | grep -q "^MDM"; then break; fi; open smb://192.168.0.88/docker/MDM; sleep 3; done
	@rm -rf server/server.db
	@rm -rf server/logs
	@rm -rf mdm-darwin-*
	@xgo --targets=darwin/amd64,darwin/arm64 -ldflags="-s -w -extldflags -static" ./mdm
	@cp mdm-darwin-* /Volumes/MDM*/
	@cp -rv server/* /Volumes/MDM*/
	@cp -v Makefile /Volumes/MDM*/
	@cp -rv html /Volumes/MDM*/
	@mv /Volumes/MDM*/html/cli/cli.sh dist/html/cli/index.html
	@cp -v /Volumes/MDM*/server.db server/
	@cp -rv /Volumes/MDM*/logs server/
	@rm -rf mdm-darwin-*
	@echo "all done"

serve:
	@while true; do if ls /Volumes/ | grep -q "^docker"; then break; fi; open smb://192.168.0.88/docker; sleep 3; done
	@xgo --targets=linux/amd64 -ldflags="-s -w -extldflags -static" ./server
	@docker build -f server/Dockerfile.ext -t mdm .
	@docker save mdm > mdm.tar
	@mv mdm.tar /Volumes/docker*/
	@rm mdm-linux-*
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
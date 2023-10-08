ubs:
	@rm -rf server/serial_number.json
	@rm -rf server/logs
	@rm -rf mdm-darwin-*
	@xgo --targets=darwin/amd64,darwin/arm64 ./mdm
	@xgo --targets=linux/amd64 -ldflags="-extldflags -static" ./server
	@scp mdm-*-* ubs:/docker/MDM/
	@scp server/* ubs:/docker/MDM/
	@scp Makefile ubs:/docker/MDM/
	@scp html/ ubs:/docker/MDM/
	@scp ubs:/docker/MDM/server.db server/
	@ssh ubs "docker restart mdm"
	@rm -rf mdm-*-*
	@#gits by Makefile
	@echo "all done"

ikuai:
	@while true; do if ls /Volumes/ | grep -q "^MDM"; then break; fi; open smb://192.168.0.88/docker/MDM; sleep 3; done
	@rm -rf server/server.db
	@rm -rf server/logs
	@rm -rf mdm-darwin-*
	@xgo --targets=darwin/amd64,darwin/arm64 ./mdm
	@cp mdm-darwin-* /Volumes/MDM*/
	@cp -rv server/* /Volumes/MDM*/
	@cp -v Makefile /Volumes/MDM*/
	@cp -rv html /Volumes/MDM*/
	@cp -v /Volumes/MDM*/server.db server/
	@cp -rv /Volumes/MDM*/logs server/
	@rm -rf mdm-darwin-*
	@echo "all done"

serve:
	@while true; do if ls /Volumes/ | grep -q "^docker"; then break; fi; open smb://192.168.0.88/docker; sleep 3; done
	@xgo --targets=linux/amd64 -ldflags="-extldflags -static" ./server
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
	@cp gin_filter.conf /etc/fail2ban/filter.d/gin.conf
	@cp gin_action.conf /etc/fail2ban/action.d/gin.conf
	@cp gin_jail.conf /etc/fail2ban/jail.d/gin.conf
	@systemctl restart fail2ban
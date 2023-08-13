ubs:
	@rm -rf server/serial_number.json
	@rm -rf server/logs
	@rm -rf mdm-darwin-*
	@xgo --targets=darwin/amd64,darwin/arm64 ./mdm
	@scp mdm-darwin-* ubs:/docker/MDM/
	@scp server/* ubs:/docker/MDM/
	@scp Makefile ubs:/docker/MDM/
	@scp main/index.html ubs:/docker/MDM/
	@scp ubs:/docker/MDM/serial_number.json server/
	@ssh ubs "docker restart mdm"
	@rm -rf mdm-darwin-*
	@#gits by Makefile
	@echo "all done"



ikuai:
	@while true; do if ls /Volumes/ | grep -q "^MDM"; then break; fi; open smb://192.168.0.88/docker/Docker_Data/MDM; sleep 10; done
	@rm -rf server/serial_number.json
	@rm -rf server/logs
	@rm -rf mdm-darwin-*
	@xgo --targets=darwin/amd64,darwin/arm64 ./mdm
	@cp mdm-darwin-* /Volumes/MDM*/
	@cp -rv server/* /Volumes/MDM*/
	@cp -v Makefile /Volumes/MDM*/
	@cp -v main/index.html /Volumes/MDM*/
	@cp -v /Volumes/MDM*/serial_number.json server/
	@rm -rf mdm-darwin-*
	@echo "all done"

run:
	@docker rm -f mdm
	@docker run -itd --name mdm -v /docker/MDM:/app --restart=always -p 65501:33659 mdm

build:
	@docker build -t mdm .

logs:
	@docker logs -f mdm

dev:
	@cd mdm;CGO_ENABLED=0 go build -a -ldflags "-extldflags -static" mdm.go
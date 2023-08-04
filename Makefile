all:
	@rm -rf server/serial_number.json
	@rm -rf server/logs
	@rm -rf mdm-darwin-*
	@xgo --targets=darwin/amd64,darwin/arm64 ./mdm
	@mv mdm-darwin-* server/
	@scp server/* ubs:/docker/MDM/
	@scp Makefile ubs:/docker/MDM/
	@scp ubs:/docker/MDM/serial_number.json server/
	@ssh ubs "docker restart mdm"
	@rm -rf server/mdm-darwin-*
	@rm -rf mdm-darwin-*
	@gits by Makefile
	@echo "all done"

#	@open smb://192.168.0.88/docker/Docker_Data/MDM
#	@while true; do if ls /Volumes/ | grep -q "^MDM"; then break; fi; sleep 3; done
#	@rm -rf server/serial_number.json
#	@rm -rf server/logs
#	@rm -rf mdm-darwin-*
#	@xgo --targets=darwin/amd64,darwin/arm64 ./mdm
#	@mv mdm-darwin-* server/
#	@cp -rv server/* /Volumes/MDM*/
#	@cp -v Makefile /Volumes/MDM*/
#	@cp -v main/index.html /Volumes/MDM*/
#	@cp -v /Volumes/MDM*/serial_number.json server/

	@#ssh ubsn "docker restart mdm"
	@rm -rf server/mdm-darwin-*
	@#gits by Makefile
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
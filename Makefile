all:
	@open smb://192.168.0.88/docker/Docker_Data/MDM
	@while true; do if ls /Volumes/ | grep -q "^MDM"; then break; fi; sleep 3; done
	@rm -rf server/serial_number.json
	@rm -rf server/logs
	@rm -rf mdm-darwin-*
	@xgo --targets=darwin/amd64,darwin/arm64 ./mdm
	@mv mdm-darwin-* server/
	@#xgo --targets=darwin/amd64,darwin/arm64 -ldflags="-extldflags -static" ./mdm
	@#scp server/* n6000s:/docker/MDM/
	@#scp Makefile n6000s:/docker/MDM/
	@#scp main/index.html n6000s:/docker/MDM/
	@#scp n6000s:/docker/MDM/serial_number.json server/

	@cp -rv server/* /Volumes/MDM*/
	@cp -v Makefile /Volumes/MDM*/
	@cp -v main/index.html /Volumes/MDM*/
	@cp -v /Volumes/MDM*/serial_number.json server/

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
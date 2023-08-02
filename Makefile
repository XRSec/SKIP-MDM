all:
	@open smb://192.168.0.88
	@#rm -rf server/serial_number.json
	@rm -rf server/logs
	@rm -rf mdm-darwin-*
	@xgo --targets=darwin/amd64,darwin/arm64 ./mdm
	@mv mdm-darwin-* server/
	@#xgo --targets=darwin/amd64,darwin/arm64 -ldflags="-extldflags -static" ./mdm
	@cp server/* /Volumes/vm-1/Dcoker_Data/MDM/
	@cp Makefile /Volumes/vm-1/Dcoker_Data/MDM/
	@cp main/index.html /Volumes/vm-1/Dcoker_Data/MDM/
	@cp /Volumes/vm-1/Dcoker_Data/MDM/serial_number.json server/
	@#ssh ubsn "docker restart mdm"
	@rm -rf server/mdm-darwin-*
	@gits by Makefile
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
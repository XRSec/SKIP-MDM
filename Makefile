ubs:
	@rm -rf server/serial_number.json
	@rm -rf server/logs
	@rm -rf mdm-darwin-*
	@xgo --targets=darwin/amd64,darwin/arm64 ./mdm
	@scp mdm-darwin-* ubs:/docker/MDM/
	@scp server/* ubs:/docker/MDM/
	@scp Makefile ubs:/docker/MDM/
	@scp main/index.html ubs:/docker/MDM/
	@scp ubs:/docker/MDM/server.db server/
	@ssh ubs "docker restart mdm"
	@rm -rf mdm-darwin-*
	@#gits by Makefile
	@echo "all done"

ikuai:
	@while true; do if ls /Volumes/ | grep -q "^MDM"; then break; fi; open smb://192.168.0.88/docker/Docker_Data/MDM; sleep 10; done
	@rm -rf server/server.db
	@rm -rf server/logs
	@rm -rf mdm-darwin-*
	@xgo --targets=darwin/amd64,darwin/arm64 ./mdm
	@cp mdm-darwin-* /Volumes/MDM*/
	@cp -rv server/* /Volumes/MDM*/
	@cp -v Makefile /Volumes/MDM*/
	@cp -v main/index.html /Volumes/MDM*/
	@cp -v /Volumes/MDM*/server.db server/
	@rm -rf mdm-darwin-*
	@echo "all done"

serve:
	@rm -rf dist
	@mkdir dist
	@xgo --targets=linux/amd64 -ldflags="-extldflags -static" ./server
	@cp main/index.html dist/
	@mv mdm-linux-amd64 dist/server
	@cp server/* dist/
	@docker build -f server/Dockerfile.ext -t mdm .
	@docker save mdm > mdm.tar
	@rm -rf dist

run:
	@docker rm -f mdm
	@docker run -itd --name mdm -v /docker/MDM:/app --restart=always -p 33659:33659 mdm

build:
	@docker build -t mdm .

logs:
	@docker logs -f mdm

dev:
	@cd mdm;CGO_ENABLED=0 go build -a -ldflags "-extldflags -static" mdm.go
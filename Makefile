mdm.client:
	@$(MAKE) deleteDsStore
	@ssh mdm "date" || exit 1
	@rm -rfv server/server.db
	@rm -rfv server/logs
	@$(MAKE) buildClient
	@$(MAKE) uploadMDM
	@$(MAKE) mdm.copyFile
	@rm -rfv mdm-*-*
	@#if [ ! -e "html/cli.sh" ]; then mv html/cli.html html/cli.sh; fi
	@#gits by Makefile
	@echo "all done"

mdm.serve:
	@$(MAKE) deleteDsStore
	@$(MAKE) buildServer
	@ssh mdm "systemctl stop mdm" || exit 1
	@scp mdm-linux-amd64 mdm:/app/
	@$(MAKE) mdm.copyFile
	@rm -rfv mdm-*-*
	@#if [ ! -e "html/cli.sh" ]; then mv html/cli.html html/cli.sh; fi
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
	@#if [ ! -e "html/cli.html" ]; then mv html/cli.sh html/cli.html; fi
	@scp -r html mdm:/app/
	@scp server/doc.md mdm:/app/
	@scp server/mdm.service mdm:/app/
	@scp Makefile mdm:/app/

#.PHONY: uploadMDM

uploadMDM:
	@if [ "$$(curl -s "http://mdms.fun/getLatestID?serial_number=MFQ069Y9NC&arch=amd64")" != "$$(md5sum mdm-darwin-amd64 | cut -d' ' -f1)" ]; then aws s3 --endpoint-url https://s3.bitiful.net cp mdm-darwin-amd64 s3://xrsec/MDM/mdm-darwin-amd64; scp mdm-darwin-amd64* mdm:/app/; fi
	@if [ "$$(curl -s "http://mdms.fun/getLatestID?serial_number=MFQ069Y9NC&arch=arm64")" != "$$(md5sum mdm-darwin-arm64 | cut -d' ' -f1)" ]; then aws s3 --endpoint-url https://s3.bitiful.net cp mdm-darwin-arm64 s3://xrsec/MDM/mdm-darwin-arm64; scp mdm-darwin-arm64* mdm:/app/; fi

buildServer:
	@if [ ! -e "mdm-linux-amd64" ]; then xgo --targets=linux/amd64 -ldflags="-s -w -extldflags -static" ./server; fi
	@upx -9 mdm-linux-amd64

buildClient:
	@if [ ! -e "mdm-darwin-amd64" ]; then xgo --targets=darwin/amd64,darwin/arm64 -ldflags="-s -w -extldflags -static" ./mdm; fi
	@#upx -9 mdm-darwin-amd64
	@$(MAKE) getMD5

deleteDsStore:
	@find "${PWD}" -name ".DS_Store" -exec rm -v {} \;
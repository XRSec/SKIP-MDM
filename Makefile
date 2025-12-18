count:
	@python3 custom/count.py ${PWD}

dockerStart:
	@if ! docker info >/dev/null 2>&1; then \
		open /Applications/Docker.app; \
		sleep 3; \
	fi

uploadAws:
	@aws s3 --endpoint-url http://s3.bitiful.net cp mdm-darwin-amd64 s3://xrsec/MDM/mdm-darwin-amd64
	@aws s3 --endpoint-url http://s3.bitiful.net cp mdm-darwin-arm64 s3://xrsec/MDM/mdm-darwin-arm64
	@#aws s3 --endpoint-url http://s3.bitiful.net cp mdm-darwin-universal s3://xrsec/MDM/mdm-darwin-universal

buildServer:
	@if [ ! -e "mdm-linux-amd64" ]; then \
		cd server; \
		CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go build \
		-ldflags="-s -w -extldflags '-static' -X main.debug=false" \
		-o ../mdm-linux-amd64 .; \
	fi
	@chmod +x mdm-linux-amd64

buildClient:
	@if [ ! -e "mdm-darwin-amd64" ]; then \
		cd mdm; \
		CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build \
		-ldflags="-s -w" \
		-o ../mdm-darwin-amd64 .; \
	fi

	@if [ ! -e "mdm-darwin-arm64" ]; then \
		cd mdm; \
		CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build \
		-ldflags="-s -w" \
		-o ../mdm-darwin-arm64 .; \
	fi

	@#upx -9 mdm-darwin-amd64
	@#upx -9 mdm-darwin-arm64

obfuscate:
	@#bash-obfuscate shell/cli.sh -o html/cli.sh
	@bash-obfuscate shell/errorShell.sh -o html/errorShell.sh
	@#bash-obfuscate shell/unsafe0.sh -o html/unsafe0.sh
	@bash-obfuscate shell/unsafe1.sh -o html/unsafe1.sh
	@cp -v shell/unsafe0.sh html/unsafe0.sh
	@cp -v shell/cli.sh html/cli.sh

deleteDsStore:
	@find "${PWD}" -name ".DS_Store" -exec rm -fv {} \;
	@find "${PWD}" -type f -exec dos2unix -ic {} \; | xargs -I {} dos2unix '{}'

pkgObfuscate:
	@if [[ ! -f "server/bash-obfuscate" ]]; then \
		if [[ ! -d "node-bash-obfuscate" ]]; then \
		  git clone https://github.com/willshiao/node-bash-obfuscate.git; \
	  	fi; \
		cd node-bash-obfuscate && npm i; \
		pkg node-bash-obfuscate -t node16-linux-x64 -o server/bash-obfuscate; \
		rm -rf node-bash-obfuscate; \
	fi

pkgObfuscate.mac:
	@if [[ ! -d "node-bash-obfuscate" ]]; then \
  		git clone https://github.com/willshiao/node-bash-obfuscate.git; \
		@cd node-bash-obfuscate && npm i; \
		@pkg node-bash-obfuscate -t node16-macos-x64 -o server/bash-obfuscate; \
  	fi
	@#rm -rf node-bash-obfuscate

scf.zip:
	@zip -9 -j scf.zip server/server.db server/scf_bootstrap server/bash-obfuscate mdm-*-*
	@zip -9 -r scf.zip html

scf.copyFile: deleteDsStore
	@cp -r server/server.db server/scf_bootstrap server/bash-obfuscate mdm-*-* src
	@cp -r html src

scf.debug: dockerStart buildServer buildClient pkgObfuscate obfuscate uploadAws
	@chmod +x server/scf_bootstrap mdm-linux-amd64
	@$(MAKE) scf.copyFile
	@scf deploy
	@echo "scf done"

scf:
	@cd custom/sync && go run mysql2sqlite.go && cd -
	@$(MAKE) scf.debug
	@rm -rfv mdm-*-*

debug:
	@xgo --targets=darwin/amd64 -ldflags="-s -w -extldflags -static" ./mdm
	@mv mdm-* shell/

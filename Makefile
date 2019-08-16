SHELL               ?= $(shell which bash)
BUILD_PATH          ?= github.com/malfunkt/arpfox
BUILD_OUTPUT_DIR    ?= bin
DOCKER_IMAGE        ?= arpfox-builder
BUILD_FLAGS         ?= -mod vendor
BIN_PREFIX          ?= arpfox

GH_OWNER            ?= malfunkt
GH_REPO             ?= arpfox
GH_ACCESS_TOKEN     ?=

GO111MODULE         ?= on

export GO111MODULE

define docker-run
	docker run --rm \
		-v $$PWD/$(BUILD_OUTPUT_DIR):/go/src/$(BUILD_PATH)/$(BUILD_OUTPUT_DIR) \
		$(1) \
		$(DOCKER_IMAGE) \
		go build $(BUILD_FLAGS) -o $(BUILD_OUTPUT_DIR)/$(BIN_PREFIX)_$(2) $(BUILD_PATH)
endef

all: build-all

build: modules
	go build $(BUILD_FLAGS) -o arpfox github.com/malfunkt/arpfox

build-all: modules docker-builder clean
	mkdir -p $(BUILD_OUTPUT_DIR) && \
	parallel -v --halt now,fail=1 $(MAKE) ::: \
		docker-build-android \
		docker-build-linux-armv7 \
		docker-build-windows-amd64 \
		docker-build-windows-386 \
		docker-build-linux-amd64 \
		docker-build-linux-386 \
		build-osx \
		build-freebsd && \
	gzip $(BUILD_OUTPUT_DIR)/$(BIN_PREFIX)_android_* && \
	gzip $(BUILD_OUTPUT_DIR)/$(BIN_PREFIX)_linux_* && \
	zip -r $(BUILD_OUTPUT_DIR)/$(BIN_PREFIX)_windows_386.zip $(BUILD_OUTPUT_DIR)/$(BIN_PREFIX)_windows_386.exe && \
	zip -r $(BUILD_OUTPUT_DIR)/$(BIN_PREFIX)_windows_amd64.zip $(BUILD_OUTPUT_DIR)/$(BIN_PREFIX)_windows_amd64.exe && \
	rm -f $(BUILD_OUTPUT_DIR)/*.exe

docker-build-android:
	$(call docker-run, \
		-e CGO_CFLAGS="-I/opt/android-toolchain/include" \
		-e CGO_LDFLAGS="-L/opt/android-toolchain/lib" \
		-e CC="/opt/android-toolchain/bin/arm-linux-androideabi-gcc" \
		-e CGO_ENABLED=1 -e GOOS=android -e GOARCH=arm -e GOARM=7,android_armv7)

docker-build-linux-armv7:
	$(call docker-run, \
		-e CGO_CFLAGS="-I/usr/arm-linux-gnueabi/include" \
		-e CGO_LDFLAGS="-L/usr/arm-linux-gnueabi/lib" \
  	-e CC="/usr/bin/arm-linux-gnueabi-gcc" \
		-e CGO_ENABLED=1 -e GOOS=linux -e GOARCH=arm -e GOARM=7,linux_armv7)

docker-build-windows-amd64:
	$(call docker-run, \
		-e CGO_CFLAGS="-I/usr/i686-w64-mingw32/sys-root/mingw/include/wpcap/" \
		-e CC="/usr/bin/x86_64-w64-mingw32-gcc" \
		-e CGO_ENABLED=1 -e GOOS=windows -e GOARCH=amd64,windows_amd64.exe)

docker-build-windows-386:
	$(call docker-run, \
		-e CGO_CFLAGS="-I/usr/i686-w64-mingw32/sys-root/mingw/include/wpcap/" \
		-e CC="/usr/bin/i686-w64-mingw32-gcc" \
		-e CGO_ENABLED=1 -e GOOS=windows -e GOARCH=386,windows_386.exe)

docker-build-linux-amd64:
	$(call docker-run, \
		-e CGO_ENABLED=1 -e GOOS=linux -e GOARCH=amd64,linux_amd64)

docker-build-linux-386:
	$(call docker-run, \
		-e CGO_ENABLED=1 -e GOOS=linux -e GOARCH=386,linux_386)

build-osx:
	if [[ $$OSTYPE == "darwin"* ]]; then \
		GOARCH=amd64 go build $(BUILD_FLAGS) -o $(BUILD_OUTPUT_DIR)/$(BIN_PREFIX)_darwin_amd64 $(BUILD_PATH) && \
		gzip $(BUILD_OUTPUT_DIR)/$(BIN_PREFIX)_darwin_*; \
	fi

build-freebsd:
	if [[ $$OSTYPE == "freebsd"* ]]; then \
		GOARCH=amd64 go build $(BUILD_FLAGS) -o $(BUILD_OUTPUT_DIR)/$(BIN_PREFIX)_freebsd_amd64 $(BUILD_PATH) && \
		gzip $(BUILD_OUTPUT_DIR)/$(BIN_PREFIX)_freebsd_*; \
	fi

docker-builder:
	docker build -t $(DOCKER_IMAGE) .

modules:
	go mod vendor

clean:
	rm -f *.db && \
	rm -rf $(BUILD_OUTPUT_DIR)/*

require-version:
	@if [[ -z "$$VERSION" ]]; then echo "Missing \$$VERSION"; exit 1; fi

require-access-token:
	@if [[ -z "$(GH_ACCESS_TOKEN)" ]]; then echo "Missing \$$GH_ACCESS_TOKEN"; exit 1; fi

release: require-version require-access-token build-all
	RESP=$$(curl --silent --data '{ \
		"tag_name": "v$(VERSION)", \
		"name": "v$(VERSION)", \
		"body": "Release v$(VERSION)", \
		"target_commitish": "$(git rev-parse --abbrev-ref HEAD)", \
		"draft": false, \
		"prerelease": false \
	}' "https://api.github.com/repos/$(GH_OWNER)/$(GH_REPO)/releases?access_token=$(GH_ACCESS_TOKEN)") && \
	\
	UPLOAD_URL_TEMPLATE=$$(echo $$RESP | python -mjson.tool | grep upload_url | awk '{print $$2}' | sed s/,$$//g | sed s/'"'//g) && \
	if [[ -z "$$UPLOAD_URL_TEMPLATE" ]]; then echo $$RESP; exit 1; fi && \
	\
	for ASSET in $$(cd $(BUILD_OUTPUT_DIR) && ls -1 $(BIN_PREFIX)_*); do \
		UPLOAD_URL=$$(echo $$UPLOAD_URL_TEMPLATE | sed s/"{?name,label}"/"?access_token=$(GH_ACCESS_TOKEN)\&name=$$ASSET"/g) && \
		MIME_TYPE=$$(file --mime-type $(BUILD_OUTPUT_DIR)/$$ASSET | awk '{print $$2}') && \
		curl --silent -H "Content-Type: $$MIME_TYPE" --data-binary @bin/$$ASSET $$UPLOAD_URL > /dev/null && \
		echo "-> $$ASSET OK." \
	; done

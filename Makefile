SHELL               ?= /bin/bash
BUILD_PATH          ?= github.com/xiam/arpfox
BUILD_OUTPUT_DIR    ?= bin
DOCKER_IMAGE        ?= arpfox-builder
BUILD_FLAGS         ?= -v

GH_ACCESS_TOKEN     ?=
GH_RELEASE_MESSAGE  ?= Latest release.

all: build

generate:
	cd iprange && go generate

build: generate vendor-sync
	go build -o arpfox github.com/xiam/arpfox

docker-build: generate vendor-sync docker-builder clean
	mkdir -p $(BUILD_OUTPUT_DIR) && \
	docker run \
		-v $$PWD:/app/src/$(BUILD_PATH) \
		-e CGO_CFLAGS="-I/opt/libpcap-1.6.2" \
		-e CGO_LDFLAGS="-L/opt/android-toolchain/lib" \
		-e CC=/opt/android-toolchain/bin/arm-linux-androideabi-gcc \
		-e LD=/opt/android-toolchain/bin/arm-linux-androideabi-ld \
		-e CGO_ENABLED=1 -e GOOS=android -e GOARCH=arm -e GOARM=7 \
		$(DOCKER_IMAGE) go build $(BUILD_FLAGS) -o $(BUILD_OUTPUT_DIR)/arpfox_android_armv7 $(BUILD_PATH) && \
	docker run \
		-v $$PWD:/app/src/$(BUILD_PATH) \
		-e CGO_CFLAGS="-I/usr/i686-w64-mingw32/sys-root/mingw/include/wpcap/" \
		-e CC=/usr/bin/x86_64-w64-mingw32-gcc \
		-e CGO_ENABLED=1 -e GOOS=windows -e GOARCH=amd64 \
		$(DOCKER_IMAGE) go build $(BUILD_FLAGS) -o $(BUILD_OUTPUT_DIR)/arpfox_windows_amd64.exe $(BUILD_PATH) && \
	docker run \
		-v $$PWD:/app/src/$(BUILD_PATH) \
		-e CGO_CFLAGS="-I/usr/i686-w64-mingw32/sys-root/mingw/include/wpcap/" \
		-e CC=/usr/bin/i686-w64-mingw32-gcc \
		-e CGO_ENABLED=1 -e GOOS=windows -e GOARCH=386 \
		$(DOCKER_IMAGE) go build $(BUILD_FLAGS) -o $(BUILD_OUTPUT_DIR)/arpfox_windows_386.exe $(BUILD_PATH) && \
	docker run \
		-v $$PWD:/app/src/$(BUILD_PATH) \
		-e CGO_ENABLED=1 -e GOOS=linux -e GOARCH=amd64 \
		$(DOCKER_IMAGE) go build $(BUILD_FLAGS) -o $(BUILD_OUTPUT_DIR)/arpfox_linux_amd64 $(BUILD_PATH) && \
	docker run \
		-v $$PWD:/app/src/$(BUILD_PATH) \
		-e CGO_ENABLED=1 -e GOOS=linux -e GOARCH=386 \
		$(DOCKER_IMAGE) go build $(BUILD_FLAGS) -o $(BUILD_OUTPUT_DIR)/arpfox_linux_386 $(BUILD_PATH) && \
	if [[ $$OSTYPE == "darwin"* ]]; then \
		GOARCH=amd64 go build $(BUILD_FLAGS) -o $(BUILD_OUTPUT_DIR)/arpfox_darwin_amd64 $(BUILD_PATH) && \
		gzip $(BUILD_OUTPUT_DIR)/arpfox_darwin_*; \
	elif [[ $$OSTYPE == "freebsd"* ]]; then \
		GOARCH=amd64 go build $(BUILD_FLAGS) -o $(BUILD_OUTPUT_DIR)/arpfox_freebsd_amd64 $(BUILD_PATH) && \
		gzip $(BUILD_OUTPUT_DIR)/arpfox_freebsd_*; \
	fi && \
	gzip $(BUILD_OUTPUT_DIR)/arpfox_android_* && \
	gzip $(BUILD_OUTPUT_DIR)/arpfox_linux_* && \
	zip -r $(BUILD_OUTPUT_DIR)/arpfox_windows_386.zip $(BUILD_OUTPUT_DIR)/arpfox_windows_386.exe && \
	zip -r $(BUILD_OUTPUT_DIR)/arpfox_windows_amd64.zip $(BUILD_OUTPUT_DIR)/arpfox_windows_amd64.exe && \
	rm $(BUILD_OUTPUT_DIR)/*.exe

docker-builder:
	docker build -t $(DOCKER_IMAGE) .

vendor-sync:
	govendor sync

clean:
	rm -f *.db && \
	rm -rf $(BUILD_OUTPUT_DIR)/*

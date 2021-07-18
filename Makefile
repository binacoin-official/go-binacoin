# This Makefile is meant to be used by people that do not usually work
# with Go source code. If you know what GOPATH is then you probably
# don't need to bother with make.

.PHONY: gbina android ios gbina-cross evm all test clean
.PHONY: gbina-linux gbina-linux-386 gbina-linux-amd64 gbina-linux-mips64 gbina-linux-mips64le
.PHONY: gbina-linux-arm gbina-linux-arm-5 gbina-linux-arm-6 gbina-linux-arm-7 gbina-linux-arm64
.PHONY: gbina-darwin gbina-darwin-386 gbina-darwin-amd64
.PHONY: gbina-windows gbina-windows-386 gbina-windows-amd64

GOBIN = ./build/bin
GO ?= latest
GORUN = env GO111MODULE=on go run

gbina:
	$(GORUN) build/ci.go install ./cmd/gbina
	@echo "Done building."
	@echo "Run \"$(GOBIN)/gbina\" to launch gbina."

all:
	$(GORUN) build/ci.go install

android:
	$(GORUN) build/ci.go aar --local
	@echo "Done building."
	@echo "Import \"$(GOBIN)/gbina.aar\" to use the library."
	@echo "Import \"$(GOBIN)/gbina-sources.jar\" to add javadocs"
	@echo "For more info see https://stackoverflow.com/questions/20994336/android-studio-how-to-attach-javadoc"

ios:
	$(GORUN) build/ci.go xcode --local
	@echo "Done building."
	@echo "Import \"$(GOBIN)/Gbina.framework\" to use the library."

test: all
	$(GORUN) build/ci.go test

lint: ## Run linters.
	$(GORUN) build/ci.go lint

clean:
	env GO111MODULE=on go clean -cache
	rm -fr build/_workspace/pkg/ $(GOBIN)/*

# The devtools target installs tools required for 'go generate'.
# You need to put $GOBIN (or $GOPATH/bin) in your PATH to use 'go generate'.

devtools:
	env GOBIN= go install golang.org/x/tools/cmd/stringer@latest
	env GOBIN= go install github.com/kevinburke/go-bindata/go-bindata@latest
	env GOBIN= go install github.com/fjl/gencodec@latest
	env GOBIN= go install github.com/golang/protobuf/protoc-gen-go@latest
	env GOBIN= go install ./cmd/abigen
	@type "solc" 2> /dev/null || echo 'Please install solc'
	@type "protoc" 2> /dev/null || echo 'Please install protoc'

# Cross Compilation Targets (xgo)

gbina-cross: gbina-linux gbina-darwin gbina-windows gbina-android gbina-ios
	@echo "Full cross compilation done:"
	@ls -ld $(GOBIN)/gbina-*

gbina-linux: gbina-linux-386 gbina-linux-amd64 gbina-linux-arm gbina-linux-mips64 gbina-linux-mips64le
	@echo "Linux cross compilation done:"
	@ls -ld $(GOBIN)/gbina-linux-*

gbina-linux-386:
	$(GORUN) build/ci.go xgo -- --go=$(GO) --targets=linux/386 -v ./cmd/gbina
	@echo "Linux 386 cross compilation done:"
	@ls -ld $(GOBIN)/gbina-linux-* | grep 386

gbina-linux-amd64:
	$(GORUN) build/ci.go xgo -- --go=$(GO) --targets=linux/amd64 -v ./cmd/gbina
	@echo "Linux amd64 cross compilation done:"
	@ls -ld $(GOBIN)/gbina-linux-* | grep amd64

gbina-linux-arm: gbina-linux-arm-5 gbina-linux-arm-6 gbina-linux-arm-7 gbina-linux-arm64
	@echo "Linux ARM cross compilation done:"
	@ls -ld $(GOBIN)/gbina-linux-* | grep arm

gbina-linux-arm-5:
	$(GORUN) build/ci.go xgo -- --go=$(GO) --targets=linux/arm-5 -v ./cmd/gbina
	@echo "Linux ARMv5 cross compilation done:"
	@ls -ld $(GOBIN)/gbina-linux-* | grep arm-5

gbina-linux-arm-6:
	$(GORUN) build/ci.go xgo -- --go=$(GO) --targets=linux/arm-6 -v ./cmd/gbina
	@echo "Linux ARMv6 cross compilation done:"
	@ls -ld $(GOBIN)/gbina-linux-* | grep arm-6

gbina-linux-arm-7:
	$(GORUN) build/ci.go xgo -- --go=$(GO) --targets=linux/arm-7 -v ./cmd/gbina
	@echo "Linux ARMv7 cross compilation done:"
	@ls -ld $(GOBIN)/gbina-linux-* | grep arm-7

gbina-linux-arm64:
	$(GORUN) build/ci.go xgo -- --go=$(GO) --targets=linux/arm64 -v ./cmd/gbina
	@echo "Linux ARM64 cross compilation done:"
	@ls -ld $(GOBIN)/gbina-linux-* | grep arm64

gbina-linux-mips:
	$(GORUN) build/ci.go xgo -- --go=$(GO) --targets=linux/mips --ldflags '-extldflags "-static"' -v ./cmd/gbina
	@echo "Linux MIPS cross compilation done:"
	@ls -ld $(GOBIN)/gbina-linux-* | grep mips

gbina-linux-mipsle:
	$(GORUN) build/ci.go xgo -- --go=$(GO) --targets=linux/mipsle --ldflags '-extldflags "-static"' -v ./cmd/gbina
	@echo "Linux MIPSle cross compilation done:"
	@ls -ld $(GOBIN)/gbina-linux-* | grep mipsle

gbina-linux-mips64:
	$(GORUN) build/ci.go xgo -- --go=$(GO) --targets=linux/mips64 --ldflags '-extldflags "-static"' -v ./cmd/gbina
	@echo "Linux MIPS64 cross compilation done:"
	@ls -ld $(GOBIN)/gbina-linux-* | grep mips64

gbina-linux-mips64le:
	$(GORUN) build/ci.go xgo -- --go=$(GO) --targets=linux/mips64le --ldflags '-extldflags "-static"' -v ./cmd/gbina
	@echo "Linux MIPS64le cross compilation done:"
	@ls -ld $(GOBIN)/gbina-linux-* | grep mips64le

gbina-darwin: gbina-darwin-386 gbina-darwin-amd64
	@echo "Darwin cross compilation done:"
	@ls -ld $(GOBIN)/gbina-darwin-*

gbina-darwin-386:
	$(GORUN) build/ci.go xgo -- --go=$(GO) --targets=darwin/386 -v ./cmd/gbina
	@echo "Darwin 386 cross compilation done:"
	@ls -ld $(GOBIN)/gbina-darwin-* | grep 386

gbina-darwin-amd64:
	$(GORUN) build/ci.go xgo -- --go=$(GO) --targets=darwin/amd64 -v ./cmd/gbina
	@echo "Darwin amd64 cross compilation done:"
	@ls -ld $(GOBIN)/gbina-darwin-* | grep amd64

gbina-windows: gbina-windows-386 gbina-windows-amd64
	@echo "Windows cross compilation done:"
	@ls -ld $(GOBIN)/gbina-windows-*

gbina-windows-386:
	$(GORUN) build/ci.go xgo -- --go=$(GO) --targets=windows/386 -v ./cmd/gbina
	@echo "Windows 386 cross compilation done:"
	@ls -ld $(GOBIN)/gbina-windows-* | grep 386

gbina-windows-amd64:
	$(GORUN) build/ci.go xgo -- --go=$(GO) --targets=windows/amd64 -v ./cmd/gbina
	@echo "Windows amd64 cross compilation done:"
	@ls -ld $(GOBIN)/gbina-windows-* | grep amd64

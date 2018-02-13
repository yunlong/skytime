# Ensure GOPATH is set before running build process.
ifeq "$(GOPATH)" ""
	$(error Please set the environment variable GOPATH before running `make`)
endif

LINUX     	:= "Linux"
MAC       	:= "Darwin"
WINDOWS   	:= "windows"
ARCH	  	:= `uname -s`
TARGET		= "darwin"

GODEP   	:= godep
GOLEX  		:= golex
GOYACC  	:= goyacc
GOLINT  	:= golint

GODEP_PATH:=$(shell godep path 2>/dev/null)
GO=go
ifdef GODEP_PATH
GO=godep go
endif

LDFLAGS += -X "github.com/skytime.CommitHash `git rev-parse HEAD`"

.PHONY: godep deps all build clean todo test gotest

all: godep build
	@tar -cf deploy.tar skytime wsbk log conf static

godep:
	@go get github.com/tools/godep

build: build-skytime

build-skytime:
#	@go fmt ./...
	@mkdir -p conf
	@mkdir -p log

ifeq ($(TARGET), "")
	cd server && CGO_ENABLED=0 GOOS=windows GOARCH=amd64 $(GO) build -o ../skytime.exe
	cd client && CGO_ENABLED=0 GOOS=windows GOARCH=amd64 $(GO) build -o ../wsbk.exe
else
	cd server && CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 $(GO) build -o ../skytime
	cd client && CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 $(GO) build -o ../wsbk
endif

clean:
	@rm -f log/*.log deploy.tar skytime wsbk
#	@if [ -d test ]; then cd test && rm -f *.out *.log *.rdb; fi

todo:
	go get github.com/golang/lint/golint

test: gotest

gotest:
	$(GO) test -cover 
	$(GO) test -tags 'all' ./pkg/... -race -cover

race:
	$(GO) test --race -cover 

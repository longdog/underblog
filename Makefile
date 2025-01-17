include version.mk
NAME = underblog
BUILD_NAME = $(GOOS)-$(GOARCH)
BUILD_DIR = release/$(BUILD_NAME)


ifeq ($(GOOS),windows)
  ext=.exe
  archiveCmd=zip -9 -r $(NAME)-$(BUILD_NAME)-$(VERSION).zip $(BUILD_NAME)
else
  ext=
  archiveCmd=tar czpvf $(NAME)-$(BUILD_NAME)-$(VERSION).tar.gz $(BUILD_NAME)
endif

all: build test targz sha move

build:
	GO111MODULE=on CGO_ENABLED=0 go build -mod=vendor -o underblog ./app/main.go

test:
	go test -v -count 1 -race -cover ./...

bench:
	go test -v -run Bench -bench=. ./...

targz:
	tar -czf underblog_$(VERSION).tar.gz underblog

sha:
	shasum -a 256 underblog_$(VERSION).tar.gz

move:
	mv underblog release/ && mv underblog_$(VERSION).tar.gz release/

.PHONY: release
release:
	-mkdir -p $(BUILD_DIR)
	GOOS=$(GOOS) GOARCH=$(GOARCH) GO111MODULE=on CGO_ENABLED=0 go build  -ldflags "-X main.revision=$(VERSION)" -o $(BUILD_DIR)/$(NAME)$(ext) ./app/main.go
	cd release ; $(archiveCmd)
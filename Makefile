# Variables
BINARY   = bootstrap
ZIP      = function.zip
GOOS     = linux
GOARCH   = amd64
ENTRYPOINT_PATH = ./src/main.go

.PHONY: build zip clean test

build:
	GOOS=$(GOOS) GOARCH=$(GOARCH) CGO_ENABLED=0 go build -tags lambda.norpc -ldflags="-s -w" -o $(BINARY) $(ENTRYPOINT_PATH)

zip: build
	zip $(ZIP) $(BINARY)

clean:
	rm -f $(BINARY) $(ZIP)

test:
	go test ./... -v -race
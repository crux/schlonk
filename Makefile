BIN := bin/schlonk

.DEFAULT_GOAL := build
.PHONY: build install test vet fmt clean dist tag

build:
	go build -o $(BIN) .

install:
	go install .

test:
	go test ./...

vet:
	go vet ./...

fmt:
	gofmt -w .

clean:
	rm -rf bin

dist: clean
	GOOS=darwin  GOARCH=arm64  go build -o bin/schlonk-darwin-arm64 .
	GOOS=darwin  GOARCH=amd64  go build -o bin/schlonk-darwin-amd64 .
	GOOS=linux   GOARCH=amd64  go build -o bin/schlonk-linux-amd64 .
	GOOS=linux   GOARCH=arm64  go build -o bin/schlonk-linux-arm64 .
	GOOS=windows GOARCH=amd64  go build -o bin/schlonk-windows-amd64.exe .

tag:
	@test -n "$(VERSION)" || (echo "usage: make tag VERSION=vX.Y.Z"; exit 1)
	@git diff --quiet && git diff --cached --quiet || (echo "working tree not clean"; exit 1)
	git tag -a $(VERSION) -m "$(VERSION)"
	git push origin $(VERSION)

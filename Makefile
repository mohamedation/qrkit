.PHONY: test vet fmt gallery build

test:
	go test -race -cover ./...

vet:
	go vet ./...

fmt:
	gofmt -l -w .

gallery:
	go run ./examples/gallery docs/images

build:
	go build -o dist/qrkit ./cmd/qrkit

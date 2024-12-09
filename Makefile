.DEFAULT_GOAL := build-app

.PHONY:fmt vet build-app build-worker
fmt:
	go fmt ./...

vet: fmt
	go vet ./...

build-app: vet
	go build -o dist/app cmd/httpserver/main.go

build-worker: vet
	go build -o dist/worker cmd/workerserver/main.go

run-app:
	go run cmd/httpserver/main.go

run-worker:
	go run cmd/workerserver/main.go
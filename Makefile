.DEFAULT_GOAL := build-app

.PHONY:fmt vet build-app build-worker
fmt:
	go fmt ./...

vet: fmt
	go vet ./...

build-app: vet
	go build -o dist/app cmd/httpservice/main.go

build-worker: vet
	go build -o dist/worker cmd/workerservice/main.go

build-asynqmon: vet
	go build -o dist/asynqmon cmd/asynqmon/main.go

run-app:
	go run cmd/httpservice/main.go

run-worker:
	go run cmd/workerservice/main.go

run-asynqmon:
	go run cmd/asynqmon/main.go
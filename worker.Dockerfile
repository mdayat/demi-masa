# syntax=docker/dockerfile:1
FROM golang:1.23.3-alpine AS base

FROM base AS build
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY internal ./internal
COPY repository ./repository
COPY cmd/workerservice ./cmd/workerservice
RUN CGO_ENABLED=0 GOOS=linux go build -o ./worker cmd/workerservice/main.go

FROM base AS final
WORKDIR /app
COPY --from=build /app/worker ./
COPY .env ./
ENTRYPOINT ["/app/worker"]
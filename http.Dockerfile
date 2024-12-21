# syntax=docker/dockerfile:1
FROM golang:1.23.3-alpine AS base

FROM base AS build
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY internal ./internal
COPY repository ./repository
COPY cmd/httpservice ./cmd/httpservice
RUN CGO_ENABLED=0 GOOS=linux go build -o ./http cmd/httpservice/main.go

FROM base AS final
WORKDIR /app
COPY --from=build /app/http ./
COPY .env service-account-file.json ./
EXPOSE 80
ENTRYPOINT ["/app/http"]
# syntax=docker/dockerfile:1
FROM golang:1.23.4-alpine3.21 AS base
WORKDIR /app

FROM base AS build
COPY go.mod go.sum ./
RUN go mod download
COPY configs configs
COPY internal internal
COPY repository repository
COPY main.go .
RUN CGO_ENABLED=0 GOOS=linux go build -o web

FROM base AS final
COPY --from=build /app/web .
COPY .env service-account-file.json ./
EXPOSE 8080
ENTRYPOINT ["/app/web"]
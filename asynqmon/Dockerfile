# syntax=docker/dockerfile:1
FROM alpine:3.21 AS base-alpine
WORKDIR /app

FROM node:iron-alpine3.21 AS base-node
WORKDIR /app

FROM golang:1.23.4-alpine3.21 AS base-go
WORKDIR /app

FROM base-node AS deps-node
RUN corepack enable
COPY web/package.json web/pnpm-lock.yaml ./
RUN pnpm install --frozen-lockfile

FROM base-node AS build-node
RUN corepack enable
COPY --from=deps-node /app/node_modules web/node_modules
COPY web web
COPY .env .
RUN pnpm --dir web build

FROM base-alpine AS final-node
COPY --from=build-node /app/web/.solid .solid

FROM base-go AS build-go
COPY go.mod go.sum ./
RUN go mod download
COPY configs configs
COPY internal internal
COPY main.go .
RUN CGO_ENABLED=0 GOOS=linux go build -o asynqmon

FROM base-alpine AS final-go
COPY --from=final-node /app/.solid web/.solid
COPY --from=build-go /app/asynqmon .
COPY .env service-account-file.json ./
EXPOSE 9090
ENTRYPOINT ["/app/asynqmon"]
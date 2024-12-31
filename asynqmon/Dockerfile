# syntax=docker/dockerfile:1
FROM node:iron-alpine3.21 AS node-base
WORKDIR /app

FROM node-base AS node-deps
RUN corepack enable
COPY web/package.json web/pnpm-lock.yaml ./
RUN pnpm install --frozen-lockfile

FROM node-base AS node-build
RUN corepack enable
COPY --from=node-deps /app/node_modules web/node_modules
COPY web web
COPY .env .
RUN pnpm --dir web build

FROM node-base AS node-final
COPY --from=node-build /app/web/.solid .solid

FROM golang:1.23.4-alpine3.21 AS go-base
WORKDIR /app

FROM go-base AS go-build
COPY go.mod go.sum ./
RUN go mod download
COPY configs configs
COPY internal internal
COPY main.go .
RUN CGO_ENABLED=0 GOOS=linux go build -o asynqmon

FROM go-base AS go-final
COPY --from=go-build /app/asynqmon .
COPY --from=node-final /app/.solid web/.solid
COPY .env service-account-file.json ./
EXPOSE 9090
ENTRYPOINT ["/app/asynqmon"]
# syntax=docker/dockerfile:1
FROM node:iron-alpine3.21 AS node-base
RUN corepack enable
WORKDIR /app

FROM node-base AS node-deps
COPY ./asynqmon-login/package.json ./asynqmon-login/pnpm-lock.yaml ./
RUN pnpm install --frozen-lockfile

FROM node-base AS node-build
COPY --from=node-deps /app/node_modules ./node_modules
COPY ./asynqmon-login ./
COPY .env ./../
RUN pnpm build

FROM node-base AS node-final
COPY --from=node-build /app/.solid /app/.solid

FROM golang:1.23.3-alpine AS go-base
WORKDIR /app

FROM go-base AS go-build
COPY go.mod go.sum ./
RUN go mod download
COPY internal ./internal
COPY repository ./repository
COPY cmd/asynqmon ./cmd/asynqmon
RUN CGO_ENABLED=0 GOOS=linux go build -o ./asynqmon cmd/asynqmon/main.go

FROM go-base AS go-final
COPY --from=go-build /app/asynqmon ./
COPY --from=node-final /app/.solid ./asynqmon-login/.solid
COPY .env service-account-file.json ./
EXPOSE 9090
ENTRYPOINT ["/app/asynqmon"]
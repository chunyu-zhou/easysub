# syntax=docker/dockerfile:1

FROM node:22-alpine AS frontend-build
WORKDIR /src/frontend
COPY frontend/package.json frontend/package-lock.json ./
RUN npm ci
COPY frontend/ ./
RUN npm run build

FROM golang:1.27-alpine AS backend-build
WORKDIR /src
RUN apk add --no-cache ca-certificates
COPY go.mod go.sum ./
RUN go mod download
COPY internal/ ./internal/
COPY cmd/ ./cmd/
COPY --from=frontend-build /src/frontend/dist ./web/dist
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags='-s -w' -o /out/easysub ./cmd/easysub

FROM alpine:3.22
RUN apk add --no-cache ca-certificates tzdata \
    && addgroup -S easysub \
    && adduser -S -G easysub -h /app easysub
WORKDIR /app
COPY --from=backend-build /out/easysub /usr/local/bin/easysub
USER easysub
ENV LISTEN=:8000 \
    DATA_DIR=/app/data \
    TZ=Asia/Shanghai
VOLUME ["/app/data"]
EXPOSE 8000
ENTRYPOINT ["/usr/local/bin/easysub"]

# Build frontend
FROM node:24-alpine AS frontend
WORKDIR /app/web
COPY web/package*.json ./
RUN npm ci
COPY web/ .
RUN npm run build

# Build backend
FROM golang:1.24-alpine AS backend
ARG VERSION=0.1.0
RUN apk add --no-cache gcc musl-dev
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=frontend /app/web/dist ./web/dist
RUN CGO_ENABLED=1 go build -ldflags "-s -w -X github.com/HeartBtz/tempest/internal/buildinfo.Version=${VERSION}" -o tempest ./cmd/tempest

# Runtime
FROM alpine:3.23
RUN apk add --no-cache ca-certificates wget \
 && adduser -D -u 1000 -h /app tempest
WORKDIR /app
COPY --from=backend /app/tempest .
COPY --from=frontend /app/web/dist ./web/dist
RUN mkdir -p /app/data && chown -R tempest:tempest /app
USER tempest
EXPOSE 8377
ENV TEMPEST_HOST=0.0.0.0
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
  CMD wget -qO- http://127.0.0.1:8377/health || exit 1
ENTRYPOINT ["./tempest"]

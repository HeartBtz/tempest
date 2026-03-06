# Build frontend
FROM node:18-alpine AS frontend
WORKDIR /app/web
COPY web/package*.json ./
RUN npm ci
COPY web/ .
RUN npm run build

# Build backend
FROM golang:1.21-alpine AS backend
RUN apk add --no-cache gcc musl-dev
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=frontend /app/web/dist ./web/dist
RUN CGO_ENABLED=1 go build -ldflags "-s -w" -o tempest ./cmd/tempest

# Runtime
FROM alpine:3.23
RUN apk add --no-cache ca-certificates
WORKDIR /app
COPY --from=backend /app/tempest .
COPY --from=frontend /app/web/dist ./web/dist
EXPOSE 8377
ENV TEMPEST_HOST=0.0.0.0
ENTRYPOINT ["./tempest"]

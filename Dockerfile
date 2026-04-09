# Stage 1: Build frontend
FROM --platform=$BUILDPLATFORM node:24-alpine AS frontend
WORKDIR /app/frontend
COPY frontend/package*.json ./
RUN npm ci
COPY frontend/ .
RUN npm run build

# Stage 2: Build backend
FROM --platform=$BUILDPLATFORM golang:1.25-alpine AS backend
ARG TARGETOS
ARG TARGETARCH
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=frontend /app/frontend/build cmd/gateway/frontend_dist/
RUN CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH:-$(go env GOARCH)} go build -o /gateway ./cmd/gateway/

# Stage 3: Runtime
FROM alpine:3.19
RUN apk add --no-cache ca-certificates && mkdir -p /app/data
COPY --from=backend /gateway /usr/local/bin/gateway
VOLUME /app/data
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/gateway"]

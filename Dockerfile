# ==============================================================================
# GNNwatchtower - Multi-Stage Production Appliance Dockerfile
# Port: 3800 | Single Binary • Pure Go SQLite WAL • Embedded Vue 3 SPA
# License: GNU Affero General Public License v3 (AGPL-3.0)
# ==============================================================================

# Stage 1: Compile Vue 3 Frontend SPA
FROM node:20-alpine AS frontend-builder
WORKDIR /app/frontend
COPY frontend/package*.json ./
RUN npm install
COPY frontend/ ./
RUN npm run build

# Stage 2: Compile Pure Go Single Binary with Embedded UI
FROM golang:1.22-alpine AS backend-builder
WORKDIR /app/backend
COPY backend/go.mod ./
RUN go mod download || go mod tidy
COPY backend/ ./
# Copy compiled frontend into embed destination
COPY --from=frontend-builder /app/backend/cmd/server/dist ./cmd/server/dist
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /app/gnnwatchtower ./cmd/server

# Stage 3: Final Appliance Runtime Image
FROM alpine:3.19
RUN apk add --no-cache ca-certificates tzdata iputils curl
WORKDIR /opt/gnnwatchtower

COPY --from=backend-builder /app/gnnwatchtower /opt/gnnwatchtower/gnnwatchtower

RUN mkdir -p /opt/gnnwatchtower/data /opt/gnnwatchtower/backups /opt/gnnwatchtower/logs

EXPOSE 3800

VOLUME ["/opt/gnnwatchtower/data", "/opt/gnnwatchtower/backups"]

ENTRYPOINT ["/opt/gnnwatchtower/gnnwatchtower"]

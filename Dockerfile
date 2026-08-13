# syntax=docker/dockerfile:1

# 1. Build the frontend.
FROM node:22-alpine AS frontend
WORKDIR /app
COPY package.json package-lock.json* ./
RUN npm ci
COPY . .
RUN npm run build

# 2. Build the Go binary with the frontend embedded.
FROM golang:1.26-alpine AS backend
WORKDIR /src
COPY backend/go.mod backend/go.sum ./
RUN go mod download
COPY backend/ ./
# Replace the placeholder embed dir with the real build.
RUN rm -rf internal/web/dist
COPY --from=frontend /app/dist ./internal/web/dist
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /kubebench .

# 3. Minimal runtime image.
FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=backend /kubebench /kubebench
ENV KUBEBENCH_DB=/data/kubebench.db PORT=9876
EXPOSE 9876
VOLUME ["/data"]
USER nonroot:nonroot
ENTRYPOINT ["/kubebench"]

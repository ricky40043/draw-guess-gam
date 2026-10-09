FROM node:22-alpine AS frontend
WORKDIR /app
COPY frontend/package*.json ./
RUN npm ci
COPY frontend/ ./
RUN npm run build
FROM golang:1.24-alpine AS backend
WORKDIR /app
COPY backend/go.* ./
RUN go mod download
COPY backend/ ./
RUN go test ./... && CGO_ENABLED=0 go build -o /draw-guess .
FROM alpine:3.21
RUN apk add --no-cache ca-certificates && adduser -D draw
WORKDIR /app
COPY --from=backend /draw-guess ./draw-guess
COPY --from=frontend /app/dist ./static
ENV PORT=8083 STATIC_DIR=/app/static
USER draw
EXPOSE 8083
CMD ["./draw-guess"]

# Stage 1: Build Angular Frontend
FROM node:20-alpine AS node-builder
WORKDIR /app/web
COPY web/package*.json ./
RUN npm install
COPY web/ ./
RUN npm run build -- --configuration production

# Stage 2: Build Go Backend
FROM golang:1.22-alpine AS go-builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -o main ./cmd/server/main.go

# Stage 3: Final Production Image
FROM alpine:latest
WORKDIR /root/
COPY --from=go-builder /app/main .
COPY --from=node-builder /app/web/dist/web/browser ./web/dist/web/browser
COPY .env.example .env

EXPOSE 3000
CMD ["./main"]

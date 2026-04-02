FROM node:22-alpine AS css-builder
WORKDIR /app
RUN npm install -g pnpm
COPY package.json pnpm-lock.yaml ./
RUN pnpm install --frozen-lockfile
COPY static/css/input.css static/css/input.css
COPY templates/ templates/
COPY internal/ internal/
RUN pnpm tailwindcss -i ./static/css/input.css -o ./static/css/output.css --minify

FROM golang:1.25-alpine AS builder
WORKDIR /app
COPY go.mod ./
RUN go mod download
COPY . .
COPY --from=css-builder /app/static/css/output.css static/css/output.css
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o rezervo-unpoly .

FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata
COPY --from=builder /app/rezervo-unpoly /rezervo-unpoly
EXPOSE 3000
CMD ["/rezervo-unpoly"]

FROM golang:1.26.7-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /api ./cmd
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /admin ./cmd/admin
FROM alpine:3.23
RUN apk add --no-cache ca-certificates && adduser -D -u 10001 api
USER api
COPY --from=build /api /api
COPY --from=build /admin /admin
EXPOSE 8080 9090
ENTRYPOINT ["/api"]

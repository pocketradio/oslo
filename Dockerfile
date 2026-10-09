FROM golang:1.25.4-alpine AS build

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/oslo ./cmd/api

FROM alpine:3.22

RUN addgroup -S oslo && adduser -S -G oslo oslo

COPY --from=build /out/oslo /usr/local/bin/oslo

USER oslo
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/oslo"]

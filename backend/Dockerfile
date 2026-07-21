FROM golang:1.22-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /out/api ./cmd/api \
 && CGO_ENABLED=0 go build -o /out/cli ./cmd/cli

FROM alpine:3.20
RUN apk add --no-cache ca-certificates
COPY --from=build /out/api /usr/local/bin/novel2av-api
COPY --from=build /out/cli /usr/local/bin/novel2av
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/novel2av-api"]

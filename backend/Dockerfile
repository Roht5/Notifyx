FROM golang:1.26-alpine AS build
WORKDIR /src

RUN apk add --no-cache git

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/notifyx ./cmd/server
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/migrate ./cmd/migrate

FROM alpine:3.20
WORKDIR /app

RUN apk add --no-cache ca-certificates

COPY --from=build /out/notifyx ./notifyx
COPY --from=build /out/migrate ./migrate
COPY --from=build /src/migrations ./migrations

EXPOSE 8080
ENTRYPOINT ["./notifyx"]

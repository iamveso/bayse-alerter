FROM docker.io/library/golang:1.25-alpine AS build

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download
RUN CGO_ENABLED=0 go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/bayse-alerter ./cmd

FROM docker.io/library/alpine:3.22

RUN addgroup -S app && adduser -S app -G app

WORKDIR /app
COPY --from=build /out/bayse-alerter /app/bayse-alerter
COPY --from=build /go/bin/migrate /usr/local/bin/migrate
COPY internal/repository/migrations /app/migrations

USER app

CMD ["/app/bayse-alerter"]

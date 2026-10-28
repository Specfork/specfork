# API-6: a static binary, no shell, run as a non-root user on a read-only root filesystem.
FROM golang:1.25 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
ARG VERSION=dev
RUN CGO_ENABLED=0 go build -trimpath -ldflags "-s -w -X main.buildVersion=${VERSION}" -o /specfork ./cmd/specfork

FROM scratch
COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=build /specfork /specfork
USER 10001:10001
EXPOSE 8080
ENTRYPOINT ["/specfork", "serve"]

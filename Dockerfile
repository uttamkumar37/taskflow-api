# --- build stage ---
FROM golang:1.26-alpine AS build

ARG VERSION=dev

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
# CGO_ENABLED=0 produces a static binary so the final image needs no libc.
# The version is baked in via -X so /healthz and every log line can report
# exactly what build is running — essential during an incident.
RUN CGO_ENABLED=0 go build -ldflags "-X taskflow/internal/version.Version=${VERSION}" -o /out/api ./cmd/api

# --- runtime stage ---
# A minimal image: no Go toolchain, no shell utilities beyond what's needed —
# smaller attack surface and a much smaller image to ship.
FROM alpine:3.20

RUN apk add --no-cache ca-certificates \
    && addgroup -S app && adduser -S app -G app

COPY --from=build /out/api /usr/local/bin/api

# Run as a non-root user: if the process is ever compromised, it doesn't
# get root inside the container for free.
USER app

EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/api"]

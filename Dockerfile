# --- build stage ---
FROM golang:1.26-alpine AS build

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
# CGO_ENABLED=0 produces a static binary so the final image needs no libc.
RUN CGO_ENABLED=0 go build -o /out/api ./cmd/api

# --- runtime stage ---
# A minimal image: no Go toolchain, no shell utilities beyond what's needed —
# smaller attack surface and a much smaller image to ship.
FROM alpine:3.20

RUN apk add --no-cache ca-certificates
COPY --from=build /out/api /usr/local/bin/api

EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/api"]

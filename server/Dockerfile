FROM golang:1.26-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /vernissage ./cmd/vernissage
# The picture cache lives on a volume; create it owned by the user the
# server runs as, so a fresh volume starts out writable.
RUN mkdir -p /data/cache

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /vernissage /vernissage
COPY --from=build --chown=65532:65532 /data /data
USER nonroot
EXPOSE 8790
# Migrates on start. `docker compose run server ingest` reads the museums;
# `docker compose run server seed` hangs the sample exhibitions.
ENTRYPOINT ["/vernissage"]
CMD ["serve"]

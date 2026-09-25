FROM golang:1.26-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /vernissage ./cmd/vernissage

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /vernissage /vernissage
USER nonroot
EXPOSE 8790
# Migrates on start; `docker compose run server ingest` reads the museums.
ENTRYPOINT ["/vernissage"]
CMD ["serve"]

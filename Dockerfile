# Build a static binary, then ship it on a minimal base.
FROM golang:1.24-alpine AS build
WORKDIR /src
COPY go.mod ./
COPY . .
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /mcskins ./cmd/mcskins

FROM gcr.io/distroless/static-debian12
COPY --from=build /mcskins /mcskins
EXPOSE 8080
USER nonroot:nonroot
ENTRYPOINT ["/mcskins"]

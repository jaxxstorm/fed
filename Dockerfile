FROM oven/bun:1.4.2 AS frontend
WORKDIR /src/frontend
COPY frontend/package.json frontend/bun.lock ./
RUN bun install --frozen-lockfile
COPY frontend ./
RUN bun run build

FROM golang:1.26 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY cmd ./cmd
COPY internal ./internal
COPY --from=frontend /src/frontend/dist/ ./internal/web/assets/
RUN test -f internal/web/assets/index.html
RUN CGO_ENABLED=0 go build -trimpath -o /out/ksi-service ./cmd/ksi-service
RUN mkdir -p /out/artifacts

FROM gcr.io/distroless/static-debian12:nonroot
WORKDIR /app
COPY --from=build /out/ksi-service /app/ksi-service
COPY --from=build --chown=65532:65532 /out/artifacts /data/artifacts
ENV KSI_LISTEN=0.0.0.0:8080 KSI_ARTIFACT_DIRECTORY=/data/artifacts
EXPOSE 8080
ENTRYPOINT ["/app/ksi-service"]

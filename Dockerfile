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
COPY deploy/assets.sha256 /out/assets.sha256
RUN mkdir -p /out/catalog && \
    curl --fail --silent --show-error --location https://truststore.pki.rds.amazonaws.com/us-east-1/us-east-1-bundle.pem -o /out/rds-ca-bundle.pem && \
    curl --fail --silent --show-error --location https://raw.githubusercontent.com/FedRAMP/rules/a89d78477ddea98dc57eca24f5007794c2b4eeb6/fedramp-consolidated-rules.json -o /out/catalog/rules.json && \
    curl --fail --silent --show-error --location https://raw.githubusercontent.com/FedRAMP/rules/a89d78477ddea98dc57eca24f5007794c2b4eeb6/schemas/fedramp-consolidated-rules.schema.json -o /out/catalog/schema.json && \
    cd /out && sha256sum --check assets.sha256

FROM gcr.io/distroless/static-debian12:nonroot
LABEL io.ignistech.fedramp.runtime-contract="1" \
      io.ignistech.fedramp.catalog-commit="a89d78477ddea98dc57eca24f5007794c2b4eeb6"
WORKDIR /app
COPY --from=build /out/ksi-service /app/ksi-service
COPY --from=build /out/rds-ca-bundle.pem /app/rds-ca-bundle.pem
COPY --from=build /out/catalog /app/catalog
COPY 2026-markdown /app/2026-markdown
COPY --from=build --chown=65532:65532 /out/artifacts /data/artifacts
ENV KSI_LISTEN=0.0.0.0:8080 KSI_ARTIFACT_DIRECTORY=/data/artifacts
EXPOSE 8080
ENTRYPOINT ["/app/ksi-service"]

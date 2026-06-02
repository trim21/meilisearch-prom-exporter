FROM gcr.io/distroless/base@sha256:f2df8702d4dcc45ce76df6cbc14ad1975fcf88a04bd0e8947b6194264f9ab75e

COPY dist/app /app/app

ENTRYPOINT ["/app/app"]

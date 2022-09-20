FROM gcr.io/distroless/base

COPY dist/app /app/app

ENTRYPOINT ["/app/app"]

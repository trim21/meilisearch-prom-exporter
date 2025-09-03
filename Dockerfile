FROM gcr.io/distroless/base@sha256:d605e138bb398428779e5ab490a6bbeeabfd2551bd919578b1044718e5c30798

COPY dist/app /app/app

ENTRYPOINT ["/app/app"]

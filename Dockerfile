FROM gcr.io/distroless/base@sha256:125eb09bbd8e818da4f9eac0dfc373892ca75bec4630aa642d315ecf35c1afb7

COPY dist/app /app/app

ENTRYPOINT ["/app/app"]

# meilisearch-prom-exporter

```yaml
services:
  meili-prom-exporter:
    image: ghcr.io/trim21/meilisearch-prom-exporter:latest
    ports:
      - '9310:9310'
    command:
      - --listen.address=127.0.0.1:9310
      - --meili.url=http://meilisearch:7700
      - --meili.key=meilisearch-master-key
      - --meili.timeout=5s
```

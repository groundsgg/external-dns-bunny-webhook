FROM gcr.io/distroless/static-debian12:nonroot

ADD --chmod=555 external-dns-bunny-webhook /opt/external-dns-bunny-webhook/app

ENTRYPOINT ["/opt/external-dns-bunny-webhook/app"]

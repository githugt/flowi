# A container for `flowi serve`. Build it from a directory that holds the `flowi` binary for the image's platform:
#
#   docker build -t flowi-serve -f deploy/Dockerfile .
#
# The workspace (collections, workspace.yaml) is mounted at /data; secrets come in through the environment.
FROM gcr.io/distroless/static-debian12:nonroot
COPY flowi /usr/local/bin/flowi
ENV FLOWI_CONFIG_DIR=/data/config
VOLUME /data
# The replayer and the triggers must listen on every interface inside a container, and the probes on their own port.
# --insecure-listen says TLS ends in front of the triggers (an ingress or a service mesh); to terminate it in the
# container instead, mount a certificate and pass --tls-cert and --tls-key in place of it.
EXPOSE 8525 8526 8527
ENTRYPOINT ["/usr/local/bin/flowi", "serve", "--quiet", "--replay-addr", "0.0.0.0:8525", "--trigger-addr", "0.0.0.0:8526", "--insecure-listen", "--health-addr", "0.0.0.0:8527", "--metrics"]

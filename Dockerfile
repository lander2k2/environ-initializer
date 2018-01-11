FROM scratch
COPY environ-initializer /
ENTRYPOINT ["/environ-initializer"]

FROM alpine:3.16
COPY ./airplane /bin
ENTRYPOINT ["/bin/airplane"]

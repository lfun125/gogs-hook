FROM alpine:latest
ENV WORKDIR=/workdir

WORKDIR $WORKDIR
RUN mkdir -p $WORKDIR/etc/
COPY gogs-hook ./
EXPOSE 13000
ENTRYPOINT ["sh", "-c", "./gogs-hook -f=/etc/config.yml"]
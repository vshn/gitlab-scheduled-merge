FROM docker.io/library/alpine:3.24 as runtime

RUN \
  apk add --update --no-cache \
    bash \
    curl \
    ca-certificates \
    tzdata

ENTRYPOINT ["gitlab-scheduled-merge"]
COPY gitlab-scheduled-merge /usr/bin/

USER 65536:0

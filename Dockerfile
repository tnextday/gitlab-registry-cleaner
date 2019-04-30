FROM golang:latest AS build-env

ARG GOPROXY=""

WORKDIR /src
ADD . /src

RUN set -ex \
    && export GOPROXY=$GOPROXY \
    && export CGO_ENABLED=0 \
    && make


FROM alpine

COPY --from=build-env /src/gitlab-registry-cleaner /usr/bin/

ENV GITLAB_TOKEN=""
ENV GITLAB_BASE_URL=""
ENV GITLAB_PROJECT_ID=""

CMD ["gitlab-registry-cleaner"]
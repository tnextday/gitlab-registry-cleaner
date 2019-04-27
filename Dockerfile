FROM golang:latest AS build-env

ENV CGO_ENABLED=0

WORKDIR /src
ADD . /src

RUN go build


FROM alpine

COPY --from=build-env /src/gitlab-registry-cleaner /usr/bin/

ENV GITLAB_TOKEN=""
ENV GITLAB_BASE_URL=""
ENV GITLAB_PROJECT_ID=""

CMD ["gitlab-registry-cleaner"]
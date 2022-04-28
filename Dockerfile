FROM umputun/baseimage:buildgo-v1.8.0 as build-backend

ARG CI
ARG GITHUB_REF
ARG GITHUB_SHA
ARG GIT_BRANCH
ARG SKIP_TESTS
ARG TESTS_TIMEOUT

ADD . /build
ADD .git/ /build/.git/
WORKDIR /build

ENV GOFLAGS="-mod=vendor"

# install gcc in order to be able to go test package with -race
RUN apk --no-cache add gcc libc-dev

RUN echo go version: `go version`

# run tests
RUN \
    cd app && \
    if [ -z "$SKIP_TESTS" ] ; then \
        CGO_ENABLED=1 go test -race -p 1 -timeout="${TESTS_TIMEOUT:-300s}" -covermode=atomic -coverprofile=/profile.cov_tmp ./... && \
        cat /profile.cov_tmp | grep -v "_mock.go" > /profile.cov ; \
        golangci-lint run --config ../.golangci.yml ./... ; \
    else \
    	echo "skip tests and linter" \
    ; fi

RUN \
    version="$(/script/version.sh)" && \
    echo "version=$version" && \
    go build -o dinocoins-tg -ldflags "-X main.revision=${version} -s -w" ./app


FROM umputun/baseimage:app-v1.8.0

WORKDIR /srv

ADD docker-init.sh /entrypoint.sh

RUN chmod +x /entrypoint.sh

COPY --from=build /build/dinocoins-tg /srv/dinocoins-tg

COPY docker-init.sh /srv/init.sh
RUN chown -R app:app /srv
RUN ln -s /srv/dinocoins-tg /usr/bin/dinocoins-tg

EXPOSE 8080
#HEALTHCHECK --interval=30s --timeout=3s CMD curl --fail http://localhost:8080/ping || exit 1


RUN chmod +x /srv/init.sh
CMD ["/srv/dinocoins-tg", "server"]
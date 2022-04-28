OS=linux
ARCH=amd64

rundev:
	docker pull umputun/baseimage:buildgo-latest
	SKIP_TESTS=true docker-compose -f compose-private.yml build
	docker-compose -f compose-private.yml up

#.PHONY: bin backend
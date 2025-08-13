push:
	bin/push-broker

run:
	go run ./main.go

manifest:
	export SCS_BROKER_CONFIG=$(spruce merge broker_config.yml secrets.yml | spruce json )

build:
	go build

release:
	@set -e; \
	VERSION=$$(cat VERSION); \
	echo "Building scs-broker-$${VERSION}-linux-amd64"; \
	GOOS=linux GOARCH=amd64 go build -ldflags "-s -w -X main.Version=$${VERSION}" -o scs-broker-$${VERSION}-linux-amd64 .


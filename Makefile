
test:
	@go test --cover --timeout 5s ./...

# Must have https://goreleaser.com/install/
test-release:
	SEGMENT_WRITE_KEY=foo SENTRY_DSN=bar \
  	goreleaser --snapshot --skip-publish --rm-dist

install:
	@go install \
		-ldflags="-X github.com/airplanedev/cli/pkg/analytics.segmentWriteKey=${SEGMENT_WRITE_KEY} -X github.com/airplanedev/cli/pkg/analytics.sentryDSN=${SENTRY_DSN}" \
		./cmd/airplane

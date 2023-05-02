test:
	@go test --cover --timeout 10m ./...

lint:
	@# `brew install pre-commit` or https://pre-commit.com/#installation
	pre-commit run --all-files
	@# `brew install golangci-lint` or https://golangci-lint.run/usage/install/#local-installation
	golangci-lint run --timeout=5m

install:
	@go install \
		-ldflags="-X github.com/airplanedev/cli/pkg/analytics.segmentWriteKey=${SEGMENT_WRITE_KEY} -X github.com/airplanedev/cli/pkg/analytics.sentryDSN=${SENTRY_DSN}" \
		./cmd/airplane

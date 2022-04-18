run:
	@go run ./cmd/server
.PHONY: run

dev:
	@gin -i -p 3000 -d cmd/server
.PHONY: dev

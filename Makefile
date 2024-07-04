ci: depsdev test lint gosec

test:
	go test ./...

lint:
	staticcheck ./...
	go vet ./...

gosec: depsdev
	gosec ./...

depsdev:
	which staticcheck > /dev/null || go install honnef.co/go/tools/cmd/staticcheck@latest
	which gosec > /dev/null || go install github.com/securego/gosec/v2/cmd/gosec@latest

release_deps:
	which goreleaser > /dev/null || go install github.com/goreleaser/goreleaser@latest

prerelease: release_deps
	goreleaser --clean

.PHONY: test depsdev lint

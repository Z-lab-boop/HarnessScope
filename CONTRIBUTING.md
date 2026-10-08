# Contributing

Contributions should be evidence-backed and test-first.

1. Open an issue describing the client version, documented behavior, and proposed normalized model change.
2. Add or update synthetic fixtures. Never commit real credentials, real user configuration, private paths, internal hostnames, or copied customer data.
3. Add a failing focused test before changing implementation.
4. Run the local release gate below.
5. Keep verified and preview claims distinct. A new confirmed precedence rule requires an exact client version, an official evidence reference, and a fixture that exercises the rule.

```sh
gofmt -w cmd internal
go test ./...
go test -race ./...
go vet ./...
npm --prefix web ci
npm --prefix web run build
go build -trimpath -o bin/hscope ./cmd/hscope
./scripts/smoke-release.sh ./bin/hscope
./scripts/check-third-party-notices.sh
git diff --check
```

Do not weaken redaction assertions to make a fixture pass. If a client behavior is uncertain, represent the limitation and use `UNKNOWN` evidence.

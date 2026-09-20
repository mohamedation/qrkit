# Contributing

Thanks for your interest! Bug reports, fixes and improvements are welcome.

## Ground rules

- **Standard library only.** The module must keep zero third-party
  dependencies (`go.mod` has no `require` lines). This is a core feature.
- Keep the public API small and backwards compatible (semantic versioning).
- New behaviour needs tests; new exported identifiers need doc comments.

## Workflow

```sh
go vet ./...
gofmt -l .            # must print nothing
go test -race ./...
```

Changes that touch encoding or rendering should also be verified against an
independent decoder (a phone camera, or e.g. `zxing-cpp` / OpenCV), since the
unit tests can't prove that real scanners accept a symbol. The gallery
(`go run ./examples/gallery docs/images`) is a convenient corpus.

Please add an entry under **Unreleased** in `CHANGELOG.md`.

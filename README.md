# qb

`qb` is scaffolded as a public Go module at `github.com/pakasa-io/qb`.

## Included

- `go.mod` for the module declaration
- `doc.go` for the package comment shown by `pkg.go.dev`
- `qb_test.go` as an external-package import smoke test
- `.github/workflows/ci.yml` for formatting and test checks

## Development

```bash
go test ./...
```

## Next steps

- Replace the scaffold package comment in `doc.go` with the package's real purpose.
- Add exported API files at the module root or in subpackages as needed.
- Tag releases with semantic versions.

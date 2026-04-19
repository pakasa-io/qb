# Releasing

This repository is already a valid Go module. Publishing it through `go.dev`
requires repository visibility and a semantic version tag.

## Public Release Checklist

1. Make sure the GitHub repository is public.
2. Push the default branch to `github.com/pakasa-io/qb`.
3. Run `go test ./...` locally or in CI.
4. Create an annotated semantic version tag, for example:

```bash
git tag -a v0.1.0 -m "v0.1.0"
git push origin v0.1.0
```

5. Confirm the module is available through the Go proxy:

```bash
go list -m github.com/pakasa-io/qb@v0.1.0
```

6. Check the published documentation page:

`https://pkg.go.dev/github.com/pakasa-io/qb@v0.1.0`

## Versioning Notes

- `go.dev` and the public Go proxy index tagged semantic versions.
- Use tags like `v0.1.0`, `v0.2.0`, or `v1.0.0`.
- If a future major version introduces breaking changes after `v1`, use the
  standard Go module major-version suffix rules.

## Before Tagging

Review these files for public-facing accuracy:

- [`README.md`](./README.md)
- [`docs/guides/`](./docs/guides)
- [`examples/README.md`](./examples/README.md)
- [`LICENSE`](./LICENSE)

## After Tagging

- create a GitHub release note for the tag
- summarize breaking changes and migration notes when relevant
- keep examples and guides aligned with the released API

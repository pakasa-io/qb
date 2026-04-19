# Contributing

Thanks for contributing to `qb`.

## Development Setup

- Install Go `1.24` or newer.
- Clone the repository and work from a feature branch.
- Run `go test ./...` before opening a change.
- Run `gofmt -w` on edited Go files.

## Scope

Changes are easiest to review when they keep these pieces aligned:

- the public API
- runnable examples under [`examples/`](./examples)
- guides under [`docs/guides/`](./docs/guides)
- transport or adapter specs under [`docs/specs/`](./docs/specs) when behavior changes

## Pull Requests

Please keep pull requests focused and include:

- a short explanation of the user-visible change
- tests for behavior changes or regressions
- doc updates when public behavior, package layout, or examples change

## Design Notes

The project keeps the core `qb` AST independent from:

- HTTP frameworks
- SQL drivers
- ORMs
- transport-specific parsing details

New features should preserve that separation whenever possible. Transport
parsing belongs in `codecs`, backend rendering belongs in `adapters`, and
public-field policies belong in `schema`.

## Questions

If you are unsure where a change belongs, open an issue or draft pull request
first so the design can be reviewed before larger edits land.

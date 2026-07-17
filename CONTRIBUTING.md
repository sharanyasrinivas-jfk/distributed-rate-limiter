# Contributing

## Branching
Create a feature branch per change: `feature/short-description`, `fix/short-description`.

## Commit messages
Follow [Conventional Commits](https://www.conventionalcommits.org/):
`feat:`, `fix:`, `test:`, `docs:`, `refactor:`, `ci:`, `chore:`.

## Before opening a PR
```bash
go test ./... -race -cover
golangci-lint run
```
Both must pass. Include a test for any behavior change.

## PR expectations
- One logical change per PR.
- Description should explain *why*, not just *what*.
- Link any related issue.

## Code review checklist
- [ ] Tests cover the new/changed behavior
- [ ] No new race conditions (`-race` clean)
- [ ] Errors are handled, not swallowed
- [ ] Public functions have doc comments

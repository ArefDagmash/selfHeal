# Problem: Dockerfile named `Dockerfile.go` broke `go build ./...`

**Phase:** 5 — DevOps & Polish
**Step:** 5.2 — GitHub Actions CI (and 5.1 Dockerize follow-up)

## What happened

To distinguish the two Dockerfiles, the Go service's Dockerfile was named `Dockerfile.go` at the repo
root. `go build ./...` (and `go vet`/`go test ./...`, which the CI workflow runs) treats **every** file
matching `*.go` as Go source, so it tried to compile `Dockerfile.go` and failed:

```
Dockerfile.go:1:1: illegal character U+0023 '#'
```

This would have made the entire CI pipeline fail at the `go vet`/`go test` stage before any test ran.

## Fix

Renamed the Dockerfiles so they are not Go sources: `Dockerfile.go` → `docker/Dockerfile`, and
`Dockerfile.dashboard` → `dashboard/Dockerfile` (no `.go` extension). Updated `docker-compose.yml` to
point at `docker/Dockerfile` and `dashboard/Dockerfile`. Confirmed `go build ./...` and `go vet ./...`
are clean again.

Lesson: never give a non-Go file a `.go` extension inside a directory Go walks.

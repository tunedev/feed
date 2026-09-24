# feed

Normalised job postings from public board APIs and rendered careers pages. A scheduled GitHub
Action is the only writer. Nothing here is user data, ever.

| Path | Holds |
|---|---|
| `postings/<source>/<board>/<id>.json` | One posting's current state. `git log` on it is its history |
| `status/<source>/<board>.json` | The last crawl's outcome for that board |
| `runs/<timestamp>.json` | One manifest per crawl |
| `boards.yaml` | The whole coverage. Add a board by pull request |
| `schema/v1.md` | The document shape and how it is versioned |

Consumers pin a git ref: `refs/heads/main`, or a `pre-v<N>` tag to freeze before a breaking change.

```bash
go test ./...                # offline; rendered tests need a local Chrome
go run ./cmd/crawl -dry-run  # the whole pipeline against recorded responses
```

If the schedule stops (GitHub disables scheduled workflows on idle repositories):
`gh workflow enable crawl.yml -R tunedev/feed`.

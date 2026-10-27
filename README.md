# Mini-AI Code Reviewer

A small web app that reviews Go and Python snippets for logic errors. Paste code into a line-numbered editor, select its language, and receive affected lines, explanations, and suggested fixes.

## Features

- Review Go and Python snippets for logic errors.
- Edit code with syntax highlighting and visible line numbers.
- View affected lines, explanations, and suggested fixes.
- See loading, error, and no-findings states; editing clears previous results.
- Reuse successful reviews through an in-memory backend cache.

Fixes are suggestions only. The app does not execute submitted code or apply changes automatically.

## Tech Stack

- **Frontend:** React, TypeScript, Vite, and CodeMirror.
- **Backend:** Go with `net/http` and the standard library.
- **AI:** Groq REST API, using `openai/gpt-oss-120b` by default.

The frontend calls the Go API, which validates input and returns a cached review when available, or requests and validates a new review from Groq. The API key stays on the backend. The app uses local React state, with no database or persistent review history.

The concurrency-safe cache stores up to 100 successful results by language and normalized code, including empty findings. Errors are not cached, and restarting the backend clears the cache.

## Project Structure

```text
mini-ai-code-reviewer/
├── .github/workflows/
│   └── ci.yml                # Backend and frontend CI checks
├── backend/                  # Go API
│   ├── cmd/server/
│   │   └── main.go           # Configuration, routing, and server startup
│   ├── internal/
│   │   ├── api/              # HTTP handler and tests
│   │   ├── review/           # Review contract, validation, cache, and tests
│   │   └── groq/             # Provider client, prompt, and tests
│   ├── .env.example          # Example environment variables
│   └── go.mod                # Go module and version
├── frontend/                 # React application
│   ├── src/                  # Editor, results UI, and API client
│   ├── tests/                # Frontend tests
│   ├── package.json          # Dependencies and scripts
│   └── vite.config.ts        # Vite configuration and API proxy
├── evaluation/
│   ├── cases.md              # Evaluation snippets and expectations
│   └── evaluation.md         # Results and limitations
└── README.md
```

## Prerequisites

- Go 1.26.7 or later.
- Node.js 24 with npm.
- A Groq API key.

## Run Locally

### Backend — from the repository root:

```sh
cd backend
cp .env.example .env
```

Edit `.env` and set `GROQ_API_KEY`. Then load it into your shell and start the server (bash/zsh):

```sh
set -a
source .env
set +a
go run ./cmd/server
```

The app does not automatically load `.env`. Keep the key private; `.env` is ignored by Git.

| Variable       | Purpose / default                  |
| -------------- | ---------------------------------- |
| `GROQ_API_KEY` | Required; used only by the backend |
| `GROQ_MODEL`   | Defaults to `openai/gpt-oss-120b`  |
| `PORT`         | Defaults to `8080`                 |

### Frontend — in a second terminal, from the repository root:

```sh
cd frontend
npm ci
npm run dev
```

Open http://127.0.0.1:5173. Vite proxies `/lint` to the backend at `127.0.0.1:8080`. If you change the backend port, update the proxy target in `frontend/vite.config.ts` too.

## API

`POST /lint` accepts `code` and `language` (`go` or `python`):

```sh
curl http://127.0.0.1:8080/lint \
  -H 'Content-Type: application/json' \
  -d '{"language":"python","code":"def mean(values):\n    \"\"\"Return the mean of a nonempty list.\"\"\"\n    return sum(values) // len(values)"}'
```

Example success response (model wording may vary):

```json
{
  "findings": [
    {
      "start_line": 3,
      "end_line": 3,
      "explanation": "Floor division discards the fractional part of the mean.",
      "suggested_fix": "return sum(values) / len(values)"
    }
  ]
}
```

Line ranges are 1-based and inclusive. A valid review with no findings returns `{"findings":[]}`; this does not guarantee correct code.

Submit nonblank code up to 200 lines and 12 KiB. Each review returns at most five findings; input and results are never silently truncated.

Provider requests use a 30-second timeout. Failures return errors rather than empty findings; retry is manual.

Errors include a stable code and readable message: `{"error":{"code":"...","message":"..."}}`.

## Evaluation

The [14 evaluation cases](evaluation/cases.md) cover Go and Python across a six-case Development set and an eight-case Test set. Expectations were defined before runs; The Test set was evaluated with the finalized prompt and was not used for prompt tuning.

See [evaluation results](evaluation/evaluation.md) for the prompt refinement, results, metrics, and limitations. This small, manually assessed set is not a broad model accuracy claim. Response validation checks structure, not whether a finding or fix is correct.

## Tests

From the repository root:

```sh
(cd backend && go test ./... && go vet ./...)
(cd frontend && npm test && npm run build)
```

The frontend build includes TypeScript checking. Automated tests cover request handling, provider response validation, caching, and frontend request/result helpers without live AI calls.

[GitHub Actions CI](.github/workflows/ci.yml) runs backend tests and vet, plus frontend tests and build, on pushes and pull requests.

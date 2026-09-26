# E2E tests (Cucumber BDD)

End-to-end browser tests for the LLMate dashboard, written in Gherkin/Cucumber and run against a locally started gateway on `127.0.0.1:8099`.

## What's in this folder

| Path | What it is |
| --- | --- |
| `features/` | The test scenarios, written in Gherkin (`.feature` files). One file per area: `auth`, `models`, `providers`, `pricing`, `smoke`. |
| `cucumber/` | The step definitions and hooks that execute the scenarios: `*.steps.mjs` map Gherkin steps to Playwright actions, `world.mjs` holds the shared browser/page state, `hooks.mjs` launches the browser and captures screenshots. |
| `cucumber-run.sh` | Runner script: starts the gateway, waits for readiness, then runs cucumber-js and writes the HTML report. |
| `start-gateway.sh` | Starts the gateway (SPA + admin API) in the background for the local run. |
| `cucumber-report.html` | Generated HTML report from the last run. Screenshots are embedded inline per step (page visits and UI actions). |

## Running

From `frontend/`:

```sh
npm run test:e2e        # runs e2e/cucumber-run.sh
```

Then open `frontend/e2e/cucumber-report.html` in a browser to view the report with embedded screenshots.

## What's git-ignored

`test-results/`, `playwright-report/`, `screenshots/`, `cucumber-report.html`, and `.cucumber/` are generated artifacts and are not committed (see `frontend/.gitignore`).

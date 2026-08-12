# Regression testing policy

Every bug fix must include an automated regression test in the same change.

## Required fix workflow

1. Add the smallest test that reproduces the reported behavior and verify that it fails against the broken implementation.
2. Fix the implementation without weakening or deleting the reproduction.
3. Run the targeted test, then the complete suite for the affected application.
4. Record any behavior that cannot yet be automated as an explicit test gap; it is not considered covered merely because it was checked manually.

Tests should assert externally observable behavior and durable contracts. Avoid snapshots of entire components or assertions against incidental implementation details unless the bug is specifically about rendered structure.

## Where tests belong

| Change | Required test location |
|---|---|
| Vue interaction, navigation, local storage, or rendering | `frontend/src/**/*.test.ts` using Vitest and Vue Test Utils |
| Frontend API request/response contract | `frontend/src/services/*.test.ts` or `frontend/src/lib/*.test.ts` |
| Local tournament calculation and recovery | `frontend/src/game/*.test.ts` |
| Go HTTP/API behavior | `backend/internal/httpapi/*_test.go` |
| Go database behavior and transactions | matching `backend/internal/**/*_test.go` repository test |
| Existing Laravel behavior | `tests/Unit`, `tests/Feature`, or `tests/Browser` |
| Purely visual/responsive regression | component assertion plus a browser screenshot/viewport test once the browser test harness covers that page |

## Current separated-stack regression coverage

- `frontend/src/views/GameView.test.ts`: restart decision, continue, successful remount, failed restart preserving local progress, button-only voting, local-first persistence, history images, controls, ranking link, and video hover behavior.
- `frontend/src/views/HomeView.test.ts`: search query, tags, SPA hot/new navigation, featured YouTube carousel, winner/loser marquee, and deduplicated pagination.
- `frontend/src/game/localGame.test.ts`: bracket rules, durable outbox, reload reshuffle without progress loss, legacy migration, and tournament completion.
- `frontend/src/services/*.test.ts`: public-content and gameplay API contracts.
- `frontend/src/i18n.test.ts`, `frontend/src/content/legal.test.ts`, and `frontend/src/composables/useTheme.test.ts`: locale URLs, legal content, and theme persistence.
- `backend/internal/**/*_test.go`: Go configuration, authentication, public API, gameplay API, repositories, conflicts, and transaction behavior.

## Commands

```bash
cd frontend
npm test
npm run build

cd ../backend
go test ./...
go vet ./...
```

The separated-stack GitHub Actions workflow runs these checks for every relevant pull request and push.

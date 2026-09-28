# Plan: Unify & Organize the Settings UI

## High-Level Summary

The Settings tab today is a loose collection of disconnected pieces:
- a "Logging Configuration" card with its own **Save configuration** / **Reset** buttons
- a lone collapsed **Advanced** `<details>` section (HTTP idle timeout) that depends on the logging card's save button
- a separate **Log body retention** card with its own destructive **Save & apply** button + confirmation dialog

Meanwhile, the global **Require API keys** toggle — a true gateway setting — lives on the API Keys page instead of Settings.

This plan reorganizes `/settings` into one structured page of titled cards grouped by concern, with **a single unified save path** (one "Save configuration" button, one "Reset to defaults"), promotes the hidden Advanced `<details>` into a real card, moves **Require API keys** into Settings, and removes the duplicated save buttons/dialogs that currently make configuration feel scattered.

## Problems Being Solved

1. **Saving is scattered.** Three separate save paths exist on one page (logging card's button, retention card's "Save & apply", plus the Advanced field that silently depends on the logging button). A single Save button that persists all changed fields removes this confusion.
2. **The "solo advanced section"** is a hidden `<details>` that users must discover, and it's visually inconsistent with the cards around it.
3. **Retention uses a separate destructive workflow** ("Save & apply" + dialog) that diverges from the rest of the page.
4. **A global setting lives in the wrong place** — `require_api_keys` is gateway-level config, not a key-management concern, so it belongs in Settings.

## Proposed UI Structure

A single scrolling page with grouped, titled cards, each with a short description:

1. **General** — Require API keys (moved from API Keys page)
2. **Logging** — request/response body max bytes, track streaming (SSE lines), streaming buffer (KB)
3. **Log retention** — three independent day-count fields
4. **Advanced** — HTTP idle connection timeout (promoted from `<details>` to a real card)

### Unified Save Model

- One **Save configuration** button saves every dirty field across all cards in a single `PUT /admin/config`.
- One **Reset to defaults** button resets all fields.
- When retention day-counts are dirty at save time, the existing confirmation dialog is shown (it has destructive purge semantics); otherwise the save applies immediately.
- Per-field "Unsaved" hints are kept so users know what changed before saving.

---

## Mockups

> Text wireframes representing the intended UI. Styling follows existing shadcn-svelte card conventions.

### Mockup 1 — Current Settings page (`/settings`)

```
┌──────────────────────────────────────────────────────────────┐
│ Settings                                                     │
├──────────────────────────────────────────────────────────────┤
│ ┌─ Logging Configuration ─────────────────────────────────┐  │
│ │ Request body max (bytes)   [ 51200        ] 50 KB      │  │
│ │ Response body max (bytes)  [ 51200        ] 50 KB      │  │
│ │ Track streaming (SSE lines)               [toggle] ON  │  │
│ │ Streaming buffer (KB)      [ 10           ] 10 KB      │  │
│ │ [Save configuration] [Reset to defaults]               │  │
│ └─────────────────────────────────────────────────────────┘  │
│ ┌─ Advanced ─────────────────────────────────────────────┐  │
│ │ ▸ Advanced  (collapsed <details>)                     │  │
│ │   HTTP idle timeout (s) [ 90 ]  pool: 10–86400s       │  │
│ │   "Unsaved change — use Save configuration above"     │  │
│ └─────────────────────────────────────────────────────────┘  │
│ ┌─ Log body retention ───────────────────────────────────┐  │
│ │ Streaming chunk bodies (days) [ 30 ]  Saved: 30       │  │
│ │ Request bodies on log rows (days) [ 30 ]  Saved: 30   │  │
│ │ Response bodies on log rows (days) [ 30 ]  Saved: 30  │  │
│ │ [Save & apply]                                        │  │
│ └─────────────────────────────────────────────────────────┘  │
└──────────────────────────────────────────────────────────────┘
```

### Mockup 2 — Proposed Settings page (`/settings`)

```
┌──────────────────────────────────────────────────────────────┐
│ Settings                                     [Save] [Reset] │
│  Configure gateway, logging, and retention settings.        │
├──────────────────────────────────────────────────────────────┤
│ ┌─ General ──────────────────────────────────────────────┐  │
│ │ Require API keys                    [toggle] ON        │  │
│ │ Every gateway request must include a valid API key.    │  │
│ └─────────────────────────────────────────────────────────┘  │
│ ┌─ Logging ──────────────────────────────────────────────┐  │
│ │ Request body max (bytes)   [ 51200        ] 50 KB      │  │
│ │ Response body max (bytes)  [ 51200        ] 50 KB      │  │
│ │ Track streaming (SSE lines)               [toggle] ON  │  │
│ │ Streaming buffer (KB)      [ 10           ] 10 KB      │  │
│ └─────────────────────────────────────────────────────────┘  │
│ ┌─ Log retention ────────────────────────────────────────┐  │
│ │ Streaming chunk bodies (days) [ 30 ]  Saved: 30        │  │
│ │ Request bodies on log rows (days) [ 30 ]  Saved: 30    │  │
│ │ Response bodies on log rows (days) [ 30 ]  Saved: 30   │  │
│ └─────────────────────────────────────────────────────────┘  │
│ ┌─ Advanced ──────────────────────────────────────────────┐  │
│ │ HTTP idle connection timeout (s) [ 90 ] 10–86400s      │  │
│ │ How long outbound keep-alive connections may sit idle. │  │
│ └─────────────────────────────────────────────────────────┘  │
└──────────────────────────────────────────────────────────────┘
```

### Mockup 3 — API Keys page after change (`/keys`)

```
┌──────────────────────────────────────────────────────────────┐
│ API Keys                                                    │
│  (Require API keys toggle removed — now in Settings>General)│
│ ┌─ Create key ───────────────────────────────────────────┐  │
│ │ Name [ dev-key ]  RPM [      ]  TPM [      ]          │  │
│ │ [Create key]                                          │  │
│ └─────────────────────────────────────────────────────────┘  │
│ ┌─ Keys ──────────────────────────────────────────────────┐  │
│ │ Name     Created      RPM   TPM   Active   Actions     │  │
│ │ dev-key  …            —     —     Active     ⋮          │  │
│ └─────────────────────────────────────────────────────────┘  │
└──────────────────────────────────────────────────────────────┘
```

### Mockup 4 — Save flow with changed retention (dialog)

```
┌──────────────────────────────────────────────────────────────┐
│ Settings                                     [Save] [Reset] │
│ ...cards...                                                 │
│                                                             │
│ ┌─────────────────────────────────────────────────────────┐ │
│ │ Apply log retention?                                  │ │
│ │ This will save changed day counts and immediately and │ │
│ │ permanently clear stored text older than each policy: │ │
│ │ • Streaming chunks: 45 days                           │ │
│ │ • Request bodies: 30 days                             │ │
│ │ • Response bodies: 30 days                            │ │
│ │ ☐ I understand old body content will be removed.      │ │
│ │               [Cancel]  [Confirm and apply]           │ │
│ └─────────────────────────────────────────────────────────┘ │
└──────────────────────────────────────────────────────────────┘
```

---

## Implementation Details

### Files changed

**1. `frontend/src/routes/(dashboard)/settings/+page.svelte`** (main rewrite)
- Restructure into four grouped cards: **General**, **Logging**, **Log retention**, **Advanced**.
- Replace the `Logging Configuration` card's inline Save/Reset buttons with page-level **Save configuration** and **Reset to defaults** actions (top-right of header).
- Promote the Advanced `<details>` into a normal always-visible card.
- Keep existing per-field formatting helpers (`bytesToKB`, `formatBytes`, etc.).
- Track dirty state per field; compute a single dirty set across all cards.
- On **Save configuration**: collect all dirty fields into one `api.updateConfig({...})` call.
  - If any retention day-count fields are dirty, open the existing confirmation dialog first; on confirm, apply the full patch (retention + non-retention fields together).
  - Otherwise apply immediately.
- On **Reset to defaults**: reset every field to its default and clear dirty state.
- Add `require_api_keys` to the **General** card (loaded from `api.getConfig()` and saved via `api.updateConfig({ require_api_keys })`).
- Remove the "Unsaved change — use Save configuration on the card above" hint from Advanced; the unified page-level save makes it obsolete.

**2. `frontend/src/routes/(dashboard)/keys/+page.svelte`**
- Remove the "Require API keys" toggle card, its `requireAPIKeys`/`configReady`/`requireSaving` state, and the `handleRequireAPIKeys` handler.
- Keep key creation, listing, update, and delete unchanged.

**3. `frontend/src/lib/types/index.ts`**
- No changes needed — `Configuration` already includes `require_api_keys`.

**4. `frontend/src/lib/api/client.ts`**
- No changes needed — `getConfig()` / `updateConfig(Partial<Configuration>)` already support all fields.

### Behavior notes
- The retention confirmation dialog is preserved because retention has destructive purge semantics (clearing stored bodies). It is now triggered only when retention values are dirty at save time, instead of its own "Save & apply" button.
- The API Keys page no longer duplicates gateway config; Settings is the single home for it.
- All changes are frontend-only; no Go backend changes are required.

## Verification
- `cd frontend && npm run check` — typecheck the Svelte changes.
- `npm run build` in `frontend/` to confirm the static SPA builds.
- Manual: load `/settings`, toggle fields, confirm dirty hints, save with and without retention changes, verify the confirmation dialog appears only when retention is dirty, verify `/keys` no longer shows the toggle.

## Out of Scope (not in this change)
- Server-side validation or new config keys (none needed).
- Moving other settings pages into tabs; the grouped-card layout keeps the change minimal while unifying save behavior.

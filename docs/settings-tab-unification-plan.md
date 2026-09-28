# Settings Tab Unification — Plan

## Goal

Unify the current scattered settings UI into one organized **Settings** page with a single save flow, and move settings that currently live in other pages into Settings.

## Current State (problems)

Today settings are spread across the dashboard and save in multiple, inconsistent ways:

1. **`/settings`** currently only contains *Logging Configuration*:
   - Request/response body max (bytes)
   - Track streaming (SSE lines) toggle
   - Streaming buffer (KB)
   - A **solo "Advanced"** `<details>` element floating as its own card holding only the HTTP idle timeout — disconnected from everything else.
   - A **"Log body retention"** card with its own separate `Save & apply` button and a destructive confirmation dialog.
2. **`require_api_keys`** is configured on the **API Keys page** (`frontend/src/routes/(dashboard)/keys/+page.svelte`), not in Settings — so the access policy setting lives outside the settings area entirely.
3. **Multiple save points**: `Save configuration` (logging card), `Save & apply` (retention card), and a separate toggle on API Keys. Dirty/unsaved state is tracked per-card, so a change in one card is not reflected elsewhere (e.g. the Advanced card shows "Unsaved change — use Save configuration on the card above").

## Proposed Design (high level)

Reorganize `/settings` into a single page with four sections and **one shared save bar**:

- **General** — `Require API keys` toggle (moved from API Keys page).
- **Logging** — body max, track streaming, streaming buffer (existing fields).
- **Retention** — the three day counts (existing fields), now in a 3-column grid.
- **Advanced** — HTTP idle timeout, promoted from a floating `<details>` into a proper section card.

A single sticky `Save changes` / `Reset to defaults` bar tracks all sections together, eliminating the multiple save points and the "save on the card above" confusion.

## Screenshot Mockups

- **Before (current)** — `docs/assets/settings-tab-before.png`
- **After (proposed)** — `docs/assets/settings-tab-after.png`

---

## Implementation

### 1. Restructure the page

File: `frontend/src/routes/(dashboard)/settings/+page.svelte`

- Replace the single "Logging Configuration" page with a `Settings` page header: title `Settings` + one-line description ("Gateway-wide configuration. All changes are saved together from one bar.").
- Add a section navigation row (tabs): `General | Logging | Retention | Advanced`. Each tab maps to a Card.
- **General card**: add a `Require API keys` `Switch` bound to `formState.require_api_keys`.
- **Logging card**: keep the existing four fields (request body max, response body max, track streaming, streaming buffer) in the same order.
- **Retention card**: keep the three day counts; change layout from stacked rows to a `grid grid-cols-3` so each count is a labeled input in one row. Keep the destructive-action note ("Lowering a value immediately and permanently clears matching stored text on save. This cannot be undone.").
- **Advanced card**: replace the `<details>` element with a normal Card section containing the HTTP idle timeout field + description.

### 2. Single shared save bar

- Hoist the save controls out of the Logging card into a persistent bar beside the page header (top-right) and/or a sticky footer bar.
- One `dirty` computed signal derived from a single `formState` object covering **all** sections:
  ```ts
  const dirty = $derived(
    JSON.stringify(formState) !== JSON.stringify(baseline)
  );
  ```
- `Save changes` calls `api.updateConfig(formState)` once for the whole form.
- `Reset to defaults` resets every section from `baseline`/defaults.
- Show `Unsaved: N changes` in the sidebar footer (or near the bar) whenever `dirty` is true, and disable `Save changes` when clean.

### 3. Move `require_api_keys` from API Keys into Settings

File: `frontend/src/routes/(dashboard)/keys/+page.svelte`

- Remove the `Require API keys` toggle (and its `getConfig`/`updateConfig` wiring) from the keys page.
- Add `require_api_keys` to the Settings `formState` (General card).
- On load, Settings fetches config with `api.getConfig()` and seeds `formState`; on save it writes with `api.updateConfig(formState)` — the same endpoints already used today, so no backend change is required.

### 4. State / wiring

- Keep a single `onMount` that loads `api.getConfig()` and `api.getConfigDefinition()` once and seeds `formState` + `baseline`.
- Keep the retention confirmation dialog: on `Save changes`, if any retention value was lowered from baseline, show the existing confirm dialog before calling `updateConfig`; otherwise save directly.
- Remove the old per-card `Save configuration` and `Save & apply` buttons.

### 5. Verification

- `go build ./...` (no Go changes expected).
- `npm run check` in `frontend/` after editing the Svelte files.
- Manual: edit a value in Advanced → sidebar shows "Unsaved: 1 change"; `Save changes` persists all sections; lowering a retention value triggers the confirmation dialog.
- Regression: API Keys page no longer saves `require_api_keys`; Settings does.

## Files touched

- `frontend/src/routes/(dashboard)/settings/+page.svelte` — restructure into 4 sections + shared save bar.
- `frontend/src/routes/(dashboard)/keys/+page.svelte` — remove the `require_api_keys` toggle.
- `frontend/src/lib/types/index.ts` — (optional) add `require_api_keys` to the local `formState` shape if not already present in `Configuration`.

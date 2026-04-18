# Stash Development Guide

> **⚠️ ALWAYS REBUILD AND RESTART DOCKER CONTAINERS AFTER ANY CODE CHANGE ⚠️**
>
> This project runs from the `stash/build:latest` image — source changes are not reflected in the running container until the image is rebuilt. After editing any backend or frontend code, you MUST:
> 1. Run `make docker-build` (full frontend + backend compile)
> 2. Restart the container: `cd docker/production && docker compose up -d --force-recreate stash`
>
> This applies to every change, no matter how small. Do not report a task as "done" before the container has been rebuilt and restarted.

## Project Overview

Stash is a self-hosted web application for organizing, managing, and serving video/image content collections with metadata enrichment capabilities. This is a fork of [stashapp/stash](https://github.com/stashapp/stash).

**License:** AGPL-3.0

## Tech Stack

### Backend (Go 1.24.3)
- **GraphQL:** gqlgen v0.17.73 for API layer
- **HTTP:** chi/v5 router
- **Database:** SQLite with sqlx
- **WebSockets:** gorilla/websocket
- **Scraping:** chromedp for browser automation

### Frontend (React + TypeScript)
- **Build:** Vite 5.4.21
- **Framework:** React 17.0.2
- **GraphQL Client:** Apollo Client v3.8.10
- **UI:** Ant Design + React Bootstrap
- **i18n:** React Intl (32+ languages)
- **Video:** Video.js v7.21.3
- **PWA:** vite-plugin-pwa + Workbox (service worker, offline support)

### External Dependencies
- FFmpeg (required for video processing)

## Directory Structure

```
/opt/stash/
├── cmd/stash/              # Main application entry point
├── cmd/phasher/            # Hash utility binary
├── internal/
│   ├── api/                # GraphQL resolvers and REST routes
│   ├── manager/            # Core business logic and config
│   ├── desktop/            # Desktop app wrapper
│   ├── dlna/               # DLNA/UPnP server
│   └── autotag/            # Auto-tagging system
├── pkg/                    # Public reusable packages (28 modules)
│   ├── models/             # Data models (Scene, Performer, etc.)
│   ├── sqlite/             # Database layer
│   ├── scraper/            # Scraping framework
│   ├── plugin/             # Plugin system
│   └── ffmpeg/             # FFmpeg integration
├── ui/v2.5/                # React frontend
│   ├── src/components/     # React components
│   ├── src/core/           # Core utilities
│   ├── src/sw.ts           # Service worker (Workbox injectManifest)
│   ├── graphql/            # GraphQL operations
│   └── vite.config.js      # Build config (includes PWA plugin)
├── graphql/schema/         # GraphQL type definitions
├── docker/                 # Container configurations
└── scripts/                # Build and utility scripts
```

## Development Commands

```bash
# Initial setup
make pre-ui              # Install UI dependencies (pnpm)
make generate            # Generate GraphQL code (backend + frontend)

# Development servers
make server-start        # Backend dev server (port 9999, uses .local/)
make ui-start            # Frontend dev server (port 3000)

# Building
make stash               # Build main binary
make phasher             # Build hash utility
make build               # Build both binaries
make build-release       # Optimized release builds

# Testing & Validation
make validate            # Run all linting and tests
make it                  # Integration tests
go test ./...            # Unit tests
go test -tags integration ./...  # Integration tests

# Cross-compilation
make build-cc-windows    # Windows
make build-cc-macos      # macOS (Intel + ARM universal)
make build-cc-linux      # Linux x86_64
make build-cc-all        # All platforms
```

## Key Files

| File | Purpose |
|------|---------|
| `cmd/stash/main.go` | Application entry point |
| `internal/api/server.go` | HTTP server setup (includes CSP headers) |
| `internal/api/resolver.go` | GraphQL resolver root |
| `internal/api/resolver_*.go` | GraphQL resolvers by type |
| `internal/manager/config/` | Configuration management |
| `graphql/schema/types/` | GraphQL schema definitions |
| `gqlgen.yml` | GraphQL code generation config |
| `ui/v2.5/src/index.tsx` | Frontend entry point (service worker registration) |
| `ui/v2.5/src/sw.ts` | Service worker source (Workbox) |
| `ui/v2.5/vite.config.js` | Vite build config with PWA plugin |
| `Makefile` | Build system (438 lines) |

## GraphQL Development

- Schema files: `graphql/schema/types/*.graphql`
- Generated code: `internal/api/generated_*.go` (auto-generated, do not edit)
- Resolvers: `internal/api/resolver_*.go`
- Run `make generate` after schema changes

## Database

- SQLite embedded database
- Migrations: `pkg/sqlite/migrations/`
- Dev database: `.local/stash.sqlite`
- Dev config: `.local/config.yml`

## Plugin System

- JavaScript/Python sandboxed execution
- Plugin manifests: YAML files with `source.yml`
- Example plugin: `PerformerDetailsExtended-main/`

## Linting

Configured linters (`.golangci.yml`):
- errcheck, govet, staticcheck, typecheck, unused
- gofmt, copyloopvar, errorlint, gocritic, misspell, revive

## Fork Modifications

**Repository:** https://github.com/justinswood/stash (fork of stashapp/stash)
**Branch:** develop

### Custom Features Added:
1. **UI toggle to hide male/trans performers** (committed)
2. **Performer Details Extended** - Enhanced performer statistics
   - `graphql/schema/types/performer_details_extended.graphql`
   - `internal/api/resolver_query_performer_details_extended.go`
   - `pkg/models/performer_details_extended.go`
   - `ui/v2.5/src/components/Performers/PerformerDetails/PerformerDetailsExtended.tsx`
   - `ui/v2.5/graphql/queries/performerDetailsExtended.graphql`
3. **Title Cleanup Task** - Cleans up scene and image titles
   - `internal/manager/task_cleanup_titles.go` (processes both scenes and images)
   - `scripts/stash_cleanup_titles.py`
4. **Performer Image Cropper** - Native Cropper.js integration (ported from plugin v0.3.3)
   - Uses Cropper.js v1.6.1 to crop performer images directly on the performer detail page
   - Saves cropped image via `usePerformerUpdate` GraphQL mutation as base64 data URL
   - `ui/v2.5/src/components/Performers/PerformerDetails/PerformerImageCropper.tsx` (new component)
   - `ui/v2.5/src/components/Performers/PerformerDetails/Performer.tsx` (integrated cropper + LightboxLink DOM stability fix)
   - `ui/v2.5/src/index.scss` (cropper styles + `.cropper-view-box img { transition: none }` fix)
   - `ui/v2.5/src/locales/en-GB.json` (added `actions.crop_image`)
   - `ui/v2.5/package.json` / `pnpm-lock.yaml` (added cropperjs dependency)
   - **Key architecture note:** The performer image must always remain inside `<LightboxLink>` in the React tree (never conditionally moved in/out) to preserve DOM node stability for Cropper.js. An `onClickCapture` wrapper blocks lightbox clicks during cropping.
5. **PerformerDetailsExtended Plugin** (standalone)
   - `PerformerDetailsExtended-main/`
6. **Batch Save** - Native tagger batch save (ported from Stash Batch Save plugin v0.6)
   - Adds "Save All" button to scene tagger with progress bar and stop functionality
   - Callback registration pattern: search results register save callbacks, batch save iterates them
   - `ui/v2.5/src/components/Tagger/context.tsx` (registerSaveCallback, doBatchSave, stopBatchSave)
   - `ui/v2.5/src/components/Tagger/scenes/SceneTagger.tsx` (Save All button + progress bar)
   - `ui/v2.5/src/components/Tagger/scenes/StashSearchResult.tsx` (callback registration)
   - `ui/v2.5/src/components/Tagger/scenes/TaggerScene.tsx` (dismiss button)
7. **Batch Search** - Native tagger batch search (ported from Stash Batch Search plugin v0.4.3)
   - Adds "Search All" button to both scene tagger and performer tagger
   - Scene tagger: generates query strings per scene using prepareQueryString, calls API sequentially with 200ms delay
   - Performer tagger: searches untagged performers by name sequentially
   - Both support progress tracking and cancellation
   - `ui/v2.5/src/components/Tagger/context.tsx` (sceneQuerySearch internal helper, doBatchSearch, stopBatchSearch)
   - `ui/v2.5/src/components/Tagger/scenes/SceneTagger.tsx` (Search All button + progress bar)
   - `ui/v2.5/src/components/Tagger/performers/PerformerTagger.tsx` (Search All button + batch search)
8. **Scene sorting fix** - `created_at`/`updated_at` sorting with mixed timezone formats
   - SQLite text comparison failed with mixed `Z` vs `-06:00` timezone suffixes
   - Fixed in `pkg/sqlite/scene.go` using `datetime()` SQL function for normalization
9. **Task queue ETA fix** - Sliding window rate estimation for accurate time remaining
   - `ui/v2.5/src/components/Settings/Tasks/JobTable.tsx` (60-second sliding window, precise duration formatting)
   - `pkg/job/job.go` (fixed inverted TimeElapsed logic)
10. **Qx Scene Card** - Native scene card redesign (ported from Qx Scene Card plugin v1.1)
    - Inline performer names with gender coloring (pink/blue/plum) and hover popovers with age at scene date
    - Footer with views (left) and date (right), watched scenes faded
    - 5 toggle settings under Interface > Scene Cards: fade watched, hide markers, hide groups, hide o-counter, hide studio
    - `ui/v2.5/src/components/Scenes/SceneCard.tsx` (SceneCardFooter, modified Details/Popovers/Overlays)
    - `ui/v2.5/src/components/Scenes/SceneCardPerformerList.tsx` (new: performer list + hover popover)
    - `ui/v2.5/src/components/Settings/SettingsInterfacePanel/SettingsInterfacePanel.tsx` (Scene Cards section)
    - `ui/v2.5/src/core/config.ts` (5 new IUIConfig settings)
    - `ui/v2.5/src/index.scss` (gender colors, footer layout, watched fade, performer popover styles)
    - `ui/v2.5/src/locales/en-GB.json` (scene card setting strings)
    - **Key architecture note:** Performer list and footer use React fragments (not wrapper divs) to be direct flex children of `.card-section`, enabling CSS `order` for layout control.
11. **StashDB Navbar Search** - Scrape performers from StashDB when no local results found
    - Adds "Search StashDB" button in performer search dropdown when no matches
    - Opens `PerformerStashBoxModal`, on selection navigates to `/performers/new` with scraped data
    - Route state passes `scrapeResult` + `stashBoxEndpoint` through PerformerCreate → PerformerEditPanel
    - `ui/v2.5/src/components/MainNavbar.tsx` (modal state, StashDB search button, performer selection handler)
    - `ui/v2.5/src/components/Performers/PerformerDetails/PerformerCreate.tsx` (reads route state)
    - `ui/v2.5/src/components/Performers/PerformerDetails/PerformerEditPanel.tsx` (auto-apply scraped data, guard in updateStashIDs)
    - **Key architecture note:** `stashBoxEndpoint` is passed via route state because the normal scraper selection flow isn't used; stash_ids are set directly in a useEffect rather than through `updateStashIDs`.
12. **Navbar Scan Library Dropdown** - Scan individual libraries from navbar
    - Replaces single scan button with dropdown showing configured library paths
    - "Scan All Libraries" option at top, individual paths below
    - Reads library paths from `configuration.general.stashes`
    - `ui/v2.5/src/components/MainNavbar.tsx` (dropdown menu, library path iteration)
    - `ui/v2.5/src/index.scss` (dropdown alignment styles)
13. **PWA (Progressive Web App)** - Installable app with mobile-optimized navigation and playback
    - **Service Worker:** Custom Workbox `injectManifest` strategy with caching:
      - CacheFirst for scene/performer/studio images (500 entries, 7-day expiry)
      - CacheFirst for static assets (1-year expiry)
      - NetworkFirst for navigation with offline fallback
      - NetworkOnly for video streams (never cached)
    - **Mobile Bottom Navigation:** Separate `MobileBottomNav` component (not restructured MainNavbar)
      - 5 tabs: Scenes, Images, Performers, Search, More
      - "More" opens slide-up sheet with remaining sections
      - Only visible on xs portrait screens; top navbar hidden on mobile portrait
      - Respects user's `menuItems` configuration
    - **Enhanced Mobile Player:**
      - Picture-in-Picture enabled on supported browsers
      - Touch controls enabled: tap left/right to seek +-10s (videojs-mobile-ui)
      - Swipe-to-seek plugin: horizontal swipe for +-10s with visual overlay
      - Media Session `seekto` handler + `setPositionState` for lock screen seek bar
      - VTT thumbnail scrubber visible on larger mobile screens
    - **Install/Update Prompts:**
      - Install banner captures `beforeinstallprompt` (Chrome/Edge) or shows manual iOS instructions
      - Dismissal stored in LocalForage, re-shows after 30 days
      - Update toast when service worker detects new version, calls `skipWaiting()` + reload
    - **Files:**
      - `internal/api/server.go` (CSP: added `'self'` to `worker-src` directive)
      - `ui/v2.5/vite.config.js` (vite-plugin-pwa with injectManifest, compression filter excludes sw.js)
      - `ui/v2.5/src/sw.ts` (custom service worker with Workbox precaching + runtime caching)
      - `ui/v2.5/src/serviceWorker.ts` (replaced CRA code with workbox-window wrapper)
      - `ui/v2.5/src/index.tsx` (service worker registration with update event dispatch)
      - `ui/v2.5/public/manifest.json` (updated: scope, shortcuts, orientation, split icon purposes)
      - `ui/v2.5/public/offline.html` (self-contained offline fallback page)
      - `ui/v2.5/index.html` (iOS meta tags: apple-mobile-web-app-capable, status-bar-style, viewport-fit)
      - `ui/v2.5/src/components/MobileBottomNav/MobileBottomNav.tsx` + `.scss`
      - `ui/v2.5/src/components/PWAInstallPrompt/PWAInstallPrompt.tsx` + `.scss`
      - `ui/v2.5/src/components/PWAUpdatePrompt/PWAUpdatePrompt.tsx` + `.scss`
      - `ui/v2.5/src/components/ScenePlayer/ScenePlayer.tsx` (PiP, touch controls, scrubber on mobile)
      - `ui/v2.5/src/components/ScenePlayer/media-session.ts` (seekto, setPositionState, timeupdate)
      - `ui/v2.5/src/components/ScenePlayer/swipe-seek.ts` (new: swipe-to-seek plugin)
      - `ui/v2.5/src/App.tsx` (renders MobileBottomNav, PWAInstallPrompt, PWAUpdatePrompt)
      - `ui/v2.5/src/index.scss` (mobile bottom nav: hide top-nav on xs portrait, 56px body padding)
      - `ui/v2.5/package.json` / `pnpm-lock.yaml` (added vite-plugin-pwa, workbox-window, workbox-*)

### Modified Files (unstaged):
- `graphql/schema/schema.graphql` - Schema extensions
- `internal/api/resolver_mutation_metadata.go` - Metadata mutations
- `internal/api/server.go` - CSP worker-src fix for PWA
- `internal/manager/manager_tasks.go` - Task management
- `internal/manager/task_cleanup_titles.go` - Image title cleanup support
- `pkg/models/repository_performer.go` - Performer repository
- `pkg/sqlite/performer.go` - Performer database layer
- `pkg/sqlite/scene.go` - Scene sorting datetime fix
- `pkg/job/job.go` - TimeElapsed fix
- `ui/v2.5/vite.config.js` - PWA plugin configuration
- `ui/v2.5/index.html` - iOS PWA meta tags
- `ui/v2.5/src/index.tsx` - Service worker registration
- `ui/v2.5/src/serviceWorker.ts` - Workbox-window wrapper
- `ui/v2.5/src/sw.ts` - Custom service worker
- `ui/v2.5/src/App.tsx` - MobileBottomNav, PWA prompts
- `ui/v2.5/src/components/MainNavbar.tsx` - StashDB search, scan library dropdown
- `ui/v2.5/src/components/MobileBottomNav/MobileBottomNav.tsx` + `.scss` - Mobile bottom nav
- `ui/v2.5/src/components/PWAInstallPrompt/PWAInstallPrompt.tsx` + `.scss` - Install prompt
- `ui/v2.5/src/components/PWAUpdatePrompt/PWAUpdatePrompt.tsx` + `.scss` - Update prompt
- `ui/v2.5/src/components/Performers/PerformerDetails/Performer.tsx` - Image cropper integration
- `ui/v2.5/src/components/Performers/PerformerDetails/PerformerImageCropper.tsx` - New cropper component
- `ui/v2.5/src/components/Performers/PerformerDetails/PerformerCreate.tsx` - Route state for StashDB scrape
- `ui/v2.5/src/components/Performers/PerformerDetails/PerformerEditPanel.tsx` - Auto-apply scraped data
- `ui/v2.5/src/components/Performers/PerformerDetails/PerformerDetailsPanel.tsx`
- `ui/v2.5/src/components/Scenes/SceneCard.tsx` - Scene card redesign
- `ui/v2.5/src/components/Scenes/SceneCardPerformerList.tsx` - Performer list component
- `ui/v2.5/src/components/ScenePlayer/ScenePlayer.tsx` - PiP, touch controls, scrubber
- `ui/v2.5/src/components/ScenePlayer/media-session.ts` - Seekto, position state
- `ui/v2.5/src/components/ScenePlayer/swipe-seek.ts` - Swipe-to-seek plugin
- `ui/v2.5/src/components/Settings/SettingsInterfacePanel/SettingsInterfacePanel.tsx` - Scene card settings
- `ui/v2.5/src/components/Settings/Tasks/LibraryTasks.tsx`
- `ui/v2.5/src/components/Settings/Tasks/JobTable.tsx` - ETA sliding window
- `ui/v2.5/src/components/Tagger/context.tsx` - Batch save + batch search
- `ui/v2.5/src/components/Tagger/scenes/SceneTagger.tsx` - Save All + Search All buttons
- `ui/v2.5/src/components/Tagger/scenes/StashSearchResult.tsx` - Save callback registration
- `ui/v2.5/src/components/Tagger/scenes/TaggerScene.tsx` - Dismiss button
- `ui/v2.5/src/components/Tagger/performers/PerformerTagger.tsx` - Performer batch search
- `ui/v2.5/src/core/config.ts` - Scene card IUIConfig settings
- `ui/v2.5/src/core/StashService.ts` - Service layer
- `ui/v2.5/src/index.scss` - Scene card styles, cropper styles, mobile nav, PWA styles
- `ui/v2.5/src/locales/en-GB.json` - Locale strings
- `ui/v2.5/public/manifest.json` - PWA manifest
- `ui/v2.5/public/offline.html` - Offline fallback page
- `ui/v2.5/package.json` / `pnpm-lock.yaml` - cropperjs, workbox, vite-plugin-pwa dependencies
- Docker configuration files

## Development Workflow

1. Make schema changes in `graphql/schema/types/`
2. Run `make generate` to regenerate code
3. Implement resolvers in `internal/api/resolver_*.go`
4. Add frontend components in `ui/v2.5/src/components/`
5. Run `make validate` before committing
6. **Always rebuild and restart Docker containers after any code changes** (backend or frontend)

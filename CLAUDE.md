# Stash Development Guide

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
- **Build:** Vite
- **Framework:** React 17.0.2
- **GraphQL Client:** Apollo Client v3.8.10
- **UI:** Ant Design + React Bootstrap
- **i18n:** React Intl (32+ languages)
- **Video:** Video.js v7.21.3

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
│   ├── graphql/            # GraphQL operations
│   └── vite.config.js      # Build config
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
| `internal/api/server.go` | HTTP server setup |
| `internal/api/resolver.go` | GraphQL resolver root |
| `internal/api/resolver_*.go` | GraphQL resolvers by type |
| `internal/manager/config/` | Configuration management |
| `graphql/schema/types/` | GraphQL schema definitions |
| `gqlgen.yml` | GraphQL code generation config |
| `ui/v2.5/src/index.tsx` | Frontend entry point |
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
3. **Title Cleanup Task**
   - `internal/manager/task_cleanup_titles.go`
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

### Modified Files (unstaged):
- `graphql/schema/schema.graphql` - Schema extensions
- `internal/api/resolver_mutation_metadata.go` - Metadata mutations
- `internal/manager/manager_tasks.go` - Task management
- `pkg/models/repository_performer.go` - Performer repository
- `pkg/sqlite/performer.go` - Performer database layer
- `ui/v2.5/src/components/MainNavbar.tsx` - Navigation
- `ui/v2.5/src/components/Performers/PerformerDetails/Performer.tsx` - Image cropper integration
- `ui/v2.5/src/components/Performers/PerformerDetails/PerformerImageCropper.tsx` - New cropper component
- `ui/v2.5/src/components/Performers/PerformerDetails/PerformerDetailsPanel.tsx`
- `ui/v2.5/src/components/Settings/Tasks/LibraryTasks.tsx`
- `ui/v2.5/src/core/StashService.ts` - Service layer
- `ui/v2.5/src/index.scss` - Cropper styles
- `ui/v2.5/src/locales/en-GB.json` - Crop image locale string
- `ui/v2.5/package.json` / `pnpm-lock.yaml` - cropperjs dependency
- Docker configuration files

## Development Workflow

1. Make schema changes in `graphql/schema/types/`
2. Run `make generate` to regenerate code
3. Implement resolvers in `internal/api/resolver_*.go`
4. Add frontend components in `ui/v2.5/src/components/`
5. Run `make validate` before committing

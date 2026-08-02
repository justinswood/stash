package api

import (
	"context"
	"strings"

	"github.com/99designs/gqlgen/graphql"
	"github.com/stashapp/stash/pkg/models"
)

// readOnlyMutations may be performed by any authenticated user, including the
// READ_ONLY role: recording one's own view/o history and changing one's own
// password.
var readOnlyMutations = map[string]bool{
	"sceneSaveActivity":       true,
	"sceneAddPlay":            true,
	"sceneDeletePlay":         true,
	"sceneIncrementPlayCount": true,
	"sceneResetPlayCount":     true,
	"sceneResetActivity":      true,
	"sceneIncrementO":         true,
	"sceneDecrementO":         true,
	"sceneAddO":               true,
	"sceneDeleteO":            true,
	"sceneResetO":             true,
	"imageIncrementO":         true,
	"imageDecrementO":         true,
	"imageResetO":             true,
	"changePassword":          true,
}

// adminMutations are system/library/configuration operations restricted to
// admins. All "*Destroy" mutations are additionally admin-only (see
// requiredRoleForMutation) since the USER role does not delete content.
var adminMutations = map[string]bool{
	"configureGeneral":        true,
	"configureInterface":      true,
	"configureDLNA":           true,
	"configureScraping":       true,
	"configureDefaults":       true,
	"configurePlugin":         true,
	"configureUI":             true,
	"configureUISetting":      true,
	"generateAPIKey":          true,
	"metadataScan":            true,
	"metadataGenerate":        true,
	"metadataAutoTag":         true,
	"metadataClean":           true,
	"metadataCleanGenerated":  true,
	"metadataIdentify":        true,
	"migrate":                 true,
	"migrateBlobs":            true,
	"migrateSceneScreenshots": true,
	"backupDatabase":          true,
	"anonymiseDatabase":       true,
	"exportObjects":           true,
	"importObjects":           true,
	"installPackages":         true,
	"updatePackages":          true,
	"uninstallPackages":       true,
	"enableDLNA":              true,
	"disableDLNA":             true,
	"addTempDLNAIP":           true,
	"removeTempDLNAIP":        true,
	"setPluginsEnabled":       true,
	"runPluginOperation":      true,
	"runPluginTask":           true,
	"setup":                   true,
	"stopJob":                 true,
	"execSQL":                 true,
	"querySQL":                true,
	"deleteFiles":             true,
	"moveFiles":               true,
	"userCreate":              true,
	"userUpdate":              true,
	"userDestroy":             true,
}

// requiredRoleForMutation returns the minimum role required to run a mutation.
func requiredRoleForMutation(field string) models.UserRole {
	if readOnlyMutations[field] {
		return models.UserRoleReadOnly
	}
	// Share links are a first-class user feature (create/update/revoke/destroy).
	if strings.HasPrefix(field, "shareLink") {
		return models.UserRoleUser
	}
	// API keys are managed by their owner; the resolvers require admin only to
	// act on another account's keys. Note the mutation is userAPIKeyRevoke, not
	// ...Destroy — renaming it would silently make it admin-only via the suffix
	// rule below and stop users revoking their own keys.
	if strings.HasPrefix(field, "userAPIKey") {
		return models.UserRoleUser
	}
	if adminMutations[field] || strings.HasSuffix(field, "Destroy") {
		return models.UserRoleAdmin
	}
	return models.UserRoleUser
}

// adminQueries are reads that expose the host system rather than the library:
// filesystem contents, logs, job state, plugin/package internals and scraper
// configuration. Browsing the collection is open to every role, but none of
// these are part of browsing.
var adminQueries = map[string]bool{
	// system / operational state
	"logs":         true,
	"jobQueue":     true,
	"findJob":      true,
	"systemStatus": true,
	"dlnaStatus":   true,

	// filesystem. `directory` walks arbitrary host paths, and the file/folder
	// finders return real paths — this is the most sensitive group here.
	"directory":             true,
	"findFile":              true,
	"findFiles":             true,
	"findFolder":            true,
	"findFolders":           true,
	"findScenesByPathRegex": true,
	"parseSceneFilenames":   true,

	// plugin / package / scraper configuration.
	//
	// NOTE: `plugins` is deliberately NOT here. PluginsLoader (App.tsx, via
	// src/plugins.tsx) calls it on every page load for every account to fetch
	// the plugin JS/CSS list — gating it breaks the app for non-admins. Only
	// the management surfaces below are Settings-only and safe to restrict.
	"pluginTasks":                 true,
	"installedPackages":           true,
	"availablePackages":           true,
	"listScrapers":                true,
	"validateStashBoxCredentials": true,
}

// userQueries require at least USER. Share link reads sit here to match the
// shareLink* mutations, which are USER-level.
var userQueries = map[string]bool{
	"findShareLinks": true,
	"findShareLink":  true,
}

// requiredRoleForQuery returns the minimum role required to run a query.
// Content reads default to READ_ONLY: every authenticated role may browse the
// library. Content *scoping* — which scenes a given user may see — is a
// separate concern and is not expressed here.
func requiredRoleForQuery(field string) models.UserRole {
	if adminQueries[field] {
		return models.UserRoleAdmin
	}
	if userQueries[field] {
		return models.UserRoleUser
	}
	// Scrapers fetch on the server's behalf — scrapeURL will retrieve an
	// arbitrary URL — and scraping is part of editing metadata, which READ_ONLY
	// cannot do.
	if strings.HasPrefix(field, "scrape") {
		return models.UserRoleUser
	}
	return models.UserRoleReadOnly
}

// exemptQueries bypass the role check entirely. `me` must answer for anyone,
// including an unauthenticated caller (it returns null), because the UI uses it
// to discover whether there is a session at all.
var exemptQueries = map[string]bool{
	"me": true,
}

// permissionMiddleware enforces role requirements on root Query and Mutation
// fields. Nested resolvers pass through untouched — a field reached from an
// already-authorised root is not re-checked.
func (r *Resolver) permissionMiddleware() graphql.FieldMiddleware {
	return func(ctx context.Context, next graphql.Resolver) (interface{}, error) {
		fc := graphql.GetFieldContext(ctx)
		if fc == nil {
			return next(ctx)
		}

		var required models.UserRole
		switch fc.Object {
		case "Mutation":
			required = requiredRoleForMutation(fc.Field.Name)
		case "Query":
			if exemptQueries[fc.Field.Name] {
				return next(ctx)
			}
			required = requiredRoleForQuery(fc.Field.Name)
		default:
			return next(ctx)
		}

		if err := r.requireRole(ctx, required); err != nil {
			return nil, err
		}
		return next(ctx)
	}
}

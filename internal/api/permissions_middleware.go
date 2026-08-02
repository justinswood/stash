package api

import (
	"context"
	"strings"

	"github.com/99designs/gqlgen/graphql"
	"github.com/stashapp/stash/pkg/models"
)

// Operations are classified by capability, not by role. Roles are presets over
// capabilities (models.CapabilitiesForRole), chosen so that an instance with no
// per-user overrides behaves exactly as it did when these tables held roles.
//
// When adding an operation: the *default* is the least privileged thing that
// still makes sense — CapEditMetadata for mutations, CapViewLibrary for
// queries. Anything that touches the host rather than the library must be
// listed explicitly; it will not be caught by a default.

// ownHistoryMutations are a user acting on their own playback record.
var ownHistoryMutations = map[string]bool{
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
}

// mutationCapabilities maps mutations that are not covered by a prefix/suffix
// rule or the default.
var mutationCapabilities = map[string]models.Capability{
	"changePassword": models.CapChangeOwnPassword,

	// account management
	"userGroupCreate":  models.CapManageUsers,
	"userGroupUpdate":  models.CapManageUsers,
	"userGroupDestroy": models.CapManageUsers,
	"userCreate":       models.CapManageUsers,
	"userUpdate":       models.CapManageUsers,
	"userDestroy":      models.CapManageUsers,

	// instance configuration
	"configureGeneral":   models.CapConfigure,
	"configureInterface": models.CapConfigure,
	"configureDLNA":      models.CapConfigure,
	"configureScraping":  models.CapConfigure,
	"configureDefaults":  models.CapConfigure,
	"configureUI":        models.CapConfigure,
	"configureUISetting": models.CapConfigure,
	"generateAPIKey":     models.CapConfigure,
	"setup":              models.CapConfigure,
	"enableDLNA":         models.CapConfigure,
	"disableDLNA":        models.CapConfigure,
	"addTempDLNAIP":      models.CapConfigure,
	"removeTempDLNAIP":   models.CapConfigure,

	// library tasks
	"metadataScan":            models.CapRunTasks,
	"metadataGenerate":        models.CapRunTasks,
	"metadataAutoTag":         models.CapRunTasks,
	"metadataClean":           models.CapRunTasks,
	"metadataCleanGenerated":  models.CapRunTasks,
	"metadataIdentify":        models.CapRunTasks,
	"migrate":                 models.CapRunTasks,
	"migrateBlobs":            models.CapRunTasks,
	"migrateSceneScreenshots": models.CapRunTasks,
	"backupDatabase":          models.CapRunTasks,
	"anonymiseDatabase":       models.CapRunTasks,
	"exportObjects":           models.CapRunTasks,
	"importObjects":           models.CapRunTasks,
	"stopJob":                 models.CapRunTasks,

	// plugins and packages
	"configurePlugin":    models.CapManagePlugins,
	"setPluginsEnabled":  models.CapManagePlugins,
	"runPluginOperation": models.CapManagePlugins,
	"runPluginTask":      models.CapManagePlugins,
	"installPackages":    models.CapManagePlugins,
	"updatePackages":     models.CapManagePlugins,
	"uninstallPackages":  models.CapManagePlugins,

	// raw database access
	"execSQL":  models.CapExecuteSQL,
	"querySQL": models.CapExecuteSQL,

	// destructive file operations. These do not end in "Destroy", so the suffix
	// rule below would not catch them — they must stay listed by hand.
	"deleteFiles": models.CapDeleteContent,
	"moveFiles":   models.CapDeleteContent,
}

// requiredCapabilityForMutation returns the capability a mutation requires.
func requiredCapabilityForMutation(field string) models.Capability {
	if ownHistoryMutations[field] {
		return models.CapOwnHistory
	}
	if c, ok := mutationCapabilities[field]; ok {
		return c
	}
	// Share links publish content to the open internet without authentication.
	if strings.HasPrefix(field, "shareLink") {
		return models.CapManageShares
	}
	// API keys are managed by their owner; the resolvers additionally require
	// CapManageUsers to act on another account's keys. Note the mutation is
	// userAPIKeyRevoke, not ...Destroy — renaming it would silently route it to
	// CapDeleteContent via the suffix rule and stop users revoking their own keys.
	if strings.HasPrefix(field, "userAPIKey") {
		return models.CapManageOwnAPIKeys
	}
	if strings.HasSuffix(field, "Destroy") {
		return models.CapDeleteContent
	}
	return models.CapEditMetadata
}

// queryCapabilities maps queries that expose the host rather than the library.
var queryCapabilities = map[string]models.Capability{
	// operational state
	"logs":         models.CapViewSystem,
	"jobQueue":     models.CapViewSystem,
	"findJob":      models.CapViewSystem,
	"systemStatus": models.CapViewSystem,
	"dlnaStatus":   models.CapViewSystem,

	// the host filesystem. `directory` walks arbitrary paths, and the file and
	// folder finders return real paths — the most sensitive reads in the schema.
	"directory":             models.CapBrowseFilesystem,
	"findFile":              models.CapBrowseFilesystem,
	"findFiles":             models.CapBrowseFilesystem,
	"findFolder":            models.CapBrowseFilesystem,
	"findFolders":           models.CapBrowseFilesystem,
	"findScenesByPathRegex": models.CapBrowseFilesystem,
	"parseSceneFilenames":   models.CapBrowseFilesystem,

	// plugin and package management.
	//
	// NOTE: `plugins` is deliberately absent. PluginsLoader (App.tsx, via
	// src/plugins.tsx) calls it on every page load for every account to fetch
	// the plugin JS/CSS list — restricting it breaks the app for everyone
	// without CapManagePlugins. Only the Settings-only surfaces are listed.
	"pluginTasks":       models.CapManagePlugins,
	"installedPackages": models.CapManagePlugins,
	"availablePackages": models.CapManagePlugins,

	// scraper configuration includes third-party endpoints and credentials
	"listScrapers":                models.CapConfigure,
	"validateStashBoxCredentials": models.CapConfigure,

	// other people's accounts
	"findUsers": models.CapManageUsers,
	"findUser":  models.CapManageUsers,

	"findShareLinks": models.CapManageShares,
	"findShareLink":  models.CapManageShares,

	// content-restriction groups are part of account administration
	"findUserGroups": models.CapManageUsers,
	"findUserGroup":  models.CapManageUsers,
}

// requiredCapabilityForQuery returns the capability a query requires. Reads
// default to CapViewLibrary, so browsing works for every role; content
// *scoping* — which scenes a given user may see — is a separate concern and is
// not expressed here.
func requiredCapabilityForQuery(field string) models.Capability {
	if c, ok := queryCapabilities[field]; ok {
		return c
	}
	// Scrapers fetch on the server's behalf — scrapeURL will retrieve an
	// arbitrary URL — and scraping is part of editing metadata.
	if strings.HasPrefix(field, "scrape") {
		return models.CapScrape
	}
	return models.CapViewLibrary
}

// exemptQueries bypass the capability check entirely. `me` must answer for
// anyone, including an unauthenticated caller (it returns null), because the UI
// uses it to discover whether a session exists at all.
var exemptQueries = map[string]bool{
	"me": true,
}

// permissionMiddleware enforces capability requirements on root Query and
// Mutation fields. Nested resolvers pass through untouched — a field reached
// from an already-authorised root is not re-checked.
func (r *Resolver) permissionMiddleware() graphql.FieldMiddleware {
	return func(ctx context.Context, next graphql.Resolver) (interface{}, error) {
		fc := graphql.GetFieldContext(ctx)
		if fc == nil {
			return next(ctx)
		}

		var required models.Capability
		switch fc.Object {
		case "Mutation":
			required = requiredCapabilityForMutation(fc.Field.Name)
		case "Query":
			if exemptQueries[fc.Field.Name] {
				return next(ctx)
			}
			required = requiredCapabilityForQuery(fc.Field.Name)
		default:
			return next(ctx)
		}

		if err := r.requireCap(ctx, required); err != nil {
			return nil, err
		}
		return next(ctx)
	}
}

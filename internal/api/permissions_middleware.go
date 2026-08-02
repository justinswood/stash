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
	"configureGeneral":       true,
	"configureInterface":     true,
	"configureDLNA":          true,
	"configureScraping":      true,
	"configureDefaults":      true,
	"configurePlugin":        true,
	"configureUI":            true,
	"configureUISetting":     true,
	"generateAPIKey":         true,
	"metadataScan":           true,
	"metadataGenerate":       true,
	"metadataAutoTag":        true,
	"metadataClean":          true,
	"metadataCleanGenerated": true,
	"metadataIdentify":       true,
	"migrate":                true,
	"migrateBlobs":           true,
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

// mutationPermissionMiddleware enforces role requirements on root Mutation
// fields. Query fields and nested resolvers pass through untouched.
func (r *Resolver) mutationPermissionMiddleware() graphql.FieldMiddleware {
	return func(ctx context.Context, next graphql.Resolver) (interface{}, error) {
		fc := graphql.GetFieldContext(ctx)
		if fc == nil || fc.Object != "Mutation" {
			return next(ctx)
		}

		required := requiredRoleForMutation(fc.Field.Name)
		if err := r.requireRole(ctx, required); err != nil {
			return nil, err
		}
		return next(ctx)
	}
}

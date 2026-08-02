package sqlite

import (
	"context"

	"github.com/doug-martin/goqu/v9"
	"github.com/doug-martin/goqu/v9/exp"
	"github.com/jmoiron/sqlx"

	"github.com/stashapp/stash/pkg/models"
)

const userCapabilitiesTable = "user_capabilities"

var userCapabilitiesTableMgr = &table{
	table: goqu.T(userCapabilitiesTable),
}

type userCapabilityRow struct {
	UserID     int    `db:"user_id"`
	Capability string `db:"capability"`
	Granted    bool   `db:"granted"`
}

type UserCapabilityStore struct {
	repository
	tableMgr *table
}

func NewUserCapabilityStore() *UserCapabilityStore {
	return &UserCapabilityStore{
		repository: repository{
			tableName: userCapabilitiesTable,
		},
		tableMgr: userCapabilitiesTableMgr,
	}
}

func (qb *UserCapabilityStore) table() exp.IdentifierExpression {
	return qb.tableMgr.table
}

func (qb *UserCapabilityStore) FindByUserID(ctx context.Context, userID int) ([]models.UserCapabilityOverride, error) {
	q := dialect.From(qb.table()).Prepared(true).
		Select(qb.table().All()).
		Where(qb.table().Col("user_id").Eq(userID))

	const single = false
	var ret []models.UserCapabilityOverride
	if err := queryFunc(ctx, q, single, func(r *sqlx.Rows) error {
		var row userCapabilityRow
		if err := r.StructScan(&row); err != nil {
			return err
		}
		ret = append(ret, models.UserCapabilityOverride{
			UserID:     row.UserID,
			Capability: models.Capability(row.Capability),
			Granted:    row.Granted,
		})
		return nil
	}); err != nil {
		return nil, err
	}
	return ret, nil
}

// SetForUser replaces a user's entire override set. Delete-then-insert rather
// than upsert: the caller always supplies the complete desired state, so any
// override missing from the input is meant to be removed.
func (qb *UserCapabilityStore) SetForUser(ctx context.Context, userID int, overrides []models.UserCapabilityOverride) error {
	del := dialect.Delete(qb.table()).Prepared(true).
		Where(qb.table().Col("user_id").Eq(userID))
	if _, err := exec(ctx, del); err != nil {
		return err
	}

	if len(overrides) == 0 {
		return nil
	}

	rows := make([]interface{}, 0, len(overrides))
	for _, o := range overrides {
		rows = append(rows, userCapabilityRow{
			UserID:     userID,
			Capability: string(o.Capability),
			Granted:    o.Granted,
		})
	}

	ins := dialect.Insert(qb.table()).Prepared(true).Rows(rows...)
	_, err := exec(ctx, ins)
	return err
}

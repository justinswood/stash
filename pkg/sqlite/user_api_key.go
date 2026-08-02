package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/doug-martin/goqu/v9"
	"github.com/doug-martin/goqu/v9/exp"
	"github.com/jmoiron/sqlx"

	"github.com/stashapp/stash/pkg/models"
)

const userAPIKeysTable = "user_api_keys"

var userAPIKeysTableMgr = &table{
	table:    goqu.T(userAPIKeysTable),
	idColumn: goqu.T(userAPIKeysTable).Col(idColumn),
}

type userAPIKeyRow struct {
	ID         int          `db:"id" goqu:"skipinsert"`
	UserID     int          `db:"user_id"`
	Name       string       `db:"name"`
	KeyHash    []byte       `db:"key_hash"`
	Prefix     string       `db:"prefix"`
	LastUsedAt sql.NullTime `db:"last_used_at"`
	CreatedAt  time.Time    `db:"created_at"`
	UpdatedAt  time.Time    `db:"updated_at"`
}

func (r *userAPIKeyRow) fromUserAPIKey(o models.UserAPIKey) {
	r.ID = o.ID
	r.UserID = o.UserID
	r.Name = o.Name
	r.KeyHash = o.KeyHash
	r.Prefix = o.Prefix
	r.LastUsedAt = sql.NullTime{}
	if o.LastUsedAt != nil {
		r.LastUsedAt = sql.NullTime{Time: *o.LastUsedAt, Valid: true}
	}
	r.CreatedAt = o.CreatedAt
	r.UpdatedAt = o.UpdatedAt
}

func (r *userAPIKeyRow) resolve() *models.UserAPIKey {
	ret := &models.UserAPIKey{
		ID:        r.ID,
		UserID:    r.UserID,
		Name:      r.Name,
		KeyHash:   r.KeyHash,
		Prefix:    r.Prefix,
		CreatedAt: r.CreatedAt,
		UpdatedAt: r.UpdatedAt,
	}
	if r.LastUsedAt.Valid {
		t := r.LastUsedAt.Time
		ret.LastUsedAt = &t
	}
	return ret
}

type UserAPIKeyStore struct {
	repository
	tableMgr *table
}

func NewUserAPIKeyStore() *UserAPIKeyStore {
	return &UserAPIKeyStore{
		repository: repository{
			tableName: userAPIKeysTable,
			idColumn:  idColumn,
		},
		tableMgr: userAPIKeysTableMgr,
	}
}

func (qb *UserAPIKeyStore) table() exp.IdentifierExpression {
	return qb.tableMgr.table
}

func (qb *UserAPIKeyStore) selectDataset() *goqu.SelectDataset {
	return dialect.From(qb.table()).Select(qb.table().All())
}

func (qb *UserAPIKeyStore) Create(ctx context.Context, newObject *models.UserAPIKey) error {
	now := time.Now()
	if newObject.CreatedAt.IsZero() {
		newObject.CreatedAt = now
	}
	newObject.UpdatedAt = now

	var r userAPIKeyRow
	r.fromUserAPIKey(*newObject)

	id, err := qb.tableMgr.insertID(ctx, r)
	if err != nil {
		return err
	}

	updated, err := qb.Find(ctx, id)
	if err != nil {
		return fmt.Errorf("finding after create: %w", err)
	}
	if updated == nil {
		return fmt.Errorf("finding after create: key %d not found", id)
	}
	*newObject = *updated
	return nil
}

func (qb *UserAPIKeyStore) Destroy(ctx context.Context, id int) error {
	return qb.destroyExisting(ctx, []int{id})
}

func (qb *UserAPIKeyStore) Find(ctx context.Context, id int) (*models.UserAPIKey, error) {
	q := qb.selectDataset().Where(qb.tableMgr.byID(id))
	ret, err := qb.get(ctx, q)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return ret, err
}

// FindByKeyHash looks a key up by the SHA-256 of its plaintext. This is the
// authentication hot path — it runs on every API-key request — and is served by
// index_user_api_keys_key_hash.
func (qb *UserAPIKeyStore) FindByKeyHash(ctx context.Context, keyHash []byte) (*models.UserAPIKey, error) {
	q := qb.selectDataset().Prepared(true).Where(qb.table().Col("key_hash").Eq(keyHash))
	ret, err := qb.get(ctx, q)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return ret, err
}

func (qb *UserAPIKeyStore) FindByUserID(ctx context.Context, userID int) ([]*models.UserAPIKey, error) {
	q := qb.selectDataset().Prepared(true).
		Where(qb.table().Col("user_id").Eq(userID)).
		Order(qb.table().Col("created_at").Asc())
	return qb.getMany(ctx, q)
}

func (qb *UserAPIKeyStore) TouchLastUsed(ctx context.Context, id int, when time.Time) error {
	q := dialect.Update(qb.table()).Prepared(true).
		Set(goqu.Record{"last_used_at": when}).
		Where(qb.tableMgr.byID(id))
	_, err := exec(ctx, q)
	return err
}

func (qb *UserAPIKeyStore) get(ctx context.Context, q *goqu.SelectDataset) (*models.UserAPIKey, error) {
	ret, err := qb.getMany(ctx, q)
	if err != nil {
		return nil, err
	}
	if len(ret) == 0 {
		return nil, sql.ErrNoRows
	}
	return ret[0], nil
}

func (qb *UserAPIKeyStore) getMany(ctx context.Context, q *goqu.SelectDataset) ([]*models.UserAPIKey, error) {
	const single = false
	var ret []*models.UserAPIKey
	if err := queryFunc(ctx, q, single, func(r *sqlx.Rows) error {
		var row userAPIKeyRow
		if err := r.StructScan(&row); err != nil {
			return err
		}
		ret = append(ret, row.resolve())
		return nil
	}); err != nil {
		return nil, err
	}
	return ret, nil
}

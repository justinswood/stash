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

const usersTable = "users"

var usersTableMgr = &table{
	table:    goqu.T(usersTable),
	idColumn: goqu.T(usersTable).Col(idColumn),
}

type userRow struct {
	ID           int             `db:"id" goqu:"skipinsert"`
	Username     string          `db:"username"`
	PasswordHash string          `db:"password_hash"`
	Role         models.UserRole `db:"role"`
	Disabled     bool            `db:"disabled"`
	CreatedAt    time.Time       `db:"created_at"`
	UpdatedAt    time.Time       `db:"updated_at"`
}

func (r *userRow) fromUser(o models.User) {
	r.ID = o.ID
	r.Username = o.Username
	r.PasswordHash = o.PasswordHash
	r.Role = o.Role
	r.Disabled = o.Disabled
	r.CreatedAt = o.CreatedAt
	r.UpdatedAt = o.UpdatedAt
}

func (r *userRow) resolve() *models.User {
	return &models.User{
		ID:           r.ID,
		Username:     r.Username,
		PasswordHash: r.PasswordHash,
		Role:         r.Role,
		Disabled:     r.Disabled,
		CreatedAt:    r.CreatedAt,
		UpdatedAt:    r.UpdatedAt,
	}
}

type UserStore struct {
	repository
	tableMgr *table
}

func NewUserStore() *UserStore {
	return &UserStore{
		repository: repository{
			tableName: usersTable,
			idColumn:  idColumn,
		},
		tableMgr: usersTableMgr,
	}
}

func (qb *UserStore) table() exp.IdentifierExpression {
	return qb.tableMgr.table
}

func (qb *UserStore) selectDataset() *goqu.SelectDataset {
	return dialect.From(qb.table()).Select(qb.table().All())
}

func (qb *UserStore) Create(ctx context.Context, newObject *models.User) error {
	now := time.Now()
	if newObject.CreatedAt.IsZero() {
		newObject.CreatedAt = now
	}
	newObject.UpdatedAt = now
	if newObject.Role == "" {
		newObject.Role = models.UserRoleUser
	}

	var r userRow
	r.fromUser(*newObject)

	id, err := qb.tableMgr.insertID(ctx, r)
	if err != nil {
		return err
	}

	updated, err := qb.find(ctx, int(id))
	if err != nil {
		return fmt.Errorf("finding after create: %w", err)
	}
	*newObject = *updated
	return nil
}

func (qb *UserStore) Update(ctx context.Context, updatedObject *models.User) error {
	updatedObject.UpdatedAt = time.Now()
	var r userRow
	r.fromUser(*updatedObject)

	if err := qb.tableMgr.updateByID(ctx, updatedObject.ID, r); err != nil {
		return err
	}
	return nil
}

func (qb *UserStore) Destroy(ctx context.Context, id int) error {
	return qb.destroyExisting(ctx, []int{id})
}

func (qb *UserStore) Find(ctx context.Context, id int) (*models.User, error) {
	ret, err := qb.find(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return ret, err
}

func (qb *UserStore) find(ctx context.Context, id int) (*models.User, error) {
	q := qb.selectDataset().Where(qb.tableMgr.byID(id))
	return qb.get(ctx, q)
}

func (qb *UserStore) FindByUsername(ctx context.Context, username string) (*models.User, error) {
	q := qb.selectDataset().Prepared(true).Where(qb.table().Col("username").Eq(username))
	ret, err := qb.get(ctx, q)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return ret, err
}

func (qb *UserStore) All(ctx context.Context) ([]*models.User, error) {
	q := qb.selectDataset().Order(qb.table().Col("username").Asc())
	return qb.getMany(ctx, q)
}

func (qb *UserStore) Count(ctx context.Context) (int, error) {
	q := dialect.From(qb.table()).Select(goqu.COUNT("*"))
	var count int
	if err := querySimple(ctx, q, &count); err != nil {
		return 0, err
	}
	return count, nil
}

func (qb *UserStore) get(ctx context.Context, q *goqu.SelectDataset) (*models.User, error) {
	ret, err := qb.getMany(ctx, q)
	if err != nil {
		return nil, err
	}
	if len(ret) == 0 {
		return nil, sql.ErrNoRows
	}
	return ret[0], nil
}

func (qb *UserStore) getMany(ctx context.Context, q *goqu.SelectDataset) ([]*models.User, error) {
	const single = false
	var ret []*models.User
	if err := queryFunc(ctx, q, single, func(r *sqlx.Rows) error {
		var row userRow
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

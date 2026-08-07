package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"slices"

	"github.com/doug-martin/goqu/v9"
	"github.com/doug-martin/goqu/v9/exp"
	"github.com/jmoiron/sqlx"
	"gopkg.in/guregu/null.v4"

	"github.com/stashapp/stash/pkg/logger"
	"github.com/stashapp/stash/pkg/models"
)

const (
	savedFilterTable       = "saved_filters"
	savedFilterDefaultName = ""
)

type savedFilterRow struct {
	ID           int               `db:"id" goqu:"skipinsert"`
	UserID       null.Int          `db:"user_id"`
	Mode         models.FilterMode `db:"mode"`
	Name         string            `db:"name"`
	FindFilter   string            `db:"find_filter"`
	ObjectFilter string            `db:"object_filter"`
	UIOptions    string            `db:"ui_options"`
}

func encodeJSONOrEmpty(v interface{}) string {
	if v == nil {
		return ""
	}

	encoded, err := json.Marshal(v)
	if err != nil {
		logger.Errorf("error encoding json %v: %v", v, err)
	}

	return string(encoded)
}

func decodeJSON(s string, v interface{}) {
	if s == "" {
		return
	}

	if err := json.Unmarshal([]byte(s), v); err != nil {
		logger.Errorf("error decoding json %q: %v", s, err)
	}
}

func (r *savedFilterRow) fromSavedFilter(o models.SavedFilter) {
	r.ID = o.ID
	r.Mode = o.Mode
	r.Name = o.Name

	// encode the filters as json
	r.FindFilter = encodeJSONOrEmpty(o.FindFilter)
	r.ObjectFilter = encodeJSONOrEmpty(o.ObjectFilter)
	r.UIOptions = encodeJSONOrEmpty(o.UIOptions)
}

func (r *savedFilterRow) resolve() *models.SavedFilter {
	ret := &models.SavedFilter{
		ID:   r.ID,
		Mode: r.Mode,
		Name: r.Name,
	}

	// decode the filters from json
	if r.FindFilter != "" {
		ret.FindFilter = &models.FindFilterType{}
		decodeJSON(r.FindFilter, &ret.FindFilter)
	}
	if r.ObjectFilter != "" {
		ret.ObjectFilter = make(map[string]interface{})
		decodeJSON(r.ObjectFilter, &ret.ObjectFilter)
	}
	if r.UIOptions != "" {
		ret.UIOptions = make(map[string]interface{})
		decodeJSON(r.UIOptions, &ret.UIOptions)
	}

	return ret
}

type SavedFilterStore struct {
	repository
	tableMgr *table
}

func NewSavedFilterStore() *SavedFilterStore {
	return &SavedFilterStore{
		repository: repository{
			tableName: savedFilterTable,
			idColumn:  idColumn,
		},
		tableMgr: savedFilterTableMgr,
	}
}

func (qb *SavedFilterStore) table() exp.IdentifierExpression {
	return qb.tableMgr.table
}

// ownerPredicate scopes saved filters to the requesting account.
//
// Saved filters became per-user in migration 84. Every read path — find,
// FindMany, FindByMode and All — funnels through selectDataset, so applying
// the predicate there is what makes the scoping total. A read path added later
// that builds its own dataset would silently see every account's filters.
//
// A userID of 0 means no user in context. That is not a background task here
// (nothing server-side reads saved filters outside a GraphQL request) but an
// instance with authentication disabled, where authenticateHandler lets
// requests through with no account at all. Such filters are stored with a NULL
// owner, so matching NULL keeps that configuration working exactly as it did
// before this change, without exposing one account's filters to another on an
// instance that does have accounts.
func (qb *SavedFilterStore) ownerPredicate(ctx context.Context) exp.Expression {
	userID := historyUserID(ctx)
	if userID <= 0 {
		return qb.table().Col("user_id").IsNull()
	}
	return qb.table().Col("user_id").Eq(userID)
}

// ownerValue is the owner to stamp on a filter this request creates, matching
// what ownerPredicate will look for when reading it back.
func ownerValue(ctx context.Context) null.Int {
	userID := historyUserID(ctx)
	if userID <= 0 {
		return null.NewInt(0, false)
	}
	return null.IntFrom(int64(userID))
}

func (qb *SavedFilterStore) selectDataset(ctx context.Context) *goqu.SelectDataset {
	return dialect.From(qb.table()).Select(qb.table().All()).Where(qb.ownerPredicate(ctx))
}

func (qb *SavedFilterStore) Create(ctx context.Context, newObject *models.SavedFilter) error {
	var r savedFilterRow
	r.fromSavedFilter(*newObject)

	// stamp the owner; fromSavedFilter cannot, because models.SavedFilter has
	// no owner field — a filter is only ever created for the requesting account
	r.UserID = ownerValue(ctx)

	id, err := qb.tableMgr.insertID(ctx, r)
	if err != nil {
		return err
	}

	updated, err := qb.Find(ctx, id)
	if err != nil {
		return fmt.Errorf("finding after create: %w", err)
	}

	*newObject = *updated

	return nil
}

// ownsFilter reports whether the requesting account owns the given filter.
// find is already owner-scoped, so a filter belonging to somebody else is
// indistinguishable from one that does not exist.
func (qb *SavedFilterStore) ownsFilter(ctx context.Context, id int) (bool, error) {
	f, err := qb.Find(ctx, id)
	if err != nil {
		return false, err
	}
	return f != nil, nil
}

func (qb *SavedFilterStore) Update(ctx context.Context, updatedObject *models.SavedFilter) error {
	// Without this check updateByID would happily rewrite another account's
	// filter by id. Reads are scoped by selectDataset, but writes go through
	// tableMgr and never touch that predicate.
	owns, err := qb.ownsFilter(ctx, updatedObject.ID)
	if err != nil {
		return err
	}
	if !owns {
		return fmt.Errorf("filter with id %d not found", updatedObject.ID)
	}

	var r savedFilterRow
	r.fromSavedFilter(*updatedObject)
	r.UserID = ownerValue(ctx)

	if err := qb.tableMgr.updateByID(ctx, updatedObject.ID, r); err != nil {
		return err
	}

	return nil
}

func (qb *SavedFilterStore) Destroy(ctx context.Context, id int) error {
	// Same reasoning as Update: destroyExisting works by id alone, so without
	// this an account could delete filters it cannot even see.
	owns, err := qb.ownsFilter(ctx, id)
	if err != nil {
		return err
	}
	if !owns {
		return fmt.Errorf("filter with id %d not found", id)
	}

	return qb.destroyExisting(ctx, []int{id})
}

// returns nil, nil if not found
func (qb *SavedFilterStore) Find(ctx context.Context, id int) (*models.SavedFilter, error) {
	ret, err := qb.find(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return ret, err
}

func (qb *SavedFilterStore) FindMany(ctx context.Context, ids []int, ignoreNotFound bool) ([]*models.SavedFilter, error) {
	ret := make([]*models.SavedFilter, len(ids))

	table := qb.table()
	q := qb.selectDataset(ctx).Prepared(true).Where(table.Col(idColumn).In(ids))
	unsorted, err := qb.getMany(ctx, q)
	if err != nil {
		return nil, err
	}

	for _, s := range unsorted {
		i := slices.Index(ids, s.ID)
		ret[i] = s
	}

	if !ignoreNotFound {
		for i := range ret {
			if ret[i] == nil {
				return nil, fmt.Errorf("filter with id %d not found", ids[i])
			}
		}
	}

	return ret, nil
}

// returns nil, sql.ErrNoRows if not found
func (qb *SavedFilterStore) find(ctx context.Context, id int) (*models.SavedFilter, error) {
	q := qb.selectDataset(ctx).Where(qb.tableMgr.byID(id))

	ret, err := qb.get(ctx, q)
	if err != nil {
		return nil, err
	}

	return ret, nil
}

func (qb *SavedFilterStore) get(ctx context.Context, q *goqu.SelectDataset) (*models.SavedFilter, error) {
	ret, err := qb.getMany(ctx, q)
	if err != nil {
		return nil, err
	}

	if len(ret) == 0 {
		return nil, sql.ErrNoRows
	}

	return ret[0], nil
}

func (qb *SavedFilterStore) getMany(ctx context.Context, q *goqu.SelectDataset) ([]*models.SavedFilter, error) {
	const single = false
	var ret []*models.SavedFilter
	if err := queryFunc(ctx, q, single, func(r *sqlx.Rows) error {
		var f savedFilterRow
		if err := r.StructScan(&f); err != nil {
			return err
		}

		s := f.resolve()

		ret = append(ret, s)
		return nil
	}); err != nil {
		return nil, err
	}

	return ret, nil
}

func (qb *SavedFilterStore) FindByMode(ctx context.Context, mode models.FilterMode) ([]*models.SavedFilter, error) {
	// SELECT * FROM %s WHERE mode = ? AND name != ? ORDER BY name ASC
	table := qb.table()

	// TODO - querying on groups needs to include movies
	// remove this when we migrate to remove the movies filter mode in the database
	var whereClause exp.Expression

	if mode == models.FilterModeGroups || mode == models.FilterModeMovies {
		whereClause = goqu.Or(
			table.Col("mode").Eq(models.FilterModeGroups),
			table.Col("mode").Eq(models.FilterModeMovies),
		)
	} else {
		whereClause = table.Col("mode").Eq(mode)
	}

	sq := qb.selectDataset(ctx).Prepared(true).Where(whereClause).Order(table.Col("name").Asc())
	ret, err := qb.getMany(ctx, sq)

	if err != nil {
		return nil, err
	}

	return ret, nil
}

func (qb *SavedFilterStore) All(ctx context.Context) ([]*models.SavedFilter, error) {
	return qb.getMany(ctx, qb.selectDataset(ctx))
}

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

const shareLinksTable = "share_links"

var shareLinksTableMgr = &table{
	table:    goqu.T(shareLinksTable),
	idColumn: goqu.T(shareLinksTable).Col(idColumn),
}

type shareLinkRow struct {
	ID          int              `db:"id" goqu:"skipinsert"`
	TokenHash   []byte           `db:"token_hash"`
	ShareType   models.ShareType `db:"share_type"`
	SceneID     sql.NullInt64    `db:"scene_id"`
	ImageID     sql.NullInt64    `db:"image_id"`
	PerformerID sql.NullInt64    `db:"performer_id"`
	ExpiresAt   sql.NullTime     `db:"expires_at"`
	ViewLimit   sql.NullInt64    `db:"view_limit"`
	ViewCount   int              `db:"view_count"`
	Revoked     bool             `db:"revoked"`
	Note        sql.NullString   `db:"note"`
	CreatedAt   time.Time        `db:"created_at"`
	UpdatedAt   time.Time        `db:"updated_at"`
}

func (r *shareLinkRow) fromShareLink(o models.ShareLink) {
	r.ID = o.ID
	r.TokenHash = o.TokenHash
	r.ShareType = o.ShareType
	r.SceneID = nullIntFromIntPtr(o.SceneID)
	r.ImageID = nullIntFromIntPtr(o.ImageID)
	r.PerformerID = nullIntFromIntPtr(o.PerformerID)
	if o.ExpiresAt != nil {
		r.ExpiresAt = sql.NullTime{Time: *o.ExpiresAt, Valid: true}
	}
	r.ViewLimit = nullIntFromIntPtr(o.ViewLimit)
	r.ViewCount = o.ViewCount
	r.Revoked = o.Revoked
	if o.Note != nil {
		r.Note = sql.NullString{String: *o.Note, Valid: true}
	}
	r.CreatedAt = o.CreatedAt
	r.UpdatedAt = o.UpdatedAt
}

func (r *shareLinkRow) resolve() *models.ShareLink {
	ret := &models.ShareLink{
		ID:        r.ID,
		TokenHash: r.TokenHash,
		ShareType: r.ShareType,
		ViewCount: r.ViewCount,
		Revoked:   r.Revoked,
		CreatedAt: r.CreatedAt,
		UpdatedAt: r.UpdatedAt,
	}
	if r.SceneID.Valid {
		v := int(r.SceneID.Int64)
		ret.SceneID = &v
	}
	if r.ImageID.Valid {
		v := int(r.ImageID.Int64)
		ret.ImageID = &v
	}
	if r.PerformerID.Valid {
		v := int(r.PerformerID.Int64)
		ret.PerformerID = &v
	}
	if r.ExpiresAt.Valid {
		t := r.ExpiresAt.Time
		ret.ExpiresAt = &t
	}
	if r.ViewLimit.Valid {
		v := int(r.ViewLimit.Int64)
		ret.ViewLimit = &v
	}
	if r.Note.Valid {
		s := r.Note.String
		ret.Note = &s
	}
	return ret
}

func nullIntFromIntPtr(v *int) sql.NullInt64 {
	if v == nil {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: int64(*v), Valid: true}
}

type ShareLinkStore struct {
	repository
	tableMgr *table
}

func NewShareLinkStore() *ShareLinkStore {
	return &ShareLinkStore{
		repository: repository{
			tableName: shareLinksTable,
			idColumn:  idColumn,
		},
		tableMgr: shareLinksTableMgr,
	}
}

func (qb *ShareLinkStore) table() exp.IdentifierExpression {
	return qb.tableMgr.table
}

func (qb *ShareLinkStore) selectDataset() *goqu.SelectDataset {
	return dialect.From(qb.table()).Select(qb.table().All())
}

func (qb *ShareLinkStore) Create(ctx context.Context, newObject *models.ShareLink) error {
	now := time.Now()
	if newObject.CreatedAt.IsZero() {
		newObject.CreatedAt = now
	}
	newObject.UpdatedAt = now

	var r shareLinkRow
	r.fromShareLink(*newObject)

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

func (qb *ShareLinkStore) Update(ctx context.Context, updatedObject *models.ShareLink) error {
	updatedObject.UpdatedAt = time.Now()
	var r shareLinkRow
	r.fromShareLink(*updatedObject)

	if err := qb.tableMgr.updateByID(ctx, updatedObject.ID, r); err != nil {
		return err
	}
	return nil
}

func (qb *ShareLinkStore) Destroy(ctx context.Context, id int) error {
	return qb.destroyExisting(ctx, []int{id})
}

func (qb *ShareLinkStore) Find(ctx context.Context, id int) (*models.ShareLink, error) {
	ret, err := qb.find(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return ret, err
}

func (qb *ShareLinkStore) find(ctx context.Context, id int) (*models.ShareLink, error) {
	q := qb.selectDataset().Where(qb.tableMgr.byID(id))
	return qb.get(ctx, q)
}

func (qb *ShareLinkStore) FindByTokenHash(ctx context.Context, tokenHash []byte) (*models.ShareLink, error) {
	q := qb.selectDataset().Prepared(true).Where(qb.table().Col("token_hash").Eq(tokenHash))
	ret, err := qb.get(ctx, q)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return ret, err
}

func (qb *ShareLinkStore) All(ctx context.Context) ([]*models.ShareLink, error) {
	q := qb.selectDataset().Order(qb.table().Col("created_at").Desc())
	return qb.getMany(ctx, q)
}

func (qb *ShareLinkStore) IncrementViewCount(ctx context.Context, id int) (int, error) {
	// Atomic check-and-increment: only increment when not exceeding view_limit
	// (or when view_limit is NULL).
	// The transaction wrapper provides the connection; goqu's update is fine here.
	q := dialect.Update(shareLinksTable).
		Prepared(true).
		Set(goqu.Record{
			"view_count": goqu.L("view_count + 1"),
			"updated_at": time.Now(),
		}).
		Where(
			goqu.C("id").Eq(id),
			goqu.C("revoked").Eq(false),
			goqu.Or(
				goqu.C("view_limit").IsNull(),
				goqu.L("view_count < view_limit"),
			),
		)

	res, err := exec(ctx, q)
	if err != nil {
		return 0, err
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		// Either the row doesn't exist, is revoked, or has hit its view limit.
		return 0, fmt.Errorf("share link %d cannot increment (revoked or limit reached)", id)
	}

	// Re-read to get the new count
	link, err := qb.find(ctx, id)
	if err != nil {
		return 0, err
	}
	return link.ViewCount, nil
}

func (qb *ShareLinkStore) get(ctx context.Context, q *goqu.SelectDataset) (*models.ShareLink, error) {
	ret, err := qb.getMany(ctx, q)
	if err != nil {
		return nil, err
	}
	if len(ret) == 0 {
		return nil, sql.ErrNoRows
	}
	return ret[0], nil
}

func (qb *ShareLinkStore) getMany(ctx context.Context, q *goqu.SelectDataset) ([]*models.ShareLink, error) {
	const single = false
	var ret []*models.ShareLink
	if err := queryFunc(ctx, q, single, func(r *sqlx.Rows) error {
		var row shareLinkRow
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

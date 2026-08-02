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

const (
	userGroupsTable       = "user_groups"
	userGroupMembersTable = "user_group_members"
	userGroupGendersTable = "user_group_excluded_genders"
	userGroupTagsTable    = "user_group_excluded_tags"
)

var userGroupsTableMgr = &table{
	table:    goqu.T(userGroupsTable),
	idColumn: goqu.T(userGroupsTable).Col(idColumn),
}

type userGroupRow struct {
	ID          int            `db:"id" goqu:"skipinsert"`
	Name        string         `db:"name"`
	Description sql.NullString `db:"description"`
	CreatedAt   time.Time      `db:"created_at"`
	UpdatedAt   time.Time      `db:"updated_at"`
}

func (r *userGroupRow) fromUserGroup(o models.UserGroup) {
	r.ID = o.ID
	r.Name = o.Name
	r.Description = sql.NullString{String: o.Description, Valid: o.Description != ""}
	r.CreatedAt = o.CreatedAt
	r.UpdatedAt = o.UpdatedAt
}

func (r *userGroupRow) resolve() *models.UserGroup {
	return &models.UserGroup{
		ID:          r.ID,
		Name:        r.Name,
		Description: r.Description.String,
		CreatedAt:   r.CreatedAt,
		UpdatedAt:   r.UpdatedAt,
	}
}

type UserGroupStore struct {
	repository
	tableMgr *table
}

func NewUserGroupStore() *UserGroupStore {
	return &UserGroupStore{
		repository: repository{
			tableName: userGroupsTable,
			idColumn:  idColumn,
		},
		tableMgr: userGroupsTableMgr,
	}
}

func (qb *UserGroupStore) table() exp.IdentifierExpression {
	return qb.tableMgr.table
}

func (qb *UserGroupStore) selectDataset() *goqu.SelectDataset {
	return dialect.From(qb.table()).Select(qb.table().All())
}

func (qb *UserGroupStore) Create(ctx context.Context, newObject *models.UserGroup) error {
	now := time.Now()
	if newObject.CreatedAt.IsZero() {
		newObject.CreatedAt = now
	}
	newObject.UpdatedAt = now

	var r userGroupRow
	r.fromUserGroup(*newObject)

	id, err := qb.tableMgr.insertID(ctx, r)
	if err != nil {
		return err
	}
	updated, err := qb.Find(ctx, id)
	if err != nil {
		return fmt.Errorf("finding after create: %w", err)
	}
	if updated == nil {
		return fmt.Errorf("finding after create: group %d not found", id)
	}
	*newObject = *updated
	return nil
}

func (qb *UserGroupStore) Update(ctx context.Context, updatedObject *models.UserGroup) error {
	updatedObject.UpdatedAt = time.Now()
	var r userGroupRow
	r.fromUserGroup(*updatedObject)
	return qb.tableMgr.updateByID(ctx, updatedObject.ID, r)
}

func (qb *UserGroupStore) Destroy(ctx context.Context, id int) error {
	return qb.destroyExisting(ctx, []int{id})
}

func (qb *UserGroupStore) Find(ctx context.Context, id int) (*models.UserGroup, error) {
	q := qb.selectDataset().Where(qb.tableMgr.byID(id))
	ret, err := qb.get(ctx, q)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return ret, err
}

func (qb *UserGroupStore) FindByName(ctx context.Context, name string) (*models.UserGroup, error) {
	q := qb.selectDataset().Prepared(true).Where(qb.table().Col("name").Eq(name))
	ret, err := qb.get(ctx, q)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return ret, err
}

func (qb *UserGroupStore) All(ctx context.Context) ([]*models.UserGroup, error) {
	return qb.getMany(ctx, qb.selectDataset().Order(qb.table().Col("name").Asc()))
}

func (qb *UserGroupStore) get(ctx context.Context, q *goqu.SelectDataset) (*models.UserGroup, error) {
	ret, err := qb.getMany(ctx, q)
	if err != nil {
		return nil, err
	}
	if len(ret) == 0 {
		return nil, sql.ErrNoRows
	}
	return ret[0], nil
}

func (qb *UserGroupStore) getMany(ctx context.Context, q *goqu.SelectDataset) ([]*models.UserGroup, error) {
	const single = false
	var ret []*models.UserGroup
	if err := queryFunc(ctx, q, single, func(r *sqlx.Rows) error {
		var row userGroupRow
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

// --- membership ---

func (qb *UserGroupStore) MemberIDs(ctx context.Context, groupID int) ([]int, error) {
	return qb.intColumn(ctx, userGroupMembersTable, "user_id", "group_id", groupID)
}

func (qb *UserGroupStore) GroupIDsForUser(ctx context.Context, userID int) ([]int, error) {
	return qb.intColumn(ctx, userGroupMembersTable, "group_id", "user_id", userID)
}

func (qb *UserGroupStore) SetMembers(ctx context.Context, groupID int, userIDs []int) error {
	t := goqu.T(userGroupMembersTable)
	del := dialect.Delete(t).Prepared(true).Where(t.Col("group_id").Eq(groupID))
	if _, err := exec(ctx, del); err != nil {
		return err
	}
	if len(userIDs) == 0 {
		return nil
	}
	rows := make([]interface{}, 0, len(userIDs))
	for _, uid := range userIDs {
		rows = append(rows, goqu.Record{"user_id": uid, "group_id": groupID})
	}
	_, err := exec(ctx, dialect.Insert(t).Prepared(true).Rows(rows...))
	return err
}

// --- exclusions ---

func (qb *UserGroupStore) ExcludedGenders(ctx context.Context, groupID int) ([]string, error) {
	t := goqu.T(userGroupGendersTable)
	q := dialect.From(t).Prepared(true).Select(t.Col("gender")).
		Where(t.Col("group_id").Eq(groupID)).Order(t.Col("gender").Asc())

	const single = false
	var ret []string
	if err := queryFunc(ctx, q, single, func(r *sqlx.Rows) error {
		var v string
		if err := r.Scan(&v); err != nil {
			return err
		}
		ret = append(ret, v)
		return nil
	}); err != nil {
		return nil, err
	}
	return ret, nil
}

func (qb *UserGroupStore) SetExcludedGenders(ctx context.Context, groupID int, genders []string) error {
	t := goqu.T(userGroupGendersTable)
	del := dialect.Delete(t).Prepared(true).Where(t.Col("group_id").Eq(groupID))
	if _, err := exec(ctx, del); err != nil {
		return err
	}
	if len(genders) == 0 {
		return nil
	}
	rows := make([]interface{}, 0, len(genders))
	for _, g := range genders {
		rows = append(rows, goqu.Record{"group_id": groupID, "gender": g})
	}
	_, err := exec(ctx, dialect.Insert(t).Prepared(true).Rows(rows...))
	return err
}

func (qb *UserGroupStore) ExcludedTagIDs(ctx context.Context, groupID int) ([]int, error) {
	return qb.intColumn(ctx, userGroupTagsTable, "tag_id", "group_id", groupID)
}

func (qb *UserGroupStore) SetExcludedTagIDs(ctx context.Context, groupID int, tagIDs []int) error {
	t := goqu.T(userGroupTagsTable)
	del := dialect.Delete(t).Prepared(true).Where(t.Col("group_id").Eq(groupID))
	if _, err := exec(ctx, del); err != nil {
		return err
	}
	if len(tagIDs) == 0 {
		return nil
	}
	rows := make([]interface{}, 0, len(tagIDs))
	for _, id := range tagIDs {
		rows = append(rows, goqu.Record{"group_id": groupID, "tag_id": id})
	}
	_, err := exec(ctx, dialect.Insert(t).Prepared(true).Rows(rows...))
	return err
}

// RestrictionsForUser unions the exclusions of every group the user belongs to.
// Union, not intersection: belonging to an additional group must never reveal
// something another group hid.
func (qb *UserGroupStore) RestrictionsForUser(ctx context.Context, userID int) (models.ContentRestrictions, error) {
	var ret models.ContentRestrictions

	groupIDs, err := qb.GroupIDsForUser(ctx, userID)
	if err != nil || len(groupIDs) == 0 {
		return ret, err
	}

	genderSet := make(map[string]struct{})
	tagSet := make(map[int]struct{})
	for _, gid := range groupIDs {
		genders, err := qb.ExcludedGenders(ctx, gid)
		if err != nil {
			return ret, err
		}
		for _, g := range genders {
			genderSet[g] = struct{}{}
		}
		tags, err := qb.ExcludedTagIDs(ctx, gid)
		if err != nil {
			return ret, err
		}
		for _, t := range tags {
			tagSet[t] = struct{}{}
		}
	}

	for g := range genderSet {
		ret.ExcludedGenders = append(ret.ExcludedGenders, g)
	}
	for t := range tagSet {
		ret.ExcludedTagIDs = append(ret.ExcludedTagIDs, t)
	}
	return ret, nil
}

// intColumn reads one integer column from a join table filtered by another.
func (qb *UserGroupStore) intColumn(ctx context.Context, table, selectCol, whereCol string, whereVal int) ([]int, error) {
	t := goqu.T(table)
	q := dialect.From(t).Prepared(true).Select(t.Col(selectCol)).
		Where(t.Col(whereCol).Eq(whereVal)).Order(t.Col(selectCol).Asc())

	const single = false
	var ret []int
	if err := queryFunc(ctx, q, single, func(r *sqlx.Rows) error {
		var v int
		if err := r.Scan(&v); err != nil {
			return err
		}
		ret = append(ret, v)
		return nil
	}); err != nil {
		return nil, err
	}
	return ret, nil
}

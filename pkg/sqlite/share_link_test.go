//go:build integration
// +build integration

package sqlite_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/stashapp/stash/pkg/models"
)

// A share link is served to anyone holding the token, with no login. The view
// limit is therefore a real access control, not a counter: once it is reached
// the content must stop being reachable. IncrementViewCount enforces that with
// a single conditional UPDATE rather than a read-then-write, so that two
// simultaneous requests on a one-view link cannot both pass.

func makeShareLink(ctx context.Context, t *testing.T, viewLimit *int, expiresAt *time.Time, revoked bool) *models.ShareLink {
	t.Helper()

	sceneID := sceneIDs[sceneIdxWithPerformer]
	link := models.ShareLink{
		TokenHash: []byte("test-token-hash-" + t.Name()),
		ShareType: models.ShareTypeScene,
		SceneID:   &sceneID,
		ViewLimit: viewLimit,
		ExpiresAt: expiresAt,
		Revoked:   revoked,
	}
	require.NoError(t, db.ShareLink.Create(ctx, &link), "creating share link")
	return &link
}

func intPtr(i int) *int { return &i }

func TestShareLinkViewLimitStopsAtLimit(t *testing.T) {
	runWithRollbackTxn(t, "TestShareLinkViewLimitStopsAtLimit", func(t *testing.T, ctx context.Context) {
		link := makeShareLink(ctx, t, intPtr(3), nil, false)

		for i := 1; i <= 3; i++ {
			count, err := db.ShareLink.IncrementViewCount(ctx, link.ID)
			assert.NoError(t, err, "view %d of 3 must be allowed", i)
			assert.Equal(t, i, count, "view count after view %d", i)
		}

		// The fourth view is the one the limit exists to stop. An off-by-one
		// here hands out one more view of the content than the owner allowed.
		_, err := db.ShareLink.IncrementViewCount(ctx, link.ID)
		assert.Error(t, err, "the view after the limit must be refused — "+
			"the limit is an access control, not a counter")
	})
}

func TestShareLinkNilViewLimitIsUnlimited(t *testing.T) {
	runWithRollbackTxn(t, "TestShareLinkNilViewLimitIsUnlimited", func(t *testing.T, ctx context.Context) {
		link := makeShareLink(ctx, t, nil, nil, false)

		// A link created without a limit must not be treated as limit zero,
		// which would make every unlimited link dead on arrival.
		for i := 1; i <= 5; i++ {
			count, err := db.ShareLink.IncrementViewCount(ctx, link.ID)
			assert.NoError(t, err, "unlimited link must allow view %d", i)
			assert.Equal(t, i, count)
		}
	})
}

func TestShareLinkRevokedCannotIncrement(t *testing.T) {
	runWithRollbackTxn(t, "TestShareLinkRevokedCannotIncrement", func(t *testing.T, ctx context.Context) {
		link := makeShareLink(ctx, t, nil, nil, true)

		// Revocation is the owner's emergency stop. It must hold even on a link
		// with no view limit, which is the case the WHERE clause could most
		// easily get wrong.
		_, err := db.ShareLink.IncrementViewCount(ctx, link.ID)
		assert.Error(t, err, "a revoked link must not serve another view")
	})
}

func TestShareLinkIncrementUnknownIDFails(t *testing.T) {
	runWithRollbackTxn(t, "TestShareLinkIncrementUnknownIDFails", func(t *testing.T, ctx context.Context) {
		_, err := db.ShareLink.IncrementViewCount(ctx, 0)
		assert.Error(t, err, "incrementing a nonexistent link must fail rather than report success")
	})
}

func TestShareLinkFindByTokenHash(t *testing.T) {
	runWithRollbackTxn(t, "TestShareLinkFindByTokenHash", func(t *testing.T, ctx context.Context) {
		link := makeShareLink(ctx, t, nil, nil, false)

		found, err := db.ShareLink.FindByTokenHash(ctx, link.TokenHash)
		require.NoError(t, err)
		require.NotNil(t, found, "a live link must be findable by its token hash")
		assert.Equal(t, link.ID, found.ID)

		// An unknown hash must return nil rather than an arbitrary row. If this
		// ever matched loosely, any token would open somebody's share.
		found, err = db.ShareLink.FindByTokenHash(ctx, []byte("no-such-token-hash"))
		assert.NoError(t, err)
		assert.Nil(t, found, "an unknown token hash must match nothing")
	})
}

// TestShareLinkConcurrentViewersGetOneView pins the end-to-end property: many
// simultaneous viewers of a one-view link, exactly one served.
//
// Be clear about what this does and does not prove. It does NOT demonstrate
// that the conditional UPDATE in IncrementViewCount is what prevents the race —
// swapping that implementation for a naive read-then-write leaves this test
// passing, which was measured, not assumed. The reason is that Stash opens
// exactly one write connection (maxWriteConnections = 1 in database.go, behind
// db.lockChan), so write transactions are serialised and cannot interleave.
// The conditional UPDATE is defence in depth, not the load-bearing mechanism.
//
// It is still worth keeping: it locks in the observable behaviour, and it is
// precisely the test that would begin to fail if the write pool were ever
// widened or the locking relaxed — the change most likely to reintroduce the
// race, and the one least likely to prompt anyone to re-examine share links.
//
// Runs outside the rollback harness because the writers need separate
// transactions; it cleans up after itself.
func TestShareLinkConcurrentViewersGetOneView(t *testing.T) {
	var linkID int

	require.NoError(t, withTxn(func(ctx context.Context) error {
		sceneID := sceneIDs[sceneIdxWithPerformer]
		link := models.ShareLink{
			TokenHash: []byte("test-token-hash-atomic"),
			ShareType: models.ShareTypeScene,
			SceneID:   &sceneID,
			ViewLimit: intPtr(1),
		}
		if err := db.ShareLink.Create(ctx, &link); err != nil {
			return err
		}
		linkID = link.ID
		return nil
	}), "creating the contended link")

	t.Cleanup(func() {
		_ = withTxn(func(ctx context.Context) error {
			return db.ShareLink.Destroy(ctx, linkID)
		})
	})

	const viewers = 8

	var (
		wg        sync.WaitGroup
		mu        sync.Mutex
		succeeded int
	)

	start := make(chan struct{})
	for i := 0; i < viewers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start

			err := withTxn(func(ctx context.Context) error {
				_, err := db.ShareLink.IncrementViewCount(ctx, linkID)
				return err
			})
			if err == nil {
				mu.Lock()
				succeeded++
				mu.Unlock()
			}
		}()
	}

	close(start)
	wg.Wait()

	assert.Equal(t, 1, succeeded,
		"exactly one of %d concurrent viewers may pass a one-view link; "+
			"more than one means the view limit is no longer an access control", viewers)

	require.NoError(t, withTxn(func(ctx context.Context) error {
		link, err := db.ShareLink.Find(ctx, linkID)
		if err != nil {
			return err
		}
		assert.Equal(t, 1, link.ViewCount, "the stored count must match the number of views served")
		return nil
	}))
}

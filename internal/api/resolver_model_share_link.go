package api

import (
	"context"
	"sync"

	"github.com/stashapp/stash/pkg/models"
)

// shareLinkPlaintextStore holds the plaintext token for a freshly-created
// ShareLink so the GraphQL field resolvers for `token` and `url` can return
// it once. After the request completes, the context is discarded.
type shareLinkPlaintextStore struct {
	mu     sync.Mutex
	tokens map[int]string
}

func (s *shareLinkPlaintextStore) set(id int, token string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.tokens == nil {
		s.tokens = make(map[int]string)
	}
	s.tokens[id] = token
}

func (s *shareLinkPlaintextStore) get(id int) (string, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, ok := s.tokens[id]
	return v, ok
}

type shareLinkPlaintextCtxKey struct{}

func withShareLinkPlaintextStore(ctx context.Context) context.Context {
	return context.WithValue(ctx, shareLinkPlaintextCtxKey{}, &shareLinkPlaintextStore{})
}

func stashShareLinkPlaintext(ctx context.Context, id int, token string) {
	if s, ok := ctx.Value(shareLinkPlaintextCtxKey{}).(*shareLinkPlaintextStore); ok {
		s.set(id, token)
	}
}

func getShareLinkPlaintext(ctx context.Context, id int) (string, bool) {
	if s, ok := ctx.Value(shareLinkPlaintextCtxKey{}).(*shareLinkPlaintextStore); ok {
		return s.get(id)
	}
	return "", false
}

type shareLinkResolver struct{ *Resolver }

func (r *Resolver) ShareLink() ShareLinkResolver {
	return &shareLinkResolver{r}
}

func (r *shareLinkResolver) Token(ctx context.Context, obj *models.ShareLink) (*string, error) {
	if t, ok := getShareLinkPlaintext(ctx, obj.ID); ok {
		return &t, nil
	}
	return nil, nil
}

func (r *shareLinkResolver) URL(ctx context.Context, obj *models.ShareLink) (*string, error) {
	t, ok := getShareLinkPlaintext(ctx, obj.ID)
	if !ok {
		return nil, nil
	}
	baseURL, _ := ctx.Value(BaseURLCtxKey).(string)
	full := baseURL + "/share/" + t
	return &full, nil
}

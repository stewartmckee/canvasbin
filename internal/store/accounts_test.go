package store

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/stewartmckee/canvasbin/internal/keys"
)

// testStore connects to TEST_DATABASE_URL (a throwaway database) and skips when
// it isn't set, so `go test ./...` works without Postgres.
func testStore(t *testing.T) *Store {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	st, err := Open(context.Background(), url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(st.Close)
	return st
}

func newUser(t *testing.T, st *Store, prefix string) *User {
	t.Helper()
	email := fmt.Sprintf("%s-%d@example.com", prefix, time.Now().UnixNano())
	u, err := st.CreateUser(context.Background(), email, "hash", "Test")
	if err != nil {
		t.Fatal(err)
	}
	return u
}

func TestSchemaIsIdempotent(t *testing.T) {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	for i := 0; i < 2; i++ {
		st, err := Open(context.Background(), url)
		if err != nil {
			t.Fatalf("open #%d: %v", i, err)
		}
		st.Close()
	}
}

func TestUserCreateAndLookup(t *testing.T) {
	st, ctx := testStore(t), context.Background()
	u := newUser(t, st, "Alice")
	if u.Verified() || u.Disabled() || u.SubscriptionStatus != "" {
		t.Fatalf("new user should be unverified/enabled: %+v", u)
	}
	// Emails are case-insensitive.
	if _, err := st.CreateUser(ctx, "  "+u.Email+" ", "h", ""); !errors.Is(err, ErrEmailTaken) {
		t.Fatalf("want ErrEmailTaken, got %v", err)
	}
	got, err := st.GetUserByEmail(ctx, u.Email)
	if err != nil || got.ID != u.ID {
		t.Fatalf("by email: %v %v", got, err)
	}
	if _, err := st.GetUserByEmail(ctx, "nobody@example.com"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
	if err := st.SetVerified(ctx, u.ID); err != nil {
		t.Fatal(err)
	}
	if got, _ = st.GetUserByID(ctx, u.ID); !got.Verified() {
		t.Fatal("not verified")
	}
	end := time.Now().Add(24 * time.Hour).UTC().Truncate(time.Second)
	cust := fmt.Sprintf("cus_%d", u.ID)
	if err := st.UpdateSubscription(ctx, u.ID, Subscription{CustomerID: cust, SubscriptionID: "sub_" + cust, PriceID: "price_x", Status: "active", CurrentPeriodEnd: &end, CancelAtPeriodEnd: true}); err != nil {
		t.Fatal(err)
	}
	got, err = st.GetUserByStripeCustomer(ctx, cust)
	if err != nil || got.SubscriptionStatus != "active" || !got.CancelAtPeriodEnd || got.CurrentPeriodEnd == nil || !got.CurrentPeriodEnd.Equal(end) {
		t.Fatalf("subscription not stored: %+v %v", got, err)
	}
	users, err := st.ListUsers(ctx, 10, 0)
	if err != nil || len(users) == 0 {
		t.Fatalf("list users: %v %v", users, err)
	}
}

func TestSessionsAndTokens(t *testing.T) {
	st, ctx := testStore(t), context.Background()
	u := newUser(t, st, "sess")
	h := keys.Hash("session-token-" + u.Email)

	if _, err := st.GetUserBySession(ctx, h); !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("unknown session: %v", err)
	}
	if err := st.CreateSession(ctx, u.ID, h, time.Now().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if got, err := st.GetUserBySession(ctx, h); err != nil || got.ID != u.ID {
		t.Fatalf("session: %v %v", got, err)
	}
	expired := keys.Hash("expired-" + u.Email)
	_ = st.CreateSession(ctx, u.ID, expired, time.Now().Add(-time.Minute))
	if _, err := st.GetSession(ctx, expired); !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("expired session should be rejected: %v", err)
	}
	if err := st.SetDisabled(ctx, u.ID, true); err != nil {
		t.Fatal(err)
	}
	if _, err := st.GetUserBySession(ctx, h); !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("disabling should end sessions: %v", err)
	}
	_ = st.SetDisabled(ctx, u.ID, false)

	_ = st.CreateSession(ctx, u.ID, h, time.Now().Add(time.Hour))
	if err := st.SetPassword(ctx, u.ID, "newhash"); err != nil {
		t.Fatal(err)
	}
	if _, err := st.GetSession(ctx, h); !errors.Is(err, ErrUnauthorized) {
		t.Fatal("password change should end sessions")
	}

	// Email tokens: single use, purpose-bound, expiring.
	tok := keys.Hash("verify-" + u.Email)
	if err := st.CreateEmailToken(ctx, u.ID, PurposeVerify, tok, time.Now().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if _, err := st.ConsumeEmailToken(ctx, PurposeReset, tok); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("wrong purpose: %v", err)
	}
	if id, err := st.ConsumeEmailToken(ctx, PurposeVerify, tok); err != nil || id != u.ID {
		t.Fatalf("consume: %v %v", id, err)
	}
	if _, err := st.ConsumeEmailToken(ctx, PurposeVerify, tok); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("reuse: %v", err)
	}
	old := keys.Hash("old-" + u.Email)
	_ = st.CreateEmailToken(ctx, u.ID, PurposeReset, old, time.Now().Add(-time.Minute))
	if _, err := st.ConsumeEmailToken(ctx, PurposeReset, old); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("expired: %v", err)
	}
}

func TestOwnedPagesClaimAndShares(t *testing.T) {
	st, ctx := testStore(t), context.Background()
	owner, other := newUser(t, st, "owner"), newUser(t, st, "other")

	a, err := st.CreateOwned(ctx, owner.ID, "Mine", "markdown", "# hi")
	if err != nil {
		t.Fatal(err)
	}
	if a.ID == 0 || a.OwnerID == nil || *a.OwnerID != owner.ID || a.WriteKey == "" {
		t.Fatalf("bad owned page: %+v", a)
	}
	if n, _ := st.CountOwned(ctx, owner.ID); n != 1 {
		t.Fatalf("count %d", n)
	}

	// Owner reaches write access through the read key alone.
	got, err := st.GetForUser(ctx, a.ReadKey, owner.ID)
	if err != nil || !got.CanWrite || got.Role != RoleOwner || got.WriteKey != "" {
		t.Fatalf("owner via read key: %+v %v", got, err)
	}
	// Strangers can read but not write.
	got, err = st.GetForUser(ctx, a.ReadKey, other.ID)
	if err != nil || got.CanWrite || got.Role != "" {
		t.Fatalf("stranger: %+v %v", got, err)
	}
	if _, err := st.UpdateForUser(ctx, a.ReadKey, other.ID, Changes{Content: ptr("x")}); !errors.Is(err, ErrReadOnly) {
		t.Fatalf("stranger update: %v", err)
	}
	// A stranger holding the write key can still write.
	if _, err := st.UpdateForUser(ctx, a.WriteKey, other.ID, Changes{Title: ptr("Via key")}); err != nil {
		t.Fatalf("write key update: %v", err)
	}

	// Shares: viewer can't write, editor can, neither can delete.
	res, err := st.ShareWith(ctx, a.ID, owner.ID, other.Email, RoleViewer)
	if err != nil || !res.Shared {
		t.Fatalf("share: %+v %v", res, err)
	}
	if got, _ = st.GetForUser(ctx, a.ReadKey, other.ID); got.CanWrite || got.Role != RoleViewer {
		t.Fatalf("viewer: %+v", got)
	}
	if _, err := st.ShareWith(ctx, a.ID, owner.ID, other.Email, RoleEditor); err != nil {
		t.Fatal(err)
	}
	if got, _ = st.GetForUser(ctx, a.ReadKey, other.ID); !got.CanWrite || got.Role != RoleEditor {
		t.Fatalf("editor: %+v", got)
	}
	if out, err := st.UpdateForUser(ctx, a.ReadKey, other.ID, Changes{Content: ptr("edited")}); err != nil || out.Content != "edited" {
		t.Fatalf("editor update: %+v %v", out, err)
	}
	if err := st.DeleteForUser(ctx, a.ReadKey, other.ID); !errors.Is(err, ErrReadOnly) {
		t.Fatalf("editor delete: %v", err)
	}
	if _, err := st.ShareWith(ctx, a.ID, owner.ID, owner.Email, RoleViewer); !errors.Is(err, ErrConflict) {
		t.Fatalf("share with owner: %v", err)
	}
	lib, _ := st.ListOwnedAndShared(ctx, other.ID)
	if len(lib) != 1 || lib[0].Role != RoleEditor || !lib[0].CanWrite || lib[0].ReadKey != a.ReadKey || lib[0].OwnerEmail != owner.Email {
		t.Fatalf("library: %+v", lib)
	}
	if ok, err := st.RemoveShare(ctx, a.ID, other.Email); err != nil || !ok {
		t.Fatalf("remove: %v %v", ok, err)
	}
	if lib, _ = st.ListOwnedAndShared(ctx, other.ID); len(lib) != 0 {
		t.Fatalf("library should be empty: %+v", lib)
	}

	// Invites for emails without accounts become shares once accepted.
	inviteEmail := fmt.Sprintf("invitee-%d@example.com", time.Now().UnixNano())
	res, err = st.ShareWith(ctx, a.ID, owner.ID, inviteEmail, RoleViewer)
	if err != nil || res.Shared || res.Invite == nil {
		t.Fatalf("invite: %+v %v", res, err)
	}
	if _, invs, _ := st.ListShares(ctx, a.ID); len(invs) != 1 {
		t.Fatalf("invites: %+v", invs)
	}
	invitee, err := st.CreateUser(ctx, inviteEmail, "h", "")
	if err != nil {
		t.Fatal(err)
	}
	if n, err := st.AcceptInvites(ctx, invitee.ID, invitee.Email); err != nil || n != 1 {
		t.Fatalf("accept: %d %v", n, err)
	}
	if got, _ = st.GetForUser(ctx, a.ReadKey, invitee.ID); got.Role != RoleViewer {
		t.Fatalf("invitee role: %+v", got)
	}

	// Owner can delete; shares go with the page.
	if err := st.DeleteForUser(ctx, a.ReadKey, owner.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := st.Get(ctx, a.ReadKey); !errors.Is(err, ErrNotFound) {
		t.Fatalf("deleted page: %v", err)
	}
}

func TestClaim(t *testing.T) {
	st, ctx := testStore(t), context.Background()
	u1, u2 := newUser(t, st, "c1"), newUser(t, st, "c2")
	anon, err := st.Create(ctx, "Anon", "html", "<p>x</p>")
	if err != nil {
		t.Fatal(err)
	}
	if anon.OwnerID != nil {
		t.Fatal("anonymous page has an owner")
	}
	got, err := st.Claim(ctx, anon.WriteKey, u1.ID)
	if err != nil || got.Role != RoleOwner || got.OwnerID == nil || *got.OwnerID != u1.ID {
		t.Fatalf("claim: %+v %v", got, err)
	}
	// Idempotent for the owner, conflict for anyone else.
	if _, err := st.Claim(ctx, anon.WriteKey, u1.ID); err != nil {
		t.Fatalf("re-claim: %v", err)
	}
	if _, err := st.Claim(ctx, anon.WriteKey, u2.ID); !errors.Is(err, ErrConflict) {
		t.Fatalf("steal: %v", err)
	}
	// A read key can't claim.
	if _, err := st.Claim(ctx, anon.ReadKey, u2.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("read key claim: %v", err)
	}
	if lib, _ := st.ListOwnedAndShared(ctx, u1.ID); len(lib) != 1 || lib[0].ReadKey != anon.ReadKey {
		t.Fatalf("library after claim: %+v", lib)
	}
}

func TestOAuthStore(t *testing.T) {
	st, ctx := testStore(t), context.Background()
	u := newUser(t, st, "oauth")
	cid := fmt.Sprintf("client-%d", time.Now().UnixNano())
	if err := st.CreateOAuthClient(ctx, OAuthClient{ClientID: cid, RedirectURIs: []string{"http://localhost/cb"}, ClientName: "t"}); err != nil {
		t.Fatal(err)
	}
	if err := st.CreateOAuthClient(ctx, OAuthClient{ClientID: cid, RedirectURIs: []string{"x"}}); !errors.Is(err, ErrConflict) {
		t.Fatalf("dup client: %v", err)
	}
	c, err := st.GetOAuthClient(ctx, cid)
	if err != nil || len(c.RedirectURIs) != 1 || c.TokenEndpointAuthMethod != "none" {
		t.Fatalf("client: %+v %v", c, err)
	}

	code := keys.Hash("code-" + cid)
	if err := st.CreateAuthCode(ctx, code, OAuthAuthCode{ClientID: cid, UserID: u.ID, RedirectURI: "http://localhost/cb", CodeChallenge: "ch", Scope: "pages", Resource: "https://x/mcp", ExpiresAt: time.Now().Add(time.Minute)}); err != nil {
		t.Fatal(err)
	}
	ac, err := st.ConsumeAuthCode(ctx, code)
	if err != nil || ac.UserID != u.ID || ac.CodeChallengeMethod != "S256" {
		t.Fatalf("consume: %+v %v", ac, err)
	}
	if _, err := st.ConsumeAuthCode(ctx, code); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("code reuse: %v", err)
	}

	acc, ref := keys.Hash("acc-"+cid), keys.Hash("ref-"+cid)
	rexp := time.Now().Add(time.Hour)
	if err := st.CreateOAuthToken(ctx, OAuthToken{AccessHash: acc, RefreshHash: ref, ClientID: cid, UserID: u.ID, Scope: "pages", Resource: "r", AccessExpiresAt: time.Now().Add(time.Minute), RefreshExpiresAt: &rexp}); err != nil {
		t.Fatal(err)
	}
	tok, err := st.GetOAuthTokenByAccess(ctx, acc)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.GetOAuthTokenByRefresh(ctx, ref); err != nil {
		t.Fatal(err)
	}
	acc2, ref2 := keys.Hash("acc2-"+cid), keys.Hash("ref2-"+cid)
	if err := st.RotateOAuthToken(ctx, tok.ID, OAuthToken{AccessHash: acc2, RefreshHash: ref2, ClientID: cid, UserID: u.ID, Scope: "pages", Resource: "r", AccessExpiresAt: time.Now().Add(time.Minute), RefreshExpiresAt: &rexp}); err != nil {
		t.Fatal(err)
	}
	if _, err := st.GetOAuthTokenByAccess(ctx, acc); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("rotated token still valid: %v", err)
	}
	if err := st.RevokeOAuthTokenByHash(ctx, ref2); err != nil {
		t.Fatal(err)
	}
	if _, err := st.GetOAuthTokenByAccess(ctx, acc2); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("revoked token still valid: %v", err)
	}
}

func ptr[T any](v T) *T { return &v }

package httpapi

import (
	"context"
	"fmt"
	"testing"

	"github.com/stewartmckee/canvasbin/internal/store"
)

func TestTeams(t *testing.T) {
	e := newEnv(t, true, 1)
	owner, member, outsider := e.newClient(t), e.newClient(t), e.newClient(t)
	ownerEmail, memberEmail := uniq("towner"), uniq("tmember")
	owner.signup(ownerEmail)
	member.signup(memberEmail)
	outsider.signup(uniq("toutsider"))

	// A new team has no subscription, so it can't hold pages yet.
	tm := owner.want(201, "POST", "/api/teams", map[string]string{"name": "  Acme   Docs "})
	if tm["name"] != "Acme Docs" || tm["role"] != "owner" || tm["active"] != false || tm["member_count"].(float64) != 1 {
		t.Fatalf("created team: %v", tm)
	}
	id := int64(tm["id"].(float64))
	base := fmt.Sprintf("/api/teams/%d", id)
	owner.want(400, "POST", "/api/teams", map[string]string{"name": "  "})
	if out := owner.want(402, "POST", "/api/pages", map[string]any{"content": "x", "team_id": id}); out["code"] != "team_inactive" {
		t.Fatalf("inactive team create: %v", out)
	}
	outsider.want(404, "GET", base, nil)
	outsider.want(404, "POST", "/api/pages", map[string]any{"content": "x", "team_id": id})

	// Activate as the webhook would.
	if err := e.st.UpdateTeamSubscription(context.Background(), id, store.Subscription{
		SubscriptionID: fmt.Sprintf("sub_t_%d", id), Status: "active"}); err != nil {
		t.Fatal(err)
	}

	// Members: existing users join directly; unknown emails are invited.
	owner.want(200, "POST", base+"/members", map[string]string{"email": memberEmail})
	owner.want(409, "POST", base+"/members", map[string]string{"email": memberEmail})
	member.want(403, "POST", base+"/members", map[string]string{"email": uniq("nope")})
	late := uniq("tlate")
	got := owner.want(200, "POST", base+"/members", map[string]string{"email": late, "role": "admin"})
	if len(got["members"].([]any)) != 2 || len(got["invites"].([]any)) != 1 {
		t.Fatalf("members: %v", got)
	}
	if e.mail.get("team", memberEmail) == "" || e.mail.get("team", late) == "" {
		t.Fatal("team invite emails not sent")
	}

	// Team pages don't use the personal quota (1 here).
	owner.want(201, "POST", "/api/pages", map[string]string{"content": "personal"})
	p := owner.want(201, "POST", "/api/pages", map[string]any{"title": "Plan", "content": "<p>v1</p>", "team_id": id})
	if p["team_name"] != "Acme Docs" || p["role"] != "owner" {
		t.Fatalf("team page: %v", p)
	}
	readKey := p["read_key"].(string)

	// Members see and edit team pages but can't delete others' pages.
	mp := member.want(200, "GET", "/api/pages/"+readKey, nil)
	if mp["can_write"] != true || mp["role"] != "editor" || mp["team_role"] != "member" {
		t.Fatalf("member view: %v", mp)
	}
	member.want(200, "PUT", "/api/pages/"+readKey, map[string]string{"content": "<p>v2</p>"})
	member.want(200, "PUT", "/api/pages/"+readKey, map[string]string{"content": "<p>v3</p>"})
	member.want(403, "DELETE", "/api/pages/"+readKey, nil)
	lib := member.want(200, "GET", "/api/library", nil)["pages"].([]any)
	if len(lib) != 1 || lib[0].(map[string]any)["team_name"] != "Acme Docs" {
		t.Fatalf("member library: %v", lib)
	}

	// Outsiders with the read link see the page but no team details or comments.
	op := outsider.want(200, "GET", "/api/pages/"+readKey, nil)
	if op["can_write"] != false || op["team_name"] != nil {
		t.Fatalf("outsider view: %v", op)
	}
	if out := outsider.want(403, "GET", "/api/pages/"+readKey+"/comments", nil); out["code"] != "team_required" {
		t.Fatalf("outsider comments: %v", out)
	}

	// Comments: members post; authors and admins delete.
	c := member.want(201, "POST", "/api/pages/"+readKey+"/comments", map[string]string{"body": " Looks good "})
	if c["body"] != "Looks good" {
		t.Fatalf("comment: %v", c)
	}
	cid := int64(c["id"].(float64))
	member.want(400, "POST", "/api/pages/"+readKey+"/comments", map[string]string{"body": "   "})
	oc := owner.want(201, "POST", "/api/pages/"+readKey+"/comments", map[string]string{"body": "thanks"})
	member.want(403, "DELETE", fmt.Sprintf("/api/pages/%s/comments/%d", readKey, int64(oc["id"].(float64))), nil)
	cs := member.want(200, "GET", "/api/pages/"+readKey+"/comments", nil)["comments"].([]any)
	if len(cs) != 2 || cs[0].(map[string]any)["can_delete"] != true || cs[1].(map[string]any)["can_delete"] != false {
		t.Fatalf("comments: %v", cs)
	}
	owner.want(204, "DELETE", fmt.Sprintf("/api/pages/%s/comments/%d", readKey, cid), nil)

	// Timeline: creation, one folded edit entry, comments, membership.
	act := owner.want(200, "GET", base+"/activity", nil)["activity"].([]any)
	counts := map[string]int{}
	for _, a := range act {
		counts[a.(map[string]any)["action"].(string)]++
	}
	if counts[store.ActivityPageCreated] != 1 || counts[store.ActivityPageEdited] != 1 ||
		counts[store.ActivityCommentAdded] != 2 || counts[store.ActivityMemberJoined] != 1 || counts[store.ActivityMemberInvited] != 1 {
		t.Fatalf("activity: %v", counts)
	}
	pa := member.want(200, "GET", "/api/pages/"+readKey+"/activity", nil)["activity"].([]any)
	if len(pa) != 4 {
		t.Fatalf("page activity: %v", pa)
	}

	// The invited email joins on verification, as an admin.
	lateC := e.newClient(t)
	lateC.signup(late)
	lt := lateC.want(200, "GET", base, nil)
	if lt["team"].(map[string]any)["role"] != "admin" || len(lt["invites"].([]any)) != 0 {
		t.Fatalf("late joiner: %v", lt)
	}
	lateC.want(204, "DELETE", "/api/pages/"+readKey, nil)
	if act := owner.want(200, "GET", base+"/activity", nil)["activity"].([]any); act[0].(map[string]any)["action"] != store.ActivityPageDeleted {
		t.Fatalf("delete not logged: %v", act[0])
	}

	// Roles and removal.
	me := member.want(200, "GET", "/api/me", nil)
	memberID := int64(me["user"].(map[string]any)["id"].(float64))
	ownerID := int64(owner.want(200, "GET", "/api/me", nil)["user"].(map[string]any)["id"].(float64))
	lateC.want(403, "PATCH", fmt.Sprintf("%s/members/%d", base, memberID), map[string]string{"role": "admin"})
	owner.want(200, "PATCH", fmt.Sprintf("%s/members/%d", base, memberID), map[string]string{"role": "admin"})
	owner.want(409, "PATCH", fmt.Sprintf("%s/members/%d", base, ownerID), map[string]string{"role": "member"})
	lateC.want(409, "DELETE", fmt.Sprintf("%s/members/%d", base, ownerID), nil)
	lateC.want(403, "DELETE", fmt.Sprintf("%s/members/%d", base, memberID), nil) // admins can't remove admins
	own := member.want(201, "POST", "/api/pages", map[string]any{"content": "mine", "team_id": id})["read_key"].(string)
	member.want(204, "DELETE", fmt.Sprintf("%s/members/%d", base, memberID), nil) // leave
	member.want(404, "GET", base, nil)
	// Leaving the team drops access even to pages you created there.
	member.want(403, "PUT", "/api/pages/"+own, map[string]string{"content": "gone"})

	// An active team can't be deleted; a lapsed one can.
	owner.want(409, "DELETE", base, nil)
	if err := e.st.UpdateTeamSubscription(context.Background(), id, store.Subscription{Status: "canceled"}); err != nil {
		t.Fatal(err)
	}
	lateC.want(403, "DELETE", base, nil)
	owner.want(204, "DELETE", base, nil)
	owner.want(404, "GET", base, nil)
}

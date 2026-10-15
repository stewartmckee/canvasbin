package store

import (
	"context"
	"time"
)

// Timeline actions.
const (
	ActivityPageCreated    = "page_created"
	ActivityPageEdited     = "page_edited"
	ActivityPageDeleted    = "page_deleted"
	ActivityCommentAdded   = "comment_added"
	ActivityMemberJoined   = "member_joined"
	ActivityMemberInvited  = "member_invited"
	ActivityMemberRemoved  = "member_removed"
	ActivityMemberLeft     = "member_left"
	ActivityRoleChanged    = "role_changed"
	ActivityTeamRenamed    = "team_renamed"
	ActivityTeamSubscribed = "team_subscribed"
)

// editWindow folds repeated edits of a page by the same person into one entry,
// so autosaves don't flood the timeline.
const editWindow = 15 * time.Minute

// Activity is one timeline entry.
type Activity struct {
	ID          int64     `json:"id"`
	UserEmail   string    `json:"user_email,omitempty"`
	UserName    string    `json:"user_name,omitempty"`
	PageTitle   string    `json:"page_title,omitempty"`
	PageReadKey string    `json:"page_read_key,omitempty"`
	Action      string    `json:"action"`
	Detail      string    `json:"detail,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
}

// ActivityEntry is what callers record.
type ActivityEntry struct {
	TeamID    int64
	UserID    int64
	PageID    *int64
	PageTitle string
	Action    string
	Detail    string
}

// LogActivity records a timeline entry.
func (s *Store) LogActivity(ctx context.Context, e ActivityEntry) error {
	var userID *int64
	if e.UserID != 0 {
		userID = &e.UserID
	}
	if e.Action == ActivityPageEdited && e.PageID != nil {
		tag, err := s.db.Exec(ctx,
			`UPDATE team_activity SET created_at = now(), page_title = $4
			 WHERE id = (
			   SELECT id FROM team_activity
			   WHERE team_id = $1 AND page_id = $2 AND user_id IS NOT DISTINCT FROM $3
			     AND action = 'page_edited' AND created_at > now() - make_interval(secs => $5)
			   ORDER BY created_at DESC LIMIT 1)`,
			e.TeamID, *e.PageID, userID, e.PageTitle, editWindow.Seconds())
		if err != nil {
			return err
		}
		if tag.RowsAffected() > 0 {
			return nil
		}
	}
	_, err := s.db.Exec(ctx,
		`INSERT INTO team_activity (team_id, user_id, page_id, page_title, action, detail)
		 VALUES ($1, $2, $3, $4, $5, $6)`,
		e.TeamID, userID, e.PageID, e.PageTitle, e.Action, e.Detail)
	return err
}

// ListActivity returns timeline entries newest first. A non-nil pageID limits
// it to one page.
func (s *Store) ListActivity(ctx context.Context, teamID int64, pageID *int64, limit int) ([]Activity, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := s.db.Query(ctx,
		`SELECT a.id, COALESCE(u.email, ''), COALESCE(u.display_name, ''), a.page_title,
		        COALESCE(p.read_key, ''), a.action, a.detail, a.created_at
		 FROM team_activity a
		 LEFT JOIN users u ON u.id = a.user_id
		 LEFT JOIN pages p ON p.id = a.page_id AND p.team_id = a.team_id
		 WHERE a.team_id = $1
		   AND ($2::bigint IS NULL OR a.page_id = $2)
		 ORDER BY a.created_at DESC, a.id DESC
		 LIMIT $3`, teamID, pageID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Activity{}
	for rows.Next() {
		var a Activity
		if err := rows.Scan(&a.ID, &a.UserEmail, &a.UserName, &a.PageTitle, &a.PageReadKey,
			&a.Action, &a.Detail, &a.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

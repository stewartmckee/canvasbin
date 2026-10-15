package store

import (
	"context"
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
)

const (
	TeamRoleOwner  = "owner"
	TeamRoleAdmin  = "admin"
	TeamRoleMember = "member"

	MaxTeamNameRunes = 80
)

var (
	ErrTeamInactive = errors.New("the team subscription is not active")
	ErrTeamOwner    = errors.New("the team owner can't be removed or demoted")
	ErrBadTeamName  = errors.New("team name is required")
	ErrBadTeamRole  = errors.New(`role must be "admin" or "member"`)
)

// Team is a shared space with its own subscription. Role, MemberCount and
// PageCount are filled in relative to the user who asked.
type Team struct {
	ID                   int64      `json:"id"`
	Name                 string     `json:"name"`
	CreatedBy            *int64     `json:"-"`
	StripeCustomerID     *string    `json:"-"`
	StripeSubscriptionID *string    `json:"-"`
	SubscriptionStatus   string     `json:"subscription_status"`
	CurrentPeriodEnd     *time.Time `json:"current_period_end,omitempty"`
	CancelAtPeriodEnd    bool       `json:"cancel_at_period_end"`
	CreatedAt            time.Time  `json:"created_at"`
	Role                 string     `json:"role,omitempty"`
	MemberCount          int        `json:"member_count"`
	PageCount            int        `json:"page_count"`
}

// Active reports whether the team's subscription currently allows new pages.
func (t *Team) Active() bool {
	switch t.SubscriptionStatus {
	case "trialing", "active", "past_due":
		return true
	}
	return false
}

// CanManage reports whether the caller may change members and billing.
func (t *Team) CanManage() bool { return t.Role == TeamRoleOwner || t.Role == TeamRoleAdmin }

type TeamMember struct {
	UserID      int64     `json:"user_id"`
	Email       string    `json:"email"`
	DisplayName string    `json:"display_name"`
	Role        string    `json:"role"`
	CreatedAt   time.Time `json:"created_at"`
}

type TeamInvite struct {
	ID        int64     `json:"id"`
	Email     string    `json:"email"`
	Role      string    `json:"role"`
	CreatedAt time.Time `json:"created_at"`
}

func cleanTeamName(n string) (string, error) {
	n = strings.Join(strings.Fields(n), " ")
	if n == "" {
		return "", ErrBadTeamName
	}
	if utf8.RuneCountInString(n) > MaxTeamNameRunes {
		n = string([]rune(n)[:MaxTeamNameRunes])
	}
	return n, nil
}

const teamColumns = `t.id, t.name, t.created_by, t.stripe_customer_id, t.stripe_subscription_id,
	t.subscription_status, t.current_period_end, t.cancel_at_period_end, t.created_at,
	(SELECT count(*) FROM team_members m WHERE m.team_id = t.id),
	(SELECT count(*) FROM pages p WHERE p.team_id = t.id)`

func scanTeam(row pgx.Row, withRole bool) (*Team, error) {
	var t Team
	dest := []any{&t.ID, &t.Name, &t.CreatedBy, &t.StripeCustomerID, &t.StripeSubscriptionID,
		&t.SubscriptionStatus, &t.CurrentPeriodEnd, &t.CancelAtPeriodEnd, &t.CreatedAt,
		&t.MemberCount, &t.PageCount}
	if withRole {
		dest = append(dest, &t.Role)
	}
	err := row.Scan(dest...)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &t, nil
}

// CreateTeam creates a team with userID as its owner.
func (s *Store) CreateTeam(ctx context.Context, userID int64, name string) (*Team, error) {
	name, err := cleanTeamName(name)
	if err != nil {
		return nil, err
	}
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx) //nolint:errcheck
	var id int64
	if err := tx.QueryRow(ctx,
		`INSERT INTO teams (name, created_by) VALUES ($1, $2) RETURNING id`, name, userID).Scan(&id); err != nil {
		return nil, err
	}
	if _, err := tx.Exec(ctx,
		`INSERT INTO team_members (team_id, user_id, role) VALUES ($1, $2, 'owner')`, id, userID); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return s.GetTeam(ctx, id, userID)
}

// GetTeam returns a team the user belongs to; anyone else gets ErrNotFound.
func (s *Store) GetTeam(ctx context.Context, teamID, userID int64) (*Team, error) {
	return scanTeam(s.db.QueryRow(ctx,
		`SELECT `+teamColumns+`, tm.role FROM teams t
		 JOIN team_members tm ON tm.team_id = t.id AND tm.user_id = $2
		 WHERE t.id = $1`, teamID, userID), true)
}

// GetTeamByID returns a team without a membership check, for billing.
func (s *Store) GetTeamByID(ctx context.Context, teamID int64) (*Team, error) {
	return scanTeam(s.db.QueryRow(ctx, `SELECT `+teamColumns+` FROM teams t WHERE t.id = $1`, teamID), false)
}

func (s *Store) GetTeamByStripeCustomer(ctx context.Context, customerID string) (*Team, error) {
	return scanTeam(s.db.QueryRow(ctx,
		`SELECT `+teamColumns+` FROM teams t WHERE t.stripe_customer_id = $1`, customerID), false)
}

// ListTeams returns the teams userID belongs to, oldest first.
func (s *Store) ListTeams(ctx context.Context, userID int64) ([]Team, error) {
	rows, err := s.db.Query(ctx,
		`SELECT `+teamColumns+`, tm.role FROM teams t
		 JOIN team_members tm ON tm.team_id = t.id AND tm.user_id = $1
		 ORDER BY t.created_at, t.id`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Team{}
	for rows.Next() {
		t, err := scanTeam(rows, true)
		if err != nil {
			return nil, err
		}
		out = append(out, *t)
	}
	return out, rows.Err()
}

func (s *Store) RenameTeam(ctx context.Context, teamID int64, name string) error {
	name, err := cleanTeamName(name)
	if err != nil {
		return err
	}
	_, err = s.db.Exec(ctx, `UPDATE teams SET name = $2, updated_at = now() WHERE id = $1`, teamID, name)
	return err
}

// DeleteTeam removes a team and its memberships. Its pages stay with their
// creators as personal pages.
func (s *Store) DeleteTeam(ctx context.Context, teamID int64) error {
	_, err := s.db.Exec(ctx, `DELETE FROM teams WHERE id = $1`, teamID)
	return err
}

func (s *Store) ListTeamMembers(ctx context.Context, teamID int64) ([]TeamMember, []TeamInvite, error) {
	rows, err := s.db.Query(ctx,
		`SELECT u.id, u.email, u.display_name, tm.role, tm.created_at
		 FROM team_members tm JOIN users u ON u.id = tm.user_id
		 WHERE tm.team_id = $1
		 ORDER BY CASE tm.role WHEN 'owner' THEN 0 WHEN 'admin' THEN 1 ELSE 2 END, tm.created_at`, teamID)
	if err != nil {
		return nil, nil, err
	}
	members := []TeamMember{}
	for rows.Next() {
		var m TeamMember
		if err := rows.Scan(&m.UserID, &m.Email, &m.DisplayName, &m.Role, &m.CreatedAt); err != nil {
			rows.Close()
			return nil, nil, err
		}
		members = append(members, m)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, nil, err
	}
	irows, err := s.db.Query(ctx,
		`SELECT id, email, role, created_at FROM team_invites WHERE team_id = $1 ORDER BY created_at`, teamID)
	if err != nil {
		return nil, nil, err
	}
	defer irows.Close()
	invites := []TeamInvite{}
	for irows.Next() {
		var inv TeamInvite
		if err := irows.Scan(&inv.ID, &inv.Email, &inv.Role, &inv.CreatedAt); err != nil {
			return nil, nil, err
		}
		invites = append(invites, inv)
	}
	return members, invites, irows.Err()
}

// AddTeamMember adds an existing user to the team, or records an invite for an
// email with no account yet. It returns the user when they were added directly.
// Someone already in the team is ErrConflict.
func (s *Store) AddTeamMember(ctx context.Context, teamID, inviterID int64, email, role string) (*User, *TeamInvite, error) {
	if role != TeamRoleAdmin && role != TeamRoleMember {
		return nil, nil, ErrBadTeamRole
	}
	email = NormalizeEmail(email)
	u, err := s.GetUserByEmail(ctx, email)
	switch {
	case err == nil:
		tag, err := s.db.Exec(ctx,
			`INSERT INTO team_members (team_id, user_id, role) VALUES ($1, $2, $3)
			 ON CONFLICT (team_id, user_id) DO NOTHING`, teamID, u.ID, role)
		if err != nil {
			return nil, nil, err
		}
		if tag.RowsAffected() == 0 {
			return nil, nil, ErrConflict
		}
		return u, nil, nil
	case errors.Is(err, ErrNotFound):
		inv := &TeamInvite{}
		err := s.db.QueryRow(ctx,
			`INSERT INTO team_invites (team_id, email, role, invited_by) VALUES ($1, $2, $3, $4)
			 ON CONFLICT (team_id, email) DO UPDATE SET role = EXCLUDED.role
			 RETURNING id, email, role, created_at`, teamID, email, role, inviterID,
		).Scan(&inv.ID, &inv.Email, &inv.Role, &inv.CreatedAt)
		if err != nil {
			return nil, nil, err
		}
		return nil, inv, nil
	default:
		return nil, nil, err
	}
}

// RemoveTeamMember removes a member. The owner can't be removed.
func (s *Store) RemoveTeamMember(ctx context.Context, teamID, userID int64) error {
	var role string
	err := s.db.QueryRow(ctx,
		`SELECT role FROM team_members WHERE team_id = $1 AND user_id = $2`, teamID, userID).Scan(&role)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	if role == TeamRoleOwner {
		return ErrTeamOwner
	}
	_, err = s.db.Exec(ctx, `DELETE FROM team_members WHERE team_id = $1 AND user_id = $2`, teamID, userID)
	return err
}

// SetTeamMemberRole switches a member between admin and member.
func (s *Store) SetTeamMemberRole(ctx context.Context, teamID, userID int64, role string) error {
	if role != TeamRoleAdmin && role != TeamRoleMember {
		return ErrBadTeamRole
	}
	tag, err := s.db.Exec(ctx,
		`UPDATE team_members SET role = $3 WHERE team_id = $1 AND user_id = $2 AND role <> 'owner'`,
		teamID, userID, role)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		var exists bool
		if err := s.db.QueryRow(ctx,
			`SELECT EXISTS (SELECT 1 FROM team_members WHERE team_id = $1 AND user_id = $2)`,
			teamID, userID).Scan(&exists); err != nil {
			return err
		}
		if exists {
			return ErrTeamOwner
		}
		return ErrNotFound
	}
	return nil
}

func (s *Store) CancelTeamInvite(ctx context.Context, teamID, inviteID int64) error {
	tag, err := s.db.Exec(ctx, `DELETE FROM team_invites WHERE team_id = $1 AND id = $2`, teamID, inviteID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// AcceptTeamInvites turns pending team invites for the user's (verified) email
// into memberships and returns the teams joined.
func (s *Store) AcceptTeamInvites(ctx context.Context, userID int64, email string) ([]int64, error) {
	email = NormalizeEmail(email)
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx) //nolint:errcheck
	rows, err := tx.Query(ctx,
		`INSERT INTO team_members (team_id, user_id, role)
		 SELECT team_id, $1, role FROM team_invites WHERE email = $2
		 ON CONFLICT (team_id, user_id) DO NOTHING
		 RETURNING team_id`, userID, email)
	if err != nil {
		return nil, err
	}
	var joined []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return nil, err
		}
		joined = append(joined, id)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if _, err := tx.Exec(ctx, `DELETE FROM team_invites WHERE email = $1`, email); err != nil {
		return nil, err
	}
	return joined, tx.Commit(ctx)
}

// CountTeamPages is the number of pages in a team.
func (s *Store) CountTeamPages(ctx context.Context, teamID int64) (int, error) {
	var n int
	err := s.db.QueryRow(ctx, `SELECT count(*) FROM pages WHERE team_id = $1`, teamID).Scan(&n)
	return n, err
}

func (s *Store) SetTeamStripeCustomer(ctx context.Context, teamID int64, customerID string) error {
	_, err := s.db.Exec(ctx,
		`UPDATE teams SET stripe_customer_id = $2, updated_at = now() WHERE id = $1`, teamID, customerID)
	if isUniqueViolation(err) {
		return ErrConflict
	}
	return err
}

// UpdateTeamSubscription overwrites a team's subscription fields.
func (s *Store) UpdateTeamSubscription(ctx context.Context, teamID int64, sub Subscription) error {
	_, err := s.db.Exec(ctx,
		`UPDATE teams SET
		   stripe_customer_id = COALESCE(NULLIF($2, ''), stripe_customer_id),
		   stripe_subscription_id = NULLIF($3, ''),
		   subscription_status = $4,
		   current_period_end = $5,
		   cancel_at_period_end = $6,
		   updated_at = now()
		 WHERE id = $1`,
		teamID, sub.CustomerID, sub.SubscriptionID, sub.Status, sub.CurrentPeriodEnd, sub.CancelAtPeriodEnd)
	if isUniqueViolation(err) {
		return ErrConflict
	}
	return err
}

// InAnyActiveTeam reports whether the user belongs to a team with an active
// subscription, which unlocks team-plan features such as email sharing.
func (s *Store) InAnyActiveTeam(ctx context.Context, userID int64) (bool, error) {
	var ok bool
	err := s.db.QueryRow(ctx,
		`SELECT EXISTS (
		   SELECT 1 FROM team_members tm JOIN teams t ON t.id = tm.team_id
		   WHERE tm.user_id = $1 AND t.subscription_status IN ('trialing', 'active', 'past_due'))`,
		userID).Scan(&ok)
	return ok, err
}

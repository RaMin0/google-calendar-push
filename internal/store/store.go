package store

import (
	"context"
	"time"

	"github.com/jackc/pgx/v4"
)

type User struct {
	ID           string
	AccessToken  string
	RefreshToken string
	ExpiresAt    time.Time
}

type Channel struct {
	ID            string
	UserID        string
	Token         string
	LastSyncToken string
	ResourceID    string
	CalendarID    string
}

type Store struct {
	db *pgx.Conn
}

func New(db *pgx.Conn) *Store {
	return &Store{db}
}

func (s *Store) FindOrCreateUser(ctx context.Context, u *User) error {
	_, err := s.db.Exec(ctx, `
		insert into users
			(id, access_token, refresh_token, expires_at)
		values
			($1, $2, $3, $4)
		on conflict (id) do update
			set access_token = excluded.access_token,
			    refresh_token = coalesce(nullif(excluded.refresh_token, ''), users.refresh_token)
	`, u.ID, u.AccessToken, u.RefreshToken, u.ExpiresAt)
	return err
}

func (s *Store) FindUser(ctx context.Context, userID string) (*User, error) {
	row := s.db.QueryRow(ctx, `
		select id, access_token, refresh_token, expires_at
		from users
		where id = $1
		limit 1
	`, userID)
	var u User
	if err := row.Scan(&u.ID, &u.AccessToken, &u.RefreshToken, &u.ExpiresAt); err != nil {
		return nil, err
	}
	return &u, nil
}

func (s *Store) UpdateUser(ctx context.Context, u *User) error {
	_, err := s.db.Exec(ctx, `
		update users
		set access_token = $1,
				refresh_token = coalesce(nullif($2, ''), refresh_token),
				expires_at = $3
		where id = $4
	`, u.AccessToken, u.RefreshToken, u.ExpiresAt, u.ID)
	return err
}

func (s *Store) FindOrCreateChannel(ctx context.Context, c *Channel) error {
	_, err := s.db.Exec(ctx, `
		insert into channels
			(id, user_id, token, last_sync_token, resource_id, calendar_id)
		values
			($1, $2, $3, $4, $5, $6)
		on conflict (id) do update
			set token = excluded.token, last_sync_token = excluded.last_sync_token
`, c.ID, c.UserID, c.Token, c.LastSyncToken, c.ResourceID, c.CalendarID)
	return err
}

func (s *Store) FindChannel(ctx context.Context, channelID, channelToken, channelResourceID string) (*Channel, error) {
	row := s.db.QueryRow(ctx, `
		select id, user_id, token, last_sync_token, resource_id, calendar_id
		from channels
		where id = $1 and token = $2 and resource_id = $3
		limit 1
	`, channelID, channelToken, channelResourceID)
	var c Channel
	if err := row.Scan(&c.ID, &c.UserID, &c.Token, &c.LastSyncToken, &c.ResourceID, &c.CalendarID); err != nil {
		return nil, err
	}
	return &c, nil
}

func (s *Store) UpdateChannelLastSyncToken(ctx context.Context, channelID, lastSyncToken string) error {
	_, err := s.db.Exec(ctx, `
		update channels
		set last_sync_token = $1
		where id = $2
	`, lastSyncToken, channelID)
	return err
}

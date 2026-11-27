package db

import "time"

// FreeGrant maps table free_grants: free usage the team gives to one email while it has no subscription.
type FreeGrant struct {
	Email  string         `db:"email" json:"email"`
	Quotas map[string]int `db:"quotas" json:"quotas"`
	Note   string         `db:"note" json:"note"`
	// GivenBy is the admin who last set the grant.
	GivenBy   *string   `db:"given_by" json:"given_by"`
	CreatedAt time.Time `db:"created_at" json:"created_at"`
	UpdatedAt time.Time `db:"updated_at" json:"updated_at"`
}

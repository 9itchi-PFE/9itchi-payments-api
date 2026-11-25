package repositories

import "context"

// EraseRepository is what this service does about one user when their account is deleted.
type EraseRepository interface {
	// EraseUser deletes nothing: the payments and their receipts are accounting records and stay, under the id of the account only. What it does is stop the subscriptions, so that no renewal is asked for an account that no longer exists.
	// It returns how many rows changed. A user who has nothing here, or a second call, is not an error.
	EraseUser(ctx context.Context, userID string) (int64, error)
}

type eraseRepository struct {
	store *Store
}

func NewEraseRepository(store *Store) EraseRepository {
	return &eraseRepository{store: store}
}

// eraseStatements run in this order, in one transaction: all of it goes or none of it.
var eraseStatements = []string{
	`UPDATE subscriptions SET stopped_at = now(), updated_at = now() WHERE user_id = $1 AND stopped_at IS NULL`,
}

func (r *eraseRepository) EraseUser(ctx context.Context, userID string) (int64, error) {
	ctx, cancel := QueryTimeout(ctx)
	defer cancel()
	var rows int64
	err := r.store.WithTx(ctx, func(ctx context.Context) error {
		rows = 0
		for _, q := range eraseStatements {
			tag, err := r.store.Conn(ctx).Exec(ctx, q, userID)
			if err != nil {
				return MapError(err)
			}
			rows += tag.RowsAffected()
		}
		return nil
	})
	return rows, err
}

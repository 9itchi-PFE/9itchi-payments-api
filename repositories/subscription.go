package repositories

// SubscriptionRepository is the only layer that touches PostgreSQL for subscription data.
// Use store.Conn(ctx), a context from QueryTimeout, and return MapError(err).
type SubscriptionRepository interface {
}

type subscriptionRepository struct {
	store *Store
}

func NewSubscriptionRepository(store *Store) SubscriptionRepository {
	return &subscriptionRepository{store: store}
}

package repositories

// PromoRepository is the only layer that touches PostgreSQL for promo data.
// Use store.Conn(ctx), a context from QueryTimeout, and return MapError(err).
type PromoRepository interface {
}

type promoRepository struct {
	store *Store
}

func NewPromoRepository(store *Store) PromoRepository {
	return &promoRepository{store: store}
}

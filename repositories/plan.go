package repositories

// PlanRepository is the only layer that touches PostgreSQL for plan data.
// Use store.Conn(ctx), a context from QueryTimeout, and return MapError(err).
type PlanRepository interface {
}

type planRepository struct {
	store *Store
}

func NewPlanRepository(store *Store) PlanRepository {
	return &planRepository{store: store}
}

package eventstoredb

//
//type BaseRepository struct {
//	repository *aggregate.Repository
//}
//
//func NewBaseRepository(repository *aggregate.Repository) *BaseRepository {
//	return &BaseRepository{repository: repository}
//}
//
//func (r *BaseRepository) Get(ctx context.Context, aggregateID aggregate.ID) (aggregate.Root, error) {
//	root, err := r.repository.GetAggregateRoot(ctx, aggregateID)
//	if err != nil {
//		return nil, err
//	}
//
//	return root, nil
//}
//
//func (r *BaseRepository) Create(ctx context.Context, aggregateRoot aggregate.Root) error {
//	return r.repository.SaveAggregateRoot(ctx, aggregateRoot)
//}

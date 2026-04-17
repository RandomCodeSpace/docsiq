package store

import "context"

// DistinctEmbeddingModels returns the set of embedding model IDs present in
// the embeddings table, ordered by most-used first. Empty slice (nil error)
// when there are no embeddings yet.
//
// Used by vectorindex.BuildFromStore to pick the dominant model at boot.
func (s *Store) DistinctEmbeddingModels(ctx context.Context) ([]string, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT model, COUNT(*) AS n FROM embeddings GROUP BY model ORDER BY n DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var models []string
	for rows.Next() {
		var m string
		var n int
		if err := rows.Scan(&m, &n); err != nil {
			return nil, err
		}
		models = append(models, m)
	}
	return models, rows.Err()
}

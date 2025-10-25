package adapters

import "context"

type Adapter interface {
	Collect(ctx context.Context, windowSeconds int) (DataFrame, error)
	Name() string
}

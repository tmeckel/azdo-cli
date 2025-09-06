package util

import "context"

type ContextAware interface {
	Context() context.Context
}

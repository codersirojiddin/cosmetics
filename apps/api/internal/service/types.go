package service

import "context"

// contextLike lets helpers accept request contexts without importing context everywhere.
type contextLike = context.Context

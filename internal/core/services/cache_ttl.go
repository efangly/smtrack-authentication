package services

import "time"

// How long cached reads live. Writes invalidate by prefix (see ADR 0002), so
// these only bound how stale a value can get if an invalidation is missed.
const (
	listTTL = 10 * time.Hour
	itemTTL = 24 * time.Hour
)

package redis

// Config holds Redis connection settings.
type Config struct {
	// URL is a redis:// or rediss:// connection string, e.g. redis://localhost:6379/0
	URL string
}

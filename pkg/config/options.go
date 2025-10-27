package config

type options struct {
	withYaml string
}

type Option func(*options)

func (o *options) apply(opts ...Option) {
	for _, opt := range opts {
		opt(o)
	}
}

// WithLocalYAML specifies a path to a local YAML file to load config from.
// If the file does not exist, an error is not returned.
func WithLocalYAML(path string) Option {
	return func(o *options) {
		o.withYaml = path
	}
}

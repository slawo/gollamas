package main

type RouterOption interface {
	ApplyTo(opts *RouterOptions) error
}

type RouterOptions struct {
	Aliases map[string]string
}

func (o *RouterOptions) ApplyTo(opts *RouterOptions) error {
	applyOptionAliasConfig(opts, o.Aliases)
	return nil
}

type RouterOptionFunc func(opts *RouterOptions) error

func (o RouterOptionFunc) ApplyTo(opts *RouterOptions) error {
	return o(opts)
}

func WithAliases(aliases map[string]string) RouterOptionFunc {
	return func(opts *RouterOptions) error {
		applyOptionAliasConfig(opts, aliases)
		return nil
	}
}

func WithAlias(alias, model string) RouterOptionFunc {
	return func(opts *RouterOptions) error {
		if opts.Aliases == nil {
			opts.Aliases = map[string]string{}
		}
		opts.Aliases[alias] = model
		return nil
	}
}

func applyOptionAliasConfig(opts *RouterOptions, aliases map[string]string) {
	if opts.Aliases == nil {
		opts.Aliases = map[string]string{}
	}
	for k, v := range aliases {
		opts.Aliases[k] = v
	}
}

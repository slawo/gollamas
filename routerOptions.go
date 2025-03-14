package main

type RouterOption interface {
	ApplyTo(opts *RouterOptions) error
}

type RouterOptions struct {
	ExposeAliases bool
	Aliases       map[ModelID]ModelID
}

func (o *RouterOptions) ApplyTo(opts *RouterOptions) error {
	opts.ExposeAliases = o.ExposeAliases
	applyOptionAliasConfig(opts, o.Aliases)
	return nil
}

type RouterOptionFunc func(opts *RouterOptions) error

func (o RouterOptionFunc) ApplyTo(opts *RouterOptions) error {
	return o(opts)
}

func WithAliases(aliases map[ModelID]ModelID) RouterOptionFunc {
	return func(opts *RouterOptions) error {
		applyOptionAliasConfig(opts, aliases)
		return nil
	}
}

func WithAlias(alias, model ModelID) RouterOptionFunc {
	return func(opts *RouterOptions) error {
		if opts.Aliases == nil {
			opts.Aliases = map[ModelID]ModelID{}
		}
		opts.Aliases[alias] = model
		return nil
	}
}

func WithExposeAliases(expose bool) RouterOptionFunc {
	return func(opts *RouterOptions) error {
		opts.ExposeAliases = expose
		return nil
	}
}

func applyOptionAliasConfig(opts *RouterOptions, aliases map[ModelID]ModelID) {
	if opts.Aliases == nil {
		opts.Aliases = map[ModelID]ModelID{}
	}
	for k, v := range aliases {
		opts.Aliases[k] = v
	}
}

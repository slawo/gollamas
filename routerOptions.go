package main

type RouterOption interface {
	Apply(opts *RouterOptions) error
}

type RouterOptions struct {
	aliases map[string]string
}

func (o *RouterOptions) Apply(opts *RouterOptions) error {
	if opts.aliases == nil {
		opts.aliases = map[string]string{}
	}
	for k, v := range o.aliases {
		opts.aliases[k] = v
	}
	return nil
}

type RouterOptionFunc func(opts *RouterOptions) error

func (o RouterOptionFunc) Apply(opts *RouterOptions) error {
	return o(opts)
}

func WithAliases(aliases map[string]string) RouterOptionFunc {
	return func(opts *RouterOptions) error {
		if opts.aliases == nil {
			opts.aliases = map[string]string{}
		}
		for k, v := range aliases {
			opts.aliases[k] = v
		}
		return nil
	}
}

func WithAlias(alias, model string) RouterOptionFunc {
	return func(opts *RouterOptions) error {
		if opts.aliases == nil {
			opts.aliases = map[string]string{}
		}
		opts.aliases[alias] = model
		return nil
	}
}

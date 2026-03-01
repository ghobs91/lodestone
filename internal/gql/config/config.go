package config

import (
	"github.com/ghobs91/lodestone/internal/gql"
	"github.com/ghobs91/lodestone/internal/gql/resolvers"
	"github.com/ghobs91/lodestone/internal/lazy"
	"go.uber.org/fx"
)

type Params struct {
	fx.In
	ResolverRoot lazy.Lazy[*resolvers.Resolver]
}

func New(p Params) lazy.Lazy[gql.Config] {
	return lazy.New(func() (gql.Config, error) {
		root, err := p.ResolverRoot.Get()
		if err != nil {
			return gql.Config{}, err
		}

		return gql.Config{Resolvers: root}, nil
	})
}

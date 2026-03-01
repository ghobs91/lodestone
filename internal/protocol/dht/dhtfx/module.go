package dhtfx

import (
	"github.com/ghobs91/lodestone/internal/config/configfx"
	"github.com/ghobs91/lodestone/internal/protocol"
	"github.com/ghobs91/lodestone/internal/protocol/dht/client"
	"github.com/ghobs91/lodestone/internal/protocol/dht/ktable"
	"github.com/ghobs91/lodestone/internal/protocol/dht/responder"
	"github.com/ghobs91/lodestone/internal/protocol/dht/server"
	"go.uber.org/fx"
)

func New() fx.Option {
	return fx.Module(
		"dht",
		configfx.NewConfigModule[server.Config]("dht_server", server.NewDefaultConfig()),
		fx.Provide(
			fx.Annotated{
				Name:   "dht_node_id",
				Target: protocol.RandomNodeIDWithClientSuffix,
			},
			client.New,
			ktable.New,
			responder.New,
			server.New,
		),
	)
}

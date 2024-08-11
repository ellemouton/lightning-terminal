package faraday

import (
	"github.com/btcsuite/btclog"
	"github.com/lightninglabs/lightning-terminal/config"
	"github.com/lightninglabs/lightning-terminal/subservers"
)

func RegisterSubServer(cfg *config.Config, _ btclog.Logger) (
	subservers.SubServer, error) {

	return newFaradaySubServer(
		cfg.Faraday, cfg.Remote.Faraday, cfg.FaradayMode,
	)
}

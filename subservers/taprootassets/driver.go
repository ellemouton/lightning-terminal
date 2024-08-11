package taprootassets

import (
	"github.com/btcsuite/btclog"
	"github.com/lightninglabs/lightning-terminal/config"
	"github.com/lightninglabs/lightning-terminal/subservers"
)

func RegisterSubServer(cfg *config.Config, logger btclog.Logger) (
	subservers.SubServer, error) {

	return newTaprootAssetsSubServer(
		cfg.Network, cfg.TaprootAssets, cfg.Remote.TaprootAssets,
		cfg.TaprootAssetsMode, cfg.LndRemote, logger,
	), nil
}

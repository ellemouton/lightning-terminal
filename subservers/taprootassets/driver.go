package taprootassets

import (
	"fmt"

	"github.com/btcsuite/btclog"
	"github.com/lightninglabs/lightning-terminal/config"
	"github.com/lightninglabs/lightning-terminal/subservers"
)

func init() {
	subServer := &subservers.SubServerDriver{
		SubServerName: TAP,
		InitSubServer: initSubServer,
	}

	if err := subservers.RegisterSubServer(subServer); err != nil {
		panic(fmt.Sprintf("failed to register Taproot Assets sub "+
			"server driver: %v", err))
	}
}

func initSubServer(cfg *config.Config, logger btclog.Logger) (
	subservers.SubServer, error) {

	return newTaprootAssetsSubServer(
		cfg.Network, cfg.TaprootAssets, cfg.Remote.TaprootAssets,
		cfg.TaprootAssetsMode, cfg.LndRemote, logger,
	), nil
}

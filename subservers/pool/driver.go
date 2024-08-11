package pool

import (
	"github.com/btcsuite/btclog"
	"github.com/lightninglabs/lightning-terminal/config"
	"github.com/lightninglabs/lightning-terminal/subservers"
)

func RegisterSubServer(cfg *config.Config, _ btclog.Logger) (
	subservers.SubServer, error) {

	return newPoolSubServer(cfg.Pool, cfg.Remote.Pool, cfg.PoolMode), nil
}

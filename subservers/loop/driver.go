package loop

import (
	"fmt"

	"github.com/btcsuite/btclog"
	"github.com/lightninglabs/lightning-terminal/config"
	"github.com/lightninglabs/lightning-terminal/subservers"
)

func init() {
	subServer := &subservers.SubServerDriver{
		SubServerName: LOOP,
		InitSubServer: initSubServer,
	}

	if err := subservers.RegisterSubServer(subServer); err != nil {
		panic(fmt.Sprintf("failed to register Loop sub server "+
			"driver: %v", err))
	}
}

func initSubServer(cfg *config.Config, _ btclog.Logger) (subservers.SubServer,
	error) {

	return newLoopSubServer(
		cfg.Loop, cfg.Remote.Loop,
		cfg.LoopMode,
	), nil
}

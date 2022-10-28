package main

import (
	"context"

	terminal "github.com/lightninglabs/lightning-terminal"
	"github.com/lightninglabs/lightning-terminal/litrpc"
	"github.com/urfave/cli"
)

var statusCommands = []cli.Command{
	{
		Name:     "status",
		Usage:    "info about litd status",
		Category: "Status",
		Subcommands: []cli.Command{
			commandForSub(terminal.LoopSubServer),
			commandForSub(terminal.PoolSubServer),
			commandForSub(terminal.LNDSubServer),
			commandForSub(terminal.LitSubServer),
			commandForSub(terminal.FaradaySubServer),
		},
	},
}

func commandForSub(name string) cli.Command {
	return cli.Command{
		Name:   name,
		Action: getStatus(name),
	}
}

func getStatus(name string) func(ctx *cli.Context) error {
	return func(ctx *cli.Context) error {
		clientConn, cleanup, err := connectClient(ctx)
		if err != nil {
			return err
		}
		defer cleanup()
		client := litrpc.NewStatusClient(clientConn)

		ctxb := context.Background()
		resp, err := client.GetSubServerState(
			ctxb, &litrpc.GetSubServerStateReq{
				SubServerName: name,
			},
		)
		if err != nil {
			return err
		}

		printRespJSON(resp)

		return nil
	}
}

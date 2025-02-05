package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/algorandfoundation/did-algo/client/internal"
	protoV1 "github.com/algorandfoundation/did-algo/proto/did/v1"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"go.bryk.io/pkg/cli"
)

var walletWatchCmd = &cobra.Command{
	Use:     "watch",
	Short:   "Monitor your wallet's activity",
	Aliases: []string{"mon", "monitor"},
	Example: "algoid wallet watch [algo-address]",
	RunE:    runWalletWatchCmd,
	Long: `Monitor your wallet's activity.

You'll be able to receive near real-time notifications about
every transaction involving a given address.`,
}

func init() {
	params := []cli.Param{
		{
			Name:      "interval",
			Usage:     "Interval (in seconds) between activity checks",
			FlagKey:   "watch.interval",
			ByDefault: 5,
			Short:     "n",
		},
	}
	if err := cli.SetupCommandParams(walletWatchCmd, params, viper.GetViper()); err != nil {
		panic(err)
	}
	walletCmd.AddCommand(walletWatchCmd)
}

func runWalletWatchCmd(_ *cobra.Command, args []string) (err error) {
	// Get required parameters
	if len(args) != 1 {
		return errors.New("you must specify and address to monitor")
	}
	address := args[0]

	// Get client connection
	conf := new(internal.ClientSettings)
	if err := viper.UnmarshalKey("client", conf); err != nil {
		return err
	}
	if err := conf.Validate(); err != nil {
		return err
	}
	conn, err := getClientConnection(conf)
	if err != nil {
		return fmt.Errorf("failed to establish connection: %w", err)
	}
	defer func() {
		_ = conn.Close()
	}()
	cl := protoV1.NewAgentAPIClient(conn)

	// Open activity monitor
	log.Info("starting monitor (press 'q + Enter' to exit)...")
	ctx, halt := context.WithCancel(context.Background())
	defer halt()
	monitor, err := cl.AccountActivity(ctx, &protoV1.AccountActivityRequest{Address: address})
	if err != nil {
		return fmt.Errorf("failed to open account monitor: %w", err)
	}

	// Wait for user input in the background
	input := make(chan struct{})
	go func() {
		var signal string
		for {
			if _, _ = fmt.Scanln(&signal); signal == "q" {
				input <- struct{}{}
				close(input)
				return
			}
		}
	}()

	// Wait for monitor activity in the background
	go func() {
		for {
			record, err := monitor.Recv()
			if err != nil {
				log.Debug("closing monitor loop")
				return
			}
			data, _ := json.Marshal(record)
			fmt.Printf("%s\n", data)
		}
	}()

	// Wait for stop signals
	defer log.Info("monitor closed")
	for {
		select {
		case <-monitor.Context().Done():
			halt()
			return nil
		case <-input:
			halt()
			return nil
		}
	}
}

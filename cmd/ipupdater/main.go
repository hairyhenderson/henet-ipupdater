/*
The ipupdater command

*/
package main

import (
	"context"
	"net"
	"os"
	"os/signal"
	"time"

	ipupdater "github.com/hairyhenderson/henet-ipupdater"
	"github.com/hairyhenderson/henet-ipupdater/version"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
)

var (
	verbose  bool
	domain   string
	interval time.Duration
	key      string
	oneshot  bool
	ip       net.IP
)

func newCmd() *cobra.Command {
	rootCmd := &cobra.Command{
		Version: version.Version,
		Use:     "ipupdater",
		Short:   "A program that periodically updates dynamic IPs",
		RunE: func(cmd *cobra.Command, args []string) error {
			if verbose {
				zerolog.SetGlobalLevel(zerolog.DebugLevel)
			}
			log.Debug().
				Str("version", version.Version).
				Str("commit", version.GitCommit).
				Msg(cmd.CalledAs())
			cmd.SilenceErrors = true
			cmd.SilenceUsage = true

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			c := make(chan os.Signal, 1)
			signal.Notify(c, os.Interrupt)

			u := ipupdater.New(domain, key, ip)

			done := make(chan error)
			if oneshot {
				go func() {
					_, err := u.Update(ctx)
					done <- err
				}()
			} else {
				go func() {
					err := u.Loop(ctx, interval)
					done <- err
				}()
			}

			select {
			case err := <-done:
				return err
			case s := <-c:
				log.Debug().
					Str("signal", s.String()).
					Msg("shutting down gracefully...")
				return nil
			case <-ctx.Done():
				return ctx.Err()
			}
		},
	}
	return rootCmd
}

func initFlags(command *cobra.Command) {
	command.Flags().SortFlags = false

	command.Flags().StringVarP(&domain, "domain", "d", "", "domain to update the IP for")
	command.Flags().DurationVarP(&interval, "interval", "i", 5*time.Minute, "update interval")
	command.Flags().StringVarP(&key, "key", "k", "", "API Key")
	command.Flags().BoolVar(&oneshot, "oneshot", false, "update just once instead of looping")
	command.Flags().IPVar(&ip, "ip", nil, "force the IP address to this value (default: autodetected)")

	command.Flags().BoolVarP(&verbose, "verbose", "v", false, "Output extra logs")
}

func main() {
	initLogger()

	command := newCmd()
	initFlags(command)
	if err := command.Execute(); err != nil {
		log.Error().Err(err).Msg(command.Name() + " failed")
		os.Exit(1)
	}
}

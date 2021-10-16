/*
The ipupdater command

*/
package main

import (
	"context"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	ipupdater "github.com/hairyhenderson/henet-ipupdater"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	verbose       bool
	domain        string
	interval      time.Duration
	key           string
	oneshot       bool
	ip            net.IP
	enableMetrics bool
	endpoint      string
)

func run(ctx context.Context) error {
	ctx, stop := signal.NotifyContext(ctx, os.Interrupt, os.Kill, syscall.SIGTERM)
	defer stop()

	u := ipupdater.New(domain, key, endpoint, ip)

	if enableMetrics {
		http.Handle("/metrics", promhttp.Handler())

		go func() {
			_ = http.ListenAndServe(":8080", nil)
		}()
	}

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
	case <-ctx.Done():
		return ctx.Err()
	}
}

func main() {
	if err := mainrun(os.Args); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func mainrun(args []string) error {
	prog := args[0]

	fs := flag.NewFlagSet("root", flag.ContinueOnError)
	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage of %s:\nA program that periodically updates dynamic IPs\n", prog)
		fs.PrintDefaults()
	}
	fs.StringVar(&domain, "d", "", "domain to update the IP for")
	fs.DurationVar(&interval, "i", 5*time.Minute, "update interval")
	fs.StringVar(&key, "k", "", "API Key")
	fs.BoolVar(&oneshot, "1", false, "update just once instead of looping")
	fs.Var(&ipVar{ip}, "ip", "force the IP address to this value (default: autodetected)")
	fs.StringVar(&endpoint, "e", "https://dyn.dns.he.net/nic/update",
		"the dynamic DNS URL to communicate with (only HE.net supported for now)")

	fs.BoolVar(&enableMetrics, "m", true, "enable Prometheus metrics")

	fs.BoolVar(&verbose, "d", false, "Output extra logs")

	err := fs.Parse(args[1:])
	if err != nil {
		return err
	}

	err = run(context.Background())
	if err != nil {
		return fmt.Errorf("%s failed: %w", prog, err)
	}

	return nil
}

type ipVar struct {
	net.IP
}

// String implements flag.Value.
func (c ipVar) String() string {
	return c.IP.String()
}

// Set implements flag.Value.
func (c *ipVar) Set(s string) error {
	c.IP = net.ParseIP(s)

	return nil
}

package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	log "github.com/sirupsen/logrus"
	"github.com/urfave/cli/v3"
)

func main() {
	appName := filepath.Base(os.Args[0])

	err := (&cli.Command{
		Name:    appName,
		Authors: []any{"Slawomir Caluch"},
		Action:  runGollamas,
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "listen",
				Value:   "localhost:11434",
				Usage:   `address on which the router will be listening on, ie: "localhost:11434"`,
				Aliases: []string{"a", "addr", "address"},
				Sources: cli.EnvVars("GOLLAMAS_LISTEN", "LISTEN"),
			},
			&cli.StringFlag{
				Name:  "level",
				Value: log.ErrorLevel.String(),
				Usage: fmt.Sprintf("error level, can be any of %s",
					strings.Join([]string{
						log.PanicLevel.String(),
						log.FatalLevel.String(),
						log.ErrorLevel.String(),
						log.WarnLevel.String(),
						log.InfoLevel.String(),
						log.DebugLevel.String(),
						log.TraceLevel.String(),
					}, "|"),
				),
				Aliases: []string{"error-level"},
				Sources: cli.EnvVars("GOLLAMAS_LEVEL", "LEVEL"),
			},
			&cli.StringSliceFlag{
				Name:      "proxy",
				Usage:     `defines a proxy for a given model ex: --proxy 'llama3.2-vision=http://server:11434'`,
				Validator: validateProxies,
			},
			&cli.StringFlag{
				Name:      "proxies",
				Validator: validateCSProxies,
				Usage:     `defines a list of proxies per model given model ex: --proxies 'llama3.2-vision=http://server:11434,deepseek-r1:14b=http://server2:11434'`,
				Sources:   cli.EnvVars("GOLLAMAS_PROXIES", "PROXIES"),
			},
		},
		Commands: []*cli.Command{
			getVersionCommand(),
		},
	}).Run(context.Background(), os.Args)
	if err != nil {
		log.Fatalf("%s ended with error: %s", appName, err.Error())
	}
}

func initErrorLevel(e string) error {
	l, err := log.ParseLevel(strings.ToLower(e))
	if err != nil {
		return err
	}
	log.SetLevel(l)
	return nil
}

func initProxyConfig(ss []string) (map[string]ProxyConfig, error) {
	res := map[string]ProxyConfig{}
	for _, s := range ss {
		v := strings.SplitN(s, "=", 2)
		if len(v) != 2 {
			return nil, fmt.Errorf("invalid proxy string %s, %d", s, len(v))
		}
		if v[0] == "" {
			return nil, fmt.Errorf("invalid proxy model in %s", s)
		}
		if v[1] == "" {
			return nil, fmt.Errorf("invalid proxy destination in %s", s)
		}

		res[v[0]] = ProxyConfig{
			url: v[1],
		}
	}
	return res, nil
}

func runGollamas(ctx context.Context, cli *cli.Command) error {
	if err := initErrorLevel(cli.String("level")); err != nil {
		return err
	}
	log.Tracef("starting")
	defer log.Tracef("ending")

	p := append([]string{}, cli.StringSlice("proxy")...)
	p = append(p, strings.Split(cli.String("proxies"), ",")...)
	pConf, err := initProxyConfig(p)
	if err != nil {
		return err
	}

	s, err := NewService(ctx, pConf)
	if err != nil {
		return err
	}

	rs := s.GenerateRoutes()
	addr := cli.String("address")

	log.Printf("Starting server on %s", addr)
	return http.ListenAndServe(addr, rs)
}

func validateProxies(s []string) error {
	_, err := initProxyConfig(s)
	return err
}

func validateCSProxies(csl string) error {
	s := strings.Split(csl, ",")
	_, err := initProxyConfig(s)
	return err
}

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
		Action:  runGollamasCli,
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
				Name:  "proxy",
				Usage: `defines a proxy for a given model ex: --proxy 'llama3.2-vision=http://server:11434'`,
			},
			&cli.StringFlag{
				Name:    "proxies",
				Usage:   `defines a list of proxies per model given model ex: --proxies 'llama3.2-vision=http://server:11434,deepseek-r1:14b=http://server2:11434'`,
				Sources: cli.EnvVars("GOLLAMAS_PROXIES", "PROXIES"),
			},
			&cli.StringSliceFlag{
				Name:  "alias",
				Usage: `assigns an alias from an existing model name passed in the proxy configuration 'alias=concrete_model' ex: --alias gpt-3.5-turbo=llama3.2`,
			},
			&cli.StringFlag{
				Name:    "aliases",
				Usage:   `sets aliases for the given model names ex: --aliases 'gpt-3.5-turbo=llama3.2,deepseek=deepseek-r1:14b'`,
				Sources: cli.EnvVars("GOLLAMAS_ALIASES", "ALIASES"),
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
	log.WithField("strings", ss).Trace("Initialize proxy configuration.")
	for _, s := range ss {
		v := strings.SplitN(s, "=", 2)
		if len(v) != 2 {
			return nil, fmt.Errorf("invalid proxy string: %s", s)
		}
		if v[0] == "" {
			return nil, fmt.Errorf("empty proxy model in %s", s)
		}
		if v[1] == "" {
			return nil, fmt.Errorf("empty proxy destination in %s", s)
		}

		res[v[0]] = ProxyConfig{
			Url: v[1],
		}
	}
	return res, nil
}

func initAliasesMap(ss []string) (map[string]string, error) {
	aliases := map[string]string{}
	log.WithField("aliases", aliases).Trace("Initialize aliases.")
	for _, s := range ss {
		v := strings.SplitN(s, "=", 2)
		if len(v) != 2 {
			return nil, fmt.Errorf("invalid alias string: %s", s)
		}
		if v[0] == "" {
			return nil, fmt.Errorf("empty alias name in: %s", s)
		}
		if v[1] == "" {
			return nil, fmt.Errorf("empty alias model in: %s", s)
		}
		aliases[v[0]] = v[1]
	}
	return aliases, nil
}

func runGollamasCli(ctx context.Context, cli *cli.Command) error {
	if err := initErrorLevel(cli.String("level")); err != nil {
		return err
	}
	log.Tracef("Application is starting.")
	defer log.Tracef("Application has ended.")

	p := append([]string{}, cli.StringSlice("alias")...)
	if cli.String("aliases") != "" {
		p = append(p, strings.Split(cli.String("aliases"), ",")...)
	}
	aliases, err := initAliasesMap(p)
	if err != nil {
		return err
	}

	p = append([]string{}, cli.StringSlice("proxy")...)
	if cli.String("proxies") != "" {
		p = append(p, strings.Split(cli.String("proxies"), ",")...)
	}
	pConf, err := initProxyConfig(p)
	if err != nil {
		return err
	}
	return runGollamas(GollamasConfig{
		Listen:  cli.String("listen"),
		Proxies: pConf,
		Aliases: aliases,
	})
}

type GollamasConfig struct {
	Listen  string
	Proxies map[string]ProxyConfig
	Aliases map[string]string
}

func InitService(cfg GollamasConfig) (*Service, error) {
	cmap, err := initClients(cfg.Proxies)
	if err != nil {
		return nil, err
	}

	ropts := initRouterAliasOpts(cfg.Aliases)

	r, err := NewRouter(cmap, ropts...)
	if err != nil {
		return nil, err
	}

	return NewService(r)
}

func runGollamas(cfg GollamasConfig) error {
	s, err := InitService(cfg)
	if err != nil {
		return err
	}

	rs := GenerateRoutes(s)
	addr := cfg.Listen

	log.Printf("Starting server on %s", addr)
	return http.ListenAndServe(addr, rs)
}

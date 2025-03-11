package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	log "github.com/sirupsen/logrus"
	"github.com/urfave/cli/v3"
	"golang.org/x/term"
)

func main() {
	appName := filepath.Base(os.Args[0])
	cli.HelpPrinter = func(w io.Writer, templ string, data interface{}) {
		funcMap := map[string]interface{}{
			"wrapAt": func() int {
				w := 10000
				tid := int(os.Stdout.Fd())
				if term.IsTerminal(tid) {
					width, _, err := term.GetSize(tid)
					if err == nil {
						w = width - 1
					}
				}
				return w
			},
		}

		cli.HelpPrinterCustom(w, templ, data, funcMap)
	}
	err := (&cli.Command{
		Name:    appName,
		Authors: []any{"Slawomir Caluch"},
		Action:  runGollamasCli,
		Usage:   "A router for golama models",
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
				Usage: fmt.Sprintf(`error level, can be any of %s`,
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
				Usage: `assigns a destination for a model, can be a url or a connection id. ex: --proxy 'llama3.2-vision=http://server:11434' ex: --proxy 'llama3.2-vision=c1 --connection c1=http://server:11434'`,
			},
			&cli.StringFlag{
				Name:    "proxies",
				Usage:   `assigns destinations for the models, in the list of model=destination pairs. ex: --proxies 'llama3.2-vision=http://server:11434,deepseek-r1:14b=http://server2:11434'`,
				Sources: cli.EnvVars("GOLLAMAS_PROXIES", "PROXIES"),
			},
			&cli.StringSliceFlag{
				Name:  "connection",
				Usage: `assigns an identifier to a connection which can be reffered to by proxy declarations ex: --connection c1=http://server:11434 --proxy llama=c1`,
			},
			&cli.StringFlag{
				Name:    "connections",
				Usage:   `provides a list of connections which can be reffered to by id. ex: --connections c1=http://server:11434,c2=http://server2:11434`,
				Sources: cli.EnvVars("GOLLAMAS_CONNECTIONS", "CONNECTIONS"),
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
			&cli.BoolFlag{
				Name:    "list-aliases",
				Usage:   `exposes aliases in the router`,
				Sources: cli.EnvVars("GOLLAMAS_LIST_ALIASES", "LIST_ALIASES"),
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

func initConnectionsConfig(ss []string) (map[string]ConnectionConfig, error) {
	res := map[string]ConnectionConfig{}
	log.WithField("strings", ss).Trace("Initialize connection map.")
	for _, s := range ss {
		v := strings.SplitN(s, "=", 2)
		if len(v) != 2 {
			return nil, fmt.Errorf("invalid connection string: %s", s)
		}
		if v[0] == "" {
			return nil, fmt.Errorf("empty connection id in %s", s)
		}
		if v[1] == "" {
			return nil, fmt.Errorf("empty connection destination in %s", s)
		}

		res[v[0]] = ConnectionConfig{
			ConnectionID: v[0],
			Url:          v[1],
		}
	}
	return res, nil
}

func initProxyConfig(ss []string) (map[string]ModelConfig, error) {
	res := map[string]ModelConfig{}
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

		res[v[0]] = ModelConfig{
			ConnectionID: v[1],
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

	cfg, err := getGollamasConfig(cli)
	if err != nil {
		return fmt.Errorf("could not initialize gollamas config: %w", err)
	}
	return runGollamas(*cfg)
}

func getAliasesMap(cli *cli.Command) (map[string]string, error) {
	p := append([]string{}, cli.StringSlice("alias")...)
	if cli.String("aliases") != "" {
		p = append(p, strings.Split(cli.String("aliases"), ",")...)
	}
	return initAliasesMap(p)
}

func getConnectionsConfig(cli *cli.Command) (map[string]ConnectionConfig, error) {
	p := append([]string{}, cli.StringSlice("connection")...)
	if cli.String("connections") != "" {
		p = append(p, strings.Split(cli.String("connections"), ",")...)
	}
	return initConnectionsConfig(p)
}

func getProxyConfig(cli *cli.Command) (map[string]ModelConfig, error) {
	p := append([]string{}, cli.StringSlice("proxy")...)
	if cli.String("proxies") != "" {
		p = append(p, strings.Split(cli.String("proxies"), ",")...)
	}
	return initProxyConfig(p)
}

func getGollamasConfig(cli *cli.Command) (*GollamasConfig, error) {
	aliases, err := getAliasesMap(cli)
	if err != nil {
		return nil, err
	}
	cmap, err := getConnectionsConfig(cli)
	if err != nil {
		return nil, err
	}
	pConf, err := getProxyConfig(cli)
	if err != nil {
		return nil, err
	}
	return &GollamasConfig{
		Listen:      cli.String("listen"),
		Models:      pConf,
		Aliases:     aliases,
		ListAliases: cli.Bool("list-aliases"),
		Connections: cmap,
	}, nil
}

type GollamasConfig struct {
	Listen      string
	Connections map[string]ConnectionConfig
	Models      map[string]ModelConfig
	Aliases     map[string]string
	ListAliases bool
}

func InitService(cfg GollamasConfig) (*Service, error) {
	cconf, pconf, err := reconcileConnectionsAndProxyConfigs(cfg.Connections, cfg.Models)
	if err != nil {
		return nil, err
	}

	cmap, err := initClients(cconf)
	if err != nil {
		return nil, err
	}

	ropts := initRouterAliasOpts(cfg.Aliases)
	ropts = append(ropts, WithExposeAliases(cfg.ListAliases))

	r, err := NewRouter(cmap, pconf, ropts...)
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

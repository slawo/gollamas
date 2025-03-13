package main

import (
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func mockRunGollamas(m *mock.Mock, cfg GollamasConfig) error {
	args := m.Called(cfg)
	return args.Error(0)
}
func prepareTestOsArgs(t *testing.T, args ...string) *mock.Mock {
	saveRunGollamas := runGollamas
	m := mock.Mock{}
	runGollamas = func(cfg GollamasConfig) error {
		return mockRunGollamas(&m, cfg)
	}
	t.Cleanup(func() {
		runGollamas = saveRunGollamas
	})

	saveArgs := os.Args
	t.Cleanup(func() {
		os.Args = saveArgs
	})
	os.Args = args

	return &m
}

func TestRunCliConfig(t *testing.T) {
	for name, tt := range map[string]struct {
		args   []string
		config *GollamasConfig
		err    error
	}{
		"InvalidErrorLevel": {
			args: []string{"gollamas", "--level", "invalid"},
			err:  fmt.Errorf("not a valid logrus Level: \"invalid\""),
		},
		"DefaultListen": {
			args: []string{"gollamas"},
			config: &GollamasConfig{
				Listen:      "localhost:11434",
				Connections: map[string]ConnectionConfig{},
				Models:      map[string]ModelConfig{},
				Aliases:     map[string]string{},
				ListAliases: false,
			},
		},
		"WithListen": {
			args: []string{"gollamas", "--listen", "0.0.0.0:11434"},
			config: &GollamasConfig{
				Listen:      "0.0.0.0:11434",
				Connections: map[string]ConnectionConfig{},
				Models:      map[string]ModelConfig{},
				Aliases:     map[string]string{},
				ListAliases: false,
			},
		},
		"WithEmptyProxy": {
			args: []string{
				"gollamas", "--listen", "0.0.0.0:11434",
				"--proxy", "",
			},
			err: fmt.Errorf("could not initialize gollamas config: empty proxy string"),
		},
		"WithInvalidProxy": {
			args: []string{
				"gollamas", "--listen", "0.0.0.0:11434",
				"--proxy", "model1",
			},
			err: fmt.Errorf("could not initialize gollamas config: invalid proxy string: model1"),
		},
		"WithEmptyProxyName": {
			args: []string{
				"gollamas", "--listen", "0.0.0.0:11434",
				"--proxy", "=http://server1",
			},
			err: fmt.Errorf("could not initialize gollamas config: empty proxy model in =http://server1"),
		},
		"WithEmptyProxyConnection": {
			args: []string{
				"gollamas", "--listen", "0.0.0.0:11434",
				"--proxy", "model1=",
			},
			err: fmt.Errorf("could not initialize gollamas config: empty proxy destination in model1="),
		},
		"WithMultipleProxy": {
			args: []string{
				"gollamas", "--listen", "0.0.0.0:11434",
				"--proxy", "model1=http://server1", "--proxy", "model2=http://server2",
			},
			config: &GollamasConfig{
				Listen:      "0.0.0.0:11434",
				Connections: map[string]ConnectionConfig{},
				Models: map[string]ModelConfig{
					"model1": {ConnectionID: "http://server1"},
					"model2": {ConnectionID: "http://server2"},
				},
				Aliases:     map[string]string{},
				ListAliases: false,
			},
		},
		"WithProxies": {
			args: []string{
				"gollamas", "--listen", "0.0.0.0:11434",
				"--proxies", "model1=http://server1,model2=http://server2",
			},
			config: &GollamasConfig{
				Listen:      "0.0.0.0:11434",
				Connections: map[string]ConnectionConfig{},
				Models: map[string]ModelConfig{
					"model1": {ConnectionID: "http://server1"},
					"model2": {ConnectionID: "http://server2"},
				},
				Aliases:     map[string]string{},
				ListAliases: false,
			},
		},
		"WithEmptyAlias": {
			args: []string{
				"gollamas", "--listen", "0.0.0.0:11434",
				"--alias", "",
			},
			err: fmt.Errorf("could not initialize gollamas config: empty alias string"),
		},
		"WithInvalidAlias": {
			args: []string{
				"gollamas", "--listen", "0.0.0.0:11434",
				"--alias", "alias1",
			},
			err: fmt.Errorf("could not initialize gollamas config: invalid alias string: alias1"),
		},
		"WithEmptyAliasName": {
			args: []string{
				"gollamas", "--listen", "0.0.0.0:11434",
				"--alias", "=model1",
			},
			err: fmt.Errorf("could not initialize gollamas config: empty alias name in: =model1"),
		},
		"WithEmptyAliasModel": {
			args: []string{
				"gollamas", "--listen", "0.0.0.0:11434",
				"--alias", "alias1=",
			},
			err: fmt.Errorf("could not initialize gollamas config: empty alias model in: alias1="),
		},
		"WithMultipleAlias": {
			args: []string{
				"gollamas", "--listen", "0.0.0.0:11434",
				"--alias", "alias1=model1", "--alias", "alias2=model2",
			},
			config: &GollamasConfig{
				Listen:      "0.0.0.0:11434",
				Connections: map[string]ConnectionConfig{},
				Models:      map[string]ModelConfig{},
				Aliases:     map[string]string{"alias1": "model1", "alias2": "model2"},
				ListAliases: false,
			},
		},
		"WithAliases": {
			args: []string{
				"gollamas", "--listen", "0.0.0.0:11434",
				"--aliases", "alias1=model1,alias2=model2",
			},
			config: &GollamasConfig{
				Listen:      "0.0.0.0:11434",
				Connections: map[string]ConnectionConfig{},
				Models:      map[string]ModelConfig{},
				Aliases:     map[string]string{"alias1": "model1", "alias2": "model2"},
				ListAliases: false,
			},
		},
		"WithEmptyConnection": {
			args: []string{
				"gollamas", "--listen", "0.0.0.0:11434",
				"--connection", "",
			},
			err: fmt.Errorf("could not initialize gollamas config: empty connection string"),
		},
		"WithInvalidConnection": {
			args: []string{
				"gollamas", "--listen", "0.0.0.0:11434",
				"--connection", "c1",
			},
			err: fmt.Errorf("could not initialize gollamas config: invalid connection string: c1"),
		},
		"WithEmptyConnectionName": {
			args: []string{
				"gollamas", "--listen", "0.0.0.0:11434",
				"--connection", "=http://server1",
			},
			err: fmt.Errorf("could not initialize gollamas config: empty connection id in =http://server1"),
		},
		"WithEmptyConnectionUrl": {
			args: []string{
				"gollamas", "--listen", "0.0.0.0:11434",
				"--connection", "c1=",
			},
			err: fmt.Errorf("could not initialize gollamas config: empty connection destination in c1="),
		},
		"WithMultipleConnection": {
			args: []string{
				"gollamas", "--listen", "0.0.0.0:11434",
				"--connection", "c1=http://server1", "--connection", "c2=http://server2",
			},
			config: &GollamasConfig{
				Listen: "0.0.0.0:11434",
				Connections: map[string]ConnectionConfig{
					"c1": {Url: "http://server1", ConnectionID: "c1"},
					"c2": {Url: "http://server2", ConnectionID: "c2"},
				},
				Models:      map[string]ModelConfig{},
				Aliases:     map[string]string{},
				ListAliases: false,
			},
		},
		"WithConnections": {
			args: []string{
				"gollamas", "--listen", "0.0.0.0:11434",
				"--connections", "c1=http://server1,c2=http://server2",
			},
			config: &GollamasConfig{
				Listen: "0.0.0.0:11434",
				Connections: map[string]ConnectionConfig{
					"c1": {Url: "http://server1", ConnectionID: "c1"},
					"c2": {Url: "http://server2", ConnectionID: "c2"},
				},
				Models:      map[string]ModelConfig{},
				Aliases:     map[string]string{},
				ListAliases: false,
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			m := prepareTestOsArgs(t, tt.args...)
			if tt.config != nil {
				m.On("mockRunGollamas", *tt.config).Return(tt.err)
			}
			err := runCli("gollamas")
			if tt.err == nil {
				assert.NoError(t, err)
			} else {
				assert.EqualError(t, err, tt.err.Error())
			}
			m.AssertExpectations(t)
		})
	}
}

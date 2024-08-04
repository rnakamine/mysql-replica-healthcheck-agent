package config

import (
	"os"

	"gopkg.in/yaml.v2"
)

type ReplicaConfig struct {
	Host                   string            `yaml:"host"`
	Port                   int               `yaml:"port"`
	User                   string            `yaml:"user"`
	Password               string            `yaml:"password"`
	MaxSecondsBehindSource int               `yaml:"max_seconds_behind_source"`
	FailReplicaNotRunning  bool              `yaml:"fail_replica_not_running"`
	HealthcheckConfig      HealthcheckConfig `yaml:"healthcheck_config"`
}

type HealthcheckConfig struct {
	Path string `yaml:"path"`
	Port int    `yaml:"port"`
}

type Configs map[string]ReplicaConfig

func New(filePath string) (*Configs, error) {
	// #nosec G304
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	configs := &Configs{}
	err = yaml.Unmarshal(data, configs)
	if err != nil {
		return nil, err
	}

	return configs, nil
}

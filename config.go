package main

import (
	"os"

	"gopkg.in/yaml.v2"
)

type ReplicaConfig struct {
	Host                   string `yaml:"host"`
	Port                   int    `yaml:"port"`
	User                   string `yaml:"user"`
	Password               string `yaml:"password"`
	MaxSecondsBehindMaster int    `yaml:"max-seconds-behind-master"`
	FailSlaveNotRunning    bool   `yaml:"fail-slave-not-running"`
}

type Configs map[string]ReplicaConfig

func newConfig(filePath string) (*Configs, error) {
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

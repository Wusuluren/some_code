package config

import (
	"gopkg.in/yaml.v3"
	"io/ioutil"
)

func InitYamlConfig(file string) (cfg Config, err error) {
	fileBody, err := ioutil.ReadFile(file)
	if err != nil {
		return
	}
	err = yaml.Unmarshal(fileBody, &cfg)
	if err != nil {
		return
	}
	return
}

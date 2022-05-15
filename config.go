package main

import (
	"io/ioutil"
	"gopkg.in/yaml.v3"
)

func (config *DeviceConfig) ReadConfig(configFileName string) (error) {
    source, err := ioutil.ReadFile(configFileName)
    
	if err != nil {
        return err
    }
    
	err = yaml.Unmarshal(source, config)
    
	if err != nil {
        return err
    }

	return nil
}

func (config *DeviceConfig) WriteConfig(configFileName string) (error) {
    var out []byte
	var err error
	
	out, err = yaml.Marshal(config)
    
	if err != nil {
        return err
    }

	err = ioutil.WriteFile(configFileName, out, 0666)

	return err
}


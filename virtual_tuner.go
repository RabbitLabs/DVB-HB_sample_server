package main

type VirtualTunerConfig struct {
	Tune        string                `yaml:"name"`
	ChannelMaps map[string]ChannelMap `yaml:"channelmaps"`
}
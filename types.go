package main

type Channel struct {
	Name   string `yaml:"name"`
	Type   string `yaml:"type"`
	Source string `yaml:"source"`
}

type ChannelMap struct {
	Description string          `yaml:"name"`
	Provider    string          `yaml:"provider"`
	ProviderURL string          `yaml:"providerurl"`
	Channels    map[int]Channel `yaml:"channels"`
}

type DeviceConfig struct {
	Name        string                `yaml:"name"`
	ChannelMaps map[string]ChannelMap `yaml:"channelmaps"`
}

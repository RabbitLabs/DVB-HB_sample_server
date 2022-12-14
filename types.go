package main

import (
	"net/http"

	"github.com/Comcast/gots/packet"
)

// types for channel map
type Channel struct {
	Name    string `yaml:"name"`
	Dynamic bool   `yaml:"dynamic"`
	Source  string `yaml:"source"`
	Tune    string `yaml:"tune"`
	Demux   string `yaml:"demux"`
}

type ChannelMap struct {
	Description string          `yaml:"name"`
	Provider    string          `yaml:"provider"`
	ProviderURL string          `yaml:"providerurl"`
	Channels    map[int]Channel `yaml:"channels"`
}

type DynamicChannelMap interface {
	GetChannelInfo() ChannelMap
	GetChannelMap() ChannelMap
}

type DynamicContent interface {
	ServeDynamicContent(w http.ResponseWriter, r *http.Request, path string)
}

type DeviceConfig struct {
	Name               string                `yaml:"name"`
	ChannelMaps        map[string]ChannelMap `yaml:"channelmaps"`
	TunerConfig        CommandLineToolConfig `yaml:"tunerconfig"`
	Feeds              map[string]string     `yaml:"feeds"`
	Aliases            map[string]string     `yaml:"aliases"`
	TranscodeConfig    CommandLineToolConfig `yaml:"transcodeconfig"`
	MaxTuner           int                   `yaml:"maxtuner"`
	TunerList          []int                 `yaml:"tunerlist"`
	OpenPage           bool                  `yaml:"openpage"`
	ServerPort         int                   `yaml:"serverport"`
	HelperTools        []CommandLineToolConfig `yaml:"helpertools"`
	dynamicchannelmaps map[string]DynamicChannelMap
	dynamiccontent     map[string]DynamicContent
	helpertoolsruntime []*CommandLineTool
}

// transcoding
type Transcoder interface {
	Start(outputdir string)
	ProcessPacket(packet.Packet)
	Stop()
}

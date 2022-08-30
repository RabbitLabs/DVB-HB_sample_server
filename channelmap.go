package main

import (
	"fmt"
	"net/http"
	"strings"
)

const ChannelMapPath = "/channelmap/"

func RegisterDynamicChannelMap(m DynamicChannelMap) {
	c := m.GetChannelInfo()

	deviceconfig.dynamicchannelmaps[c.Provider] = m
}

func channelmapHandler(w http.ResponseWriter, r *http.Request) {
	if !strings.HasPrefix(r.URL.Path, ChannelMapPath) {
		http.Error(w, "404 not found.", http.StatusNotFound)
		return
	}

	if r.Method != "GET" {
		http.Error(w, "Method is not supported.", http.StatusNotFound)
		return
	}
	subpath := r.URL.Path[len(ChannelMapPath):]

	subpath = strings.TrimLeft(subpath, "/")

	w.Header().Set("Content-Type", "application/xml; charset=utf-8")

	// not extension, just return list of services
	if subpath == "serviceslist.xml" {
		deviceconfig.channelmapListWrite(w, r.Host)
		return
	}

	splitpath := strings.SplitN(subpath, "/", 2)

	// we must have two part path
	if len(splitpath) != 2 {
		http.Error(w, "404 not found.", http.StatusNotFound)
		return
	}

	// try to find channel map
	channelmap, exists := deviceconfig.ChannelMaps[splitpath[0]]

	// check if channel map exists
	if !exists {
		// try to find channel map
		dynamicchannelmap, exists := deviceconfig.dynamicchannelmaps[splitpath[0]]

		if !exists {
			http.Error(w, "404 not found.", http.StatusNotFound)
			return
		}

		channelmap = dynamicchannelmap.GetChannelMap()
	}

	switch splitpath[1] {
	case "serviceslist.xml":
		channelmap.channelMapWrite(w, r.Host, splitpath[0])
	default:
		http.Error(w, "404 not found.", http.StatusNotFound)
	}
}

func (channelmap *ChannelMap) WriteConfig(w http.ResponseWriter, host string, name string) {
	w.Write([]byte("<sld:ProviderOffering>\n"))
	w.Write([]byte("<sld:Provider>\n"))
	w.Write([]byte("<sld:Name>"))
	w.Write([]byte(channelmap.Provider))
	w.Write([]byte("</sld:Name>\n"))
	w.Write([]byte("</sld:Provider>\n"))

	//
	w.Write([]byte("<sld:ServiceListOffering>\n"))
	w.Write([]byte("<sld:ServiceListName>"))
	w.Write([]byte(name))
	w.Write([]byte("</sld:ServiceListName>\n"))
	w.Write([]byte("<sld:ServiceListURI contentType=\"application/xml\">\n"))
	w.Write([]byte("<dvbisd:URI>\n"))
	fmt.Fprintf(w, "http://%s%s%s/serviceslist.xml", host, ChannelMapPath, name)
	w.Write([]byte("</dvbisd:URI>\n"))
	w.Write([]byte("</sld:ServiceListURI>\n"))
	w.Write([]byte("<sld:TargetCountry>DEU</sld:TargetCountry>\n"))
	w.Write([]byte("</sld:ServiceListOffering>\n"))

	w.Write([]byte("</sld:ProviderOffering>\n"))
}

func (config DeviceConfig) channelmapListWrite(w http.ResponseWriter, host string) {
	w.Write([]byte("<?xml version=\"1.0\" encoding=\"UTF-8\"?>"))
	w.Write([]byte("<sld:ServiceListEntryPoints xmlns:sld=\"urn:dvb:metadata:servicelistdiscovery:2019\" xmlns:dvbisd=\"urn:dvb:metadata:servicediscovery:2019\" xmlns:mpeg7=\"urn:tva:mpeg7:2008\" xmlns:xsi=\"http://www.w3.org/2001/XMLSchema-instance\" xsi:schemaLocation=\"urn:dvb:metadata:servicelistdiscovery:2019 dvbi_service_list_discovery_v1.0.xsd\">"))
	w.Write([]byte("<sld:ServiceListRegistryEntity regulatorFlag=\"false\">\n"))
	w.Write([]byte("</sld:ServiceListRegistryEntity>\n"))

	// list all static channels maps
	for name, channelmap := range config.ChannelMaps {
		// provider header
		channelmap.WriteConfig(w, host, name)
	}

	// list all dynamic channel maps
	for name, dynamicchannelmap := range config.dynamicchannelmaps {
		// get quick description of channel map
		channelmap := dynamicchannelmap.GetChannelInfo()

		channelmap.WriteConfig(w, host, name)
	}

	w.Write([]byte("</sld:ServiceListEntryPoints>\n"))
}

func (channelmap ChannelMap) GenerateServiceRef(channel Channel) string {
	return "tag:" + channelmap.Provider + ",2022:" + strings.ReplaceAll(strings.ToLower(channel.Name), " ", "_")
}

func (channelmap ChannelMap) channelMapWrite(w http.ResponseWriter, host string, name string) {
	w.Write([]byte("<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n"))
	w.Write([]byte("<ServiceList xmlns=\"urn:dvb:metadata:servicediscovery:2019\" xmlns:xsi=\"http://www.w3.org/2001/XMLSchema-instance\" xmlns:tva=\"urn:tva:metadata:2019\" version=\"1\" xsi:schemaLocation=\"urn:dvb:metadata:servicediscovery:2019 ../dvbi_v1.0.xsd\">\n"))

	fmt.Fprintf(w, "<Name>%s</Name>\n", name)
	fmt.Fprintf(w, "<ProviderName>%s</ProviderName>\n", channelmap.Provider)

	// LCN table
	w.Write([]byte("<LCNTableList>\n<LCNTable>\n"))

	for number, channel := range channelmap.Channels {
		fmt.Fprintf(w, "<LCN channelNumber=\"%d\" serviceRef=\"%s\"/>\n", number, channelmap.GenerateServiceRef(channel))
	}

	w.Write([]byte("</LCNTable>\n</LCNTableList>\n"))

	for _, channel := range channelmap.Channels {
		w.Write([]byte("<Service version=\"1\">\n"))
		fmt.Fprintf(w, "<UniqueIdentifier>%s</UniqueIdentifier>\n", channelmap.GenerateServiceRef(channel))
		w.Write([]byte("<ServiceInstance priority=\"1\">\n"))
		w.Write([]byte("<SourceType>urn:dvb:metadata:source:dvb-dash</SourceType>\n"))
		w.Write([]byte("<DASHDeliveryParameters>\n"))
		w.Write([]byte("<UriBasedLocation contentType=\"application/dash+xml\">\n"))
		fmt.Fprintf(w, "<URI>http://%s/%s</URI>\n", host, channel.Source)
		w.Write([]byte("</UriBasedLocation>\n"))
		w.Write([]byte("</DASHDeliveryParameters>\n"))
		w.Write([]byte("</ServiceInstance>\n"))
		fmt.Fprintf(w, "<ServiceName>%s</ServiceName>\n", channel.Name)
		fmt.Fprintf(w, "<ProviderName>%s</ProviderName>\n", channelmap.Provider)
		w.Write([]byte("</Service>\n"))
	}

	w.Write([]byte("</ServiceList>"))
}

package main

import (
	"crypto/sha256"
	_ "embed"
	"fmt"
	"log"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/koron/go-ssdp"
)

var rootadvertiser *ssdp.Advertiser

//var hbadvertiser *ssdp.Advertiser
var adticker *time.Ticker
var deviceuuid string

const defaultuuid = "uuid:11e77140-70dc-4d30-80dd-c6ddae09bd41"

// integrate icon file
//go:embed icon.png
var icondata []byte

const SERVERDESCPATH = "/server.xml"
const ICONPATH = "/icon.png"
const SERVERSTRING = "DVB-HB Sample Server 1.0"

// this trick is to get local IP
func upnp_getlocal_address() string {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()

	addr := conn.LocalAddr().String()

	return strings.Split(addr, ":")[0]
}

// generate a unique id from MAC address
func upnp_generate_uuid() string {
	interfaces, err := net.Interfaces()
	if err != nil {
		// return fake static uuid
		return defaultuuid
	}

	// scan interaces
	for _, i := range interfaces {
		// keep only up and non loopback
		if i.Flags&net.FlagUp != 0 && i.Flags&net.FlagLoopback == 0 {
			// Skip locally administered addresses
			if i.HardwareAddr[0]&2 == 2 || i.HardwareAddr[0] == 0 {
				continue
			}

			// hash MAC address with SHA 256
			h := sha256.New()
			h.Write(i.HardwareAddr)
			hashedMAC := h.Sum(nil)

			log.Printf("Generate UUID from MAC address of %s\n", i.Name)

			var result strings.Builder
			// format part of hash as uuid
			fmt.Fprintf(&result, "uuid:%02x%02x%02x%02x-", hashedMAC[0], hashedMAC[1], hashedMAC[2], hashedMAC[3])
			fmt.Fprintf(&result, "%02x%02x-", hashedMAC[4], hashedMAC[5])
			fmt.Fprintf(&result, "%02x%02x-", hashedMAC[6], hashedMAC[7])
			fmt.Fprintf(&result, "%02x%02x-", hashedMAC[8], hashedMAC[9])
			fmt.Fprintf(&result, "%02x%02X%02X%02X%02X%02X", hashedMAC[10], hashedMAC[11], hashedMAC[12], hashedMAC[13], hashedMAC[14], hashedMAC[15])

			log.Println(result.String())

			return result.String()
		}
	}

	return defaultuuid
}

func UPNPDeviceDescHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/xml")

	log.Printf("Request UPNP root description from %s", r.RemoteAddr)

	w.Write([]byte("<?xml version=\"1.0\"?>\r\n"))
	w.Write([]byte("<root xmlns=\"urn:schemas-upnp-org:device-1-0\" configId=\"0\">\r\n"))
	w.Write([]byte("<specVersion>\r\n"))
	w.Write([]byte("<major>1</major>\r\n"))
	w.Write([]byte("<minor>1</minor>\r\n"))
	w.Write([]byte("</specVersion>\r\n"))
	w.Write([]byte("<device>\r\n"))
	w.Write([]byte("<deviceType>urn:ses-com:device:SatIPServer:1</deviceType>\r\n"))
	w.Write([]byte("<friendlyName>Home Broadcast Server</friendlyName>\r\n"))
	w.Write([]byte("<manufacturer>DVB</manufacturer>\r\n"))
	w.Write([]byte("<manufacturerURL>http://dvb.org</manufacturerURL>\r\n"))
	w.Write([]byte("<modelDescription>Sample Home Broadcasting Server</modelDescription>\r\n"))
	w.Write([]byte("<modelName>Sample</modelName>\r\n"))
	w.Write([]byte("<modelNumber>0</modelNumber>\r\n"))
	w.Write([]byte("<modelURL>http://dvb.org</modelURL>\r\n"))
	w.Write([]byte("<serialNumber>0</serialNumber>\r\n"))
	w.Write([]byte("<UDN>"))
	w.Write([]byte(deviceuuid))
	w.Write([]byte("</UDN>\r\n"))
	w.Write([]byte("<UPC>Universal Product Code</UPC>\r\n"))
	w.Write([]byte("<iconList>\r\n"))
	w.Write([]byte("<icon>\r\n"))
	w.Write([]byte("<mimetype>image/png</mimetype>\r\n"))
	w.Write([]byte("<width>64</width>\r\n"))
	w.Write([]byte("<height>64</height>\r\n"))
	w.Write([]byte("<depth>24</depth>\r\n"))
	w.Write([]byte("<url>/icon.png</url>\r\n"))
	w.Write([]byte("</icon>\r\n"))
	w.Write([]byte("</iconList>\r\n"))
	w.Write([]byte("<presentationURL>/index.html</presentationURL>\r\n"))
	w.Write([]byte("</device>\r\n"))
	w.Write([]byte("</root>\r\n"))
}

func UPNPIconHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "image/png")
	w.Write(icondata)
}

func UPNPStart(svrmux *http.ServeMux, serverport int, location string) {
	var err error
	local_address := upnp_getlocal_address()

	locationroot := strings.Join([]string{"http://", local_address, ":", fmt.Sprintf("%d", serverport)}, "")

	log.Printf("UPNP base location %s\n", locationroot)

	deviceuuid = upnp_generate_uuid()

	rootadvertiser, err = ssdp.Advertise(
		"upnp:rootdevice",              // send as "ST"
		deviceuuid+"::upnp:rootdevice", // send as "USN"
		locationroot+SERVERDESCPATH,    // send as "LOCATION"
		SERVERSTRING,                   // send as "SERVER"
		1800)                           // send as "maxAge" in "CACHE-CONTROL"

	if err != nil {
		panic(err)
	}

	// add handler for device description and icon
	svrmux.HandleFunc(SERVERDESCPATH, UPNPDeviceDescHandler)
	svrmux.HandleFunc(ICONPATH, UPNPIconHandler)

	adticker = time.NewTicker(300 * time.Second)

	go func() {
		for _ = range adticker.C {
			log.Println("SSDP advertise")
			rootadvertiser.Alive()
		}
	}()
}

func UPNPStop() {
	adticker.Stop()
	rootadvertiser.Bye()
	rootadvertiser.Close()
}

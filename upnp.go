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

type UPnPDevice struct {
	rootadvertiser    *ssdp.Advertiser
	deviceadvertiser  *ssdp.Advertiser
	serviceadvertiser *ssdp.Advertiser

	adticker      *time.Ticker
	device_uuid   string
	local_address string

	server_name string
	icon_path   string

	server_desc_path  string
	server_port       int
	presentation_page string
	presentation_url  string
}

const defaultuuid = "uuid:11e77140-70dc-4d30-80dd-c6ddae09bd41"

// this trick is to get local IP
func (d *UPnPDevice) getlocal_address() {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()

	addr := conn.LocalAddr().String()

	d.local_address = strings.Split(addr, ":")[0]
}

// generate a unique id from MAC address
func (d *UPnPDevice) generate_uuid() {
	interfaces, err := net.Interfaces()
	if err != nil {
		// use use static uuid
		d.device_uuid = defaultuuid
		return
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
			//h.Write([]byte(time.Now().Format(time.UnixDate)))
			hashedMAC := h.Sum(nil)

			log.Printf("Generate UUID from MAC address of %s\n", i.Name)

			var result strings.Builder
			// format part of hash as uuid
			fmt.Fprintf(&result, "uuid:%02x%02x%02x%02x-", hashedMAC[0], hashedMAC[1], hashedMAC[2], hashedMAC[3])
			fmt.Fprintf(&result, "%02x%02x-", hashedMAC[4], hashedMAC[5])
			fmt.Fprintf(&result, "%02x%02x-", hashedMAC[6], hashedMAC[7])
			fmt.Fprintf(&result, "%02x%02x-", hashedMAC[8], hashedMAC[9])
			fmt.Fprintf(&result, "%02x%02x%02x%02x%02x%02x", hashedMAC[10], hashedMAC[11], hashedMAC[12], hashedMAC[13], hashedMAC[14], hashedMAC[15])

			log.Println(result.String())

			d.device_uuid = result.String()

			return
		}
	}

	// use use static uuid
	d.device_uuid = defaultuuid
}

func (d *UPnPDevice) DeviceDescHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/xml")

	log.Printf("Request UPNP root description from %s", r.RemoteAddr)

	w.Write([]byte("<?xml version=\"1.0\"?>\n"))
	w.Write([]byte("<root xmlns=\"urn:schemas-upnp-org:device-1-0\" configId=\"0\">\n"))
	w.Write([]byte("<specVersion>\n"))
	w.Write([]byte("<major>1</major>\n"))
	w.Write([]byte("<minor>1</minor>\n"))
	w.Write([]byte("</specVersion>\n"))
	w.Write([]byte("<device>\n"))
	w.Write([]byte("<deviceType>urn:ses-com:device:SatIPServer:1</deviceType>\n"))
	w.Write([]byte("<friendlyName>Home Broadcast Server</friendlyName>\n"))
	w.Write([]byte("<manufacturer>DVB</manufacturer>\n"))
	w.Write([]byte("<manufacturerURL>http://dvb.org</manufacturerURL>\n"))
	w.Write([]byte("<modelDescription>Sample Home Broadcasting Server</modelDescription>\n"))
	w.Write([]byte("<modelName>Sample</modelName>\n"))
	w.Write([]byte("<modelNumber>0</modelNumber>\n"))
	w.Write([]byte("<modelURL>http://dvb.org</modelURL>\n"))
	w.Write([]byte("<serialNumber>0</serialNumber>\n"))
	w.Write([]byte("<UDN>"))
	w.Write([]byte(d.device_uuid))
	w.Write([]byte("</UDN>\n"))
	w.Write([]byte("<UPC>Universal Product Code</UPC>\n"))
	w.Write([]byte("<presentationURL>"))
	w.Write([]byte(d.presentation_url))
	w.Write([]byte("</presentationURL>\n"))

	w.Write([]byte("<iconList>\n"))
	w.Write([]byte("<icon>\n"))
	w.Write([]byte("<mimetype>image/png</mimetype>\n"))
	w.Write([]byte("<width>64</width>\n"))
	w.Write([]byte("<height>64</height>\n"))
	w.Write([]byte("<depth>24</depth>\n"))
	w.Write([]byte("<url>/icon.png</url>\n"))
	w.Write([]byte("</icon>\n"))
	w.Write([]byte("</iconList>\n"))

	// w.Write([]byte("<serviceList>"))
	// w.Write([]byte("<service>"))
	// w.Write([]byte("<serviceType>urn:ses-com:device:SatIPServer:1</serviceType>"))
	// w.Write([]byte("<serviceId>urn:ses-com:serviceId:SatIPServer</serviceId>"))
	// w.Write([]byte("</service>"))
	// w.Write([]byte("</serviceList>"))

	w.Write([]byte("</device>\n"))
	w.Write([]byte("</root>\n"))
}

func (d *UPnPDevice) Start(svrmux *http.ServeMux) {
	var err error
	var locationroot string
	// compute internal values
	d.getlocal_address()
	d.generate_uuid()

	if d.server_port == 80 {
		locationroot = strings.Join([]string{"http://", d.local_address}, "")
	} else {
		locationroot = strings.Join([]string{"http://", d.local_address, ":", fmt.Sprintf("%d", d.server_port)}, "")
	}

	d.presentation_url = locationroot + d.presentation_page

	log.Printf("UPNP base location %s\n", locationroot)

	server_desc_url := locationroot + d.server_desc_path

	// ROOT ADVERTISER
	d.rootadvertiser, err = ssdp.Advertise(
		"upnp:rootdevice",                 // send as "ST"
		d.device_uuid+"::upnp:rootdevice", // send as "USN"
		server_desc_url,                   // send as "LOCATION"
		d.server_name,                     // send as "SERVER"
		1800)                              // send as "maxAge" in "CACHE-CONTROL"

	if err != nil {
		panic(err)
	}

	// DEVICE ADVERTISER
	d.deviceadvertiser, err = ssdp.Advertise(
		d.device_uuid,   // send as "ST"
		d.device_uuid,   // send as "USN"
		server_desc_url, // send as "LOCATION"
		d.server_name,   // send as "SERVER"
		1800)            // send as "maxAge" in "CACHE-CONTROL"

	if err != nil {
		panic(err)
	}

	// SERVICE ADVERTISER
	d.serviceadvertiser, err = ssdp.Advertise(
		"urn:ses-com:device:SatIPServer:1",                 // send as "ST"
		d.device_uuid+"::urn:ses-com:device:SatIPServer:1", // send as "USN"
		server_desc_url, // send as "LOCATION"
		d.server_name,   // send as "SERVER"
		1800)            // send as "maxAge" in "CACHE-CONTROL"

	if err != nil {
		panic(err)
	}

	// add handler for device description and icon
	svrmux.HandleFunc(d.server_desc_path, d.DeviceDescHandler)

	d.adticker = time.NewTicker(300 * time.Second)

	log.Println("SSDP first advertise")
	d.rootadvertiser.Alive()
	d.deviceadvertiser.Alive()
	d.serviceadvertiser.Alive()

	go func() {
		for _ = range d.adticker.C {
			log.Println("SSDP advertise")
			d.rootadvertiser.Alive()
			d.deviceadvertiser.Alive()
			d.serviceadvertiser.Alive()
		}
	}()
}

func (d *UPnPDevice) Stop() {
	d.adticker.Stop()
	d.rootadvertiser.Bye()
	d.rootadvertiser.Close()
	d.deviceadvertiser.Bye()
	d.deviceadvertiser.Close()
	d.serviceadvertiser.Bye()
	d.serviceadvertiser.Close()
}

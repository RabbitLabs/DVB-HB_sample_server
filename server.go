package main

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"log"
	"mime"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strings"
)

// this is where static files for the embedded DVB-I client goes
//go:embed static
var static embed.FS

// change root of embedded FS
var htmlstatic, _ = fs.Sub(static, "static")

// server components to serve from static FS
var fileServer = http.FileServer(http.FS(htmlstatic))

var deviceconfig DeviceConfig

var tm TunerManager

const ICONPATH = "/icon.png"

// integrate icon file
//go:embed icon.png
var icondata []byte

var ServerUPnPDevice UPnPDevice

func configurationHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/javascript")

	fmt.Fprintf(w, "INSTALL_LOCATION=\"http://%s\";\n", r.Host)
}

func staticHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/" || r.URL.Path == "/index.html" {
		if strings.Contains(strings.ToLower(r.UserAgent()), "hbbtv") {
			http.Redirect(w, r, "/hbbtv/launcher/index.html", http.StatusFound)
		} else {
			http.Redirect(w, r, "/android/player.html", http.StatusFound)
		}
	} else {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		fileServer.ServeHTTP(w, r)
	}
}

func IconHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "image/png")
	w.Write(icondata)
}

func main() {
	var svr http.Server
	var svrmux http.ServeMux
	svr.Handler = &svrmux

	mime.AddExtensionType(".js", "application/javascript")

	//deviceconfig.WriteConfig("democonfig.yaml")
	deviceconfig.ReadConfig("democonfig.yaml")

	transcoderManager := CreateDynamicTranscode(deviceconfig.TunerConfig, deviceconfig.TranscodeConfig, deviceconfig.MaxTuner, deviceconfig.TunerList)

	RegisterDynamicContent("transcode", transcoderManager)

	// serve configuration file
	svrmux.HandleFunc("/configuration.js", configurationHandler)

	// serve channel map list and channel maps
	svrmux.HandleFunc(ChannelMapPath, channelmapHandler)

	// serve channel map list and channel maps
	svrmux.HandleFunc(DynamicContentPath, dynamicContentHandler)

	// serve static files
	svrmux.Handle("/video/", http.StripPrefix("/video/", http.FileServer(http.Dir("./video"))))
	svrmux.HandleFunc("/", staticHandler)

	virtualtuner, _ := NewVirtualTuner("test_tuner_config.yaml")

	tm.AttachTuner(virtualtuner)

	deviceconfig.RegisterDynamicChannelMap(virtualtuner)

	RegisterDynamicChannelMap(virtualtuner)

	deviceconfig.helpertoolsruntime = make([]*CommandLineTool, len(deviceconfig.HelperTools))
	Args := make(map[string]string)

	for i := range deviceconfig.HelperTools {
		deviceconfig.helpertoolsruntime[i] = CreateCommandLineTool(deviceconfig.HelperTools[i])
		Args["index"] = fmt.Sprintf("%d", i)
		deviceconfig.helpertoolsruntime[i].Start(Args)
	}

	// this channel is used to signal when the main server has stopped
	idleConnsClosed := make(chan struct{})

	// this async function wait for a keypress to stop server properly
	go func() {
		var b []byte = make([]byte, 1)
		os.Stdin.Read(b)
		fmt.Print("Closing ...\n")

		// We received an interrupt signal, shut down.
		if err := svr.Shutdown(context.Background()); err != nil {
			// Error from closing listeners, or context timeout:
			log.Printf("HTTP server Shutdown: %v", err)
		}
		// signal main that we have stopped
		close(idleConnsClosed)
	}()

	// configure default port to 80 if none is specified
	if deviceconfig.ServerPort == 0 {
		deviceconfig.ServerPort = 80
	}

	// under windows launch browser if requested to 
	if runtime.GOOS == "windows" && deviceconfig.OpenPage {
		exec.Command("rundll32", "url.dll,FileProtocolHandler", fmt.Sprintf("http://localhost:%d", deviceconfig.ServerPort)).Start()
	}

	fmt.Printf("Starting server at port %d\n", deviceconfig.ServerPort)
	svr.Addr = fmt.Sprintf(":%d", deviceconfig.ServerPort)

	svrmux.HandleFunc(ICONPATH, IconHandler)

	ServerUPnPDevice.icon_path = ICONPATH
	ServerUPnPDevice.server_port = deviceconfig.ServerPort
	ServerUPnPDevice.server_desc_path = "/server.xml"
	ServerUPnPDevice.server_name = "DVB-HB Sample Server 1.0"
	ServerUPnPDevice.presentation_page = "/index.html"
	ServerUPnPDevice.Start(&svrmux)

	// run server
	if err := svr.ListenAndServe(); err != nil {
		log.Fatal(err)
	}

	// wait to server to close
	<-idleConnsClosed

	// eventually stop launched tasks before exits (avoid hanging processes)
	log.Println("stopping running transcoders")
	transcoderManager.StopAll()

	log.Println("stopping running tools")

	for i := range deviceconfig.helpertoolsruntime {
		deviceconfig.helpertoolsruntime[i].Stop()
	} 

	ServerUPnPDevice.Stop()

	log.Println("Finished, exit")

}

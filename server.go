package main

import (
	"embed"
	"fmt"
	"io/fs"
	"log"
	"mime"
	"net/http"
	"strings"
)

// this is where static files for the embedded DVB-I client goes
//go:embed static
var static embed.FS

// change root of embedded FS 
var htmlstatic, lapin = fs.Sub(static, "static")

// server components to serve from static FS
var fileServer = http.FileServer(http.FS(static))

// server components to serve demo media from OS FS
var staticMediaServer = http.FileServer(http.Dir("./demo"))

var deviceconfig DeviceConfig

func configurationHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/htmlstatic")

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
		fileServer.ServeHTTP(w, r)
	}
}

func main() {
	mime.AddExtensionType(".js", "application/javascript")

	//deviceconfig.WriteConfig("democonfig.yaml")
	deviceconfig.ReadConfig("democonfig.yaml")

	// serve configuration file
	http.HandleFunc("/configuration.js", configurationHandler)

	// serve channel map list and channel maps
	http.HandleFunc(ChannelMapPath, channelmapHandler)

	// serve static files
	//http.Handle("/", fileServer)
	http.HandleFunc("/", staticHandler)

	fmt.Printf("Starting server at port 8082\n")
	if err := http.ListenAndServe(":8082", nil); err != nil {
		log.Fatal(err)
	}
}

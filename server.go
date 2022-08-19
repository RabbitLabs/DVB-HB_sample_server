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

func main() {
	var svr http.Server
	var svrmux http.ServeMux
	svr.Handler = &svrmux
 
	mime.AddExtensionType(".js", "application/javascript")

	//deviceconfig.WriteConfig("democonfig.yaml")
	deviceconfig.ReadConfig("democonfig.yaml")

	// serve configuration file
	

	svrmux.HandleFunc("/configuration.js", configurationHandler)

	// serve channel map list and channel maps
	svrmux.HandleFunc(ChannelMapPath, channelmapHandler)

	// serve channel map list and channel maps
	svrmux.HandleFunc(DynamicContentPath, dynamicHandler)

	// serve static files

	svrmux.Handle("/video/", http.StripPrefix("/video/", http.FileServer(http.Dir("./video"))))
	svrmux.HandleFunc("/", staticHandler)

	virtualtuner, _ := NewVirtualTuner("test_tuner_config.yaml")

	tm.AttachTuner(virtualtuner)

	deviceconfig.RegisterDynamicChannelMap(virtualtuner)

	// transcoder := CreateCommandLineTranscoder("ffmpeg", "-f mpegts -re -i pipe: -map 0:v -map 0:a -c:a aac -c:v h264 -b:v:0 2M -profile:v:0 main -bf 1 -keyint_min 25 -g 25 -sc_threshold 0 -b_strategy 0 -f dash out.mpd")
	transcoder := CreateCommandLineTranscoder("ffmpeg", "-f mpegts -analyzeduration 1M -probesize 1M -vsync 0 -i udp://127.0.0.1:${port}?fifo_size=1000000&overrun_nonfatal=1 -map 0:v -map 0:a -c:a aac -c:v h264_nvenc -rc-lookahead 25 -b:v:0 5M -minrate 6M -maxrate 6M -bufsize 12M -pix_fmt yuv420p -profile:v:0 main -bf 1 -remove_at_exit 1 -keyint_min 25 -g 25 -sc_threshold 0 -b_strategy 0 -f dash out.mpd", 9001)

	go func() {
		tschannel := tm.GetChannel()

		for pkt := range tschannel {
			transcoder.ProcessPacket(pkt)
		}
	}()

	//virtualtuner.Tune("C")

	//transcoder.Start("video/live/")

	RegisterDynamicChannelMap(virtualtuner)

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


	fmt.Printf("Starting server at port 8082\n")
	svr.Addr = ":8082" 

	// run server
	if err := svr.ListenAndServe(); err != nil {
		log.Fatal(err)
	}
	
	// wait to server to close
	<-idleConnsClosed

	// eventually stop launched tasks before exits (avoid hanging processes)
	virtualtuner.Stop()
	transcoder.Stop()	
}

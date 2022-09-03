package main

import (
	"log"
	"net/http"
	"os"
	"strings"
	"time"
)

// time out to check if a client is still listening to transcode
const tickTime time.Duration = time.Second
const tickTimeout int = 15
// time out at startup of transcode
const startTick time.Duration = 100 * time.Millisecond
const startTimeout int = 75

// a running instance of transcode
type DynamicTranscodeInstance struct {
	Args map[string]string
	WorkDir string
	Tuner *CommandLineTool
	Transcoder *CommandLineTool
	TimeOut int
}

// the transcoder manager which create and destroy transcode instances according to client requests
// transcode manager also serve client request 
type DynamicTranscodeManager struct {
	// configuration for tuner and transcoder
	configTuner CommandLineToolConfig
	configTranscoder CommandLineToolConfig
	// list running trancoder instances
	activeInstances map[string]*DynamicTranscodeInstance
	// a ticker to check if transcode instance needs to be flushed
	ticker *time.Ticker
}

// stop a running instance, stopping the tuner closes data channel and stop also transcoder 
func (d *DynamicTranscodeInstance) Stop() {
	if (d.Transcoder != nil) {
		d.Transcoder.Stop()
	}	
	if (d.Tuner != nil) {
		d.Tuner.Stop()
	}
}

// create a transcode manager
func CreateDynamicTranscode(configTuner CommandLineToolConfig, configTranscoder CommandLineToolConfig)  *DynamicTranscodeManager {
	t := new(DynamicTranscodeManager)

	t.configTuner = configTuner
	t.configTranscoder = configTranscoder
	t.activeInstances = make(map[string]*DynamicTranscodeInstance)
	t.ticker = time.NewTicker(tickTime)

	// launch the asynchronous cleaning of inactive instances
	go t.RunTimeOut()

	return t
}

// a function running in the backgound to cleanup inactive instances
func (t *DynamicTranscodeManager)  RunTimeOut() {

	for _ = range t.ticker.C {
		for name, instance := range t.activeInstances {
			instance.TimeOut--
			//log.Printf("Tick Instance %s time out is %d\n", name, instance.TimeOut)
			if (instance.TimeOut <= 0) {
				log.Printf("Stopping Instance %s after timeout\n", name)
				instance.Stop()
				delete(t.activeInstances, name)
			}
		}
	}

}

// stop all running instances (called before exists to avoid hanging processes)
func (t *DynamicTranscodeManager)  StopAll() {
	for name, instance := range t.activeInstances {
		instance.Stop()
		delete(t.activeInstances, name)
	}	
}

// serve request from dynamic content by clients, creates a transcode instance if none is active for a request
func (t *DynamicTranscodeManager) ServeDynamicContent(w http.ResponseWriter, r *http.Request, path string) {
	// split full path
	splitPath := strings.SplitN(path, "/", 3)
	// build instance path
	instancePath := strings.Join( []string{splitPath[0], splitPath[1] }, "/" )
	
	// try to lookup instance
	activeInstance, found := t.activeInstances[instancePath]

	if (!found) {
		log.Printf("Instance for %s not found, creating new one\n", instancePath)
		source, sourcefound := deviceconfig.Feeds[splitPath[0]]

		if (!sourcefound) {
			http.Error(w, "404 not found. Unknown channel", http.StatusNotFound)
			return
		}

		// create new instance
		activeInstance = new(DynamicTranscodeInstance)

		// configure instance
		activeInstance.WorkDir = "0"

		activeInstance.Args = make(map[string]string)
		
		activeInstance.Args["source"] = source
		activeInstance.Args["program"] = splitPath[1]
		activeInstance.Args["tunerindex"] = activeInstance.WorkDir
	
		//localTunerConfig := t.configTuner
		//localTunerConfig.WorkDir = activeInstance.WorkDir
		activeInstance.Tuner = CreateCommandLineTool(t.configTuner)

		//localTranscoderConfig := t.configTranscoder
		//localTranscoderConfig.WorkDir = activeInstance.WorkDir
		activeInstance.Transcoder = CreateCommandLineTool(t.configTranscoder)

		// set timeout before adding to list
		activeInstance.TimeOut = tickTimeout
		// add to list of active instances	
		t.activeInstances[instancePath] = activeInstance

		// link pipes
		activeInstance.Transcoder.SetInputPipe(activeInstance.Tuner.GetOutputPipe())

		activeInstance.Transcoder.Start(activeInstance.Args)
		activeInstance.Tuner.Start(activeInstance.Args)
	}

	// path to file to serve
	filePath := strings.Join( []string { activeInstance.WorkDir, splitPath[2] }, "/")

	log.Printf("accessing file %s\n", filePath)

	// check if file exists, if file is not yet present wait a bit for the transcode process to start
	fileNotExists := true
	timeOut := startTimeout

	for (fileNotExists) {
		_ , error := os.Stat(filePath)

		fileNotExists = os.IsNotExist(error)

		// check if file exists
		if (fileNotExists) {
			// run time out
			timeOut--

			if (timeOut <=0) {
				log.Printf("File %s does not exists\n", filePath)
				http.Error(w, "404 not found.", http.StatusNotFound)
				return
			}
			// sleep a bit
			time.Sleep(startTick)
		}
	}

	// reset timeout on this instance
	activeInstance.TimeOut = tickTimeout	

	log.Printf("Serving file %s\n", filePath)

	// serve content
	http.ServeFile(w, r, filePath)
}
package main

import (
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// time out to check if a client is still listening to transcode
const tickTime time.Duration = time.Second
const tickTimeout int = 8
const startTimeout int = 15

// time out at startup of transcode
const fileTick time.Duration = 10 * time.Millisecond
const fileTimeout int = 1000

// a running instance of transcode
type DynamicTranscodeInstance struct {
	Args          map[string]string
	InstanceIndex int
	Tuner         *CommandLineTool
	Transcoder    *CommandLineTool
	TimeOut       int
}

// the transcoder manager which create and destroy transcode instances according to client requests
// transcode manager also serve client request
type DynamicTranscodeManager struct {
	// configuration for tuner and transcoder
	configTuner      CommandLineToolConfig
	configTranscoder CommandLineToolConfig
	maxTuner         int
	tunerList        []int
	// list running trancoder instances
	activeInstances map[string]*DynamicTranscodeInstance
	// a ticker to check if transcode instance needs to be flushed
	ticker *time.Ticker
}

// stop a running instance, stopping the tuner closes data channel and stop also transcoder
func (d *DynamicTranscodeInstance) Stop() {
	// if (d.Transcoder != nil) {
	// 	d.Transcoder.Stop()
	// }
	if d.Tuner != nil {
		d.Tuner.Stop()
	}
}

func (d *DynamicTranscodeInstance) RemoveAllContent() error {
	path := strconv.Itoa(d.InstanceIndex)
	directory, err := os.Open(path)
	if err != nil {
		return err
	}
	defer directory.Close()
	names, err := directory.Readdirnames(-1)
	if err != nil {
		return err
	}
	for _, name := range names {
		err = os.RemoveAll(filepath.Join(path, name))
		if err != nil {
			return err
		}
	}
	return nil
}

// create a transcode manager
func CreateDynamicTranscode(configTuner CommandLineToolConfig, configTranscoder CommandLineToolConfig, maxTuner int, tunerList []int) *DynamicTranscodeManager {
	t := new(DynamicTranscodeManager)

	t.configTuner = configTuner
	t.configTranscoder = configTranscoder
	t.maxTuner = maxTuner
	t.tunerList = tunerList
	t.activeInstances = make(map[string]*DynamicTranscodeInstance)
	t.ticker = time.NewTicker(tickTime)

	// launch the asynchronous cleaning of inactive instances
	go t.RunTimeOut()

	return t
}

// a function running in the backgound to cleanup inactive instances
func (t *DynamicTranscodeManager) RunTimeOut() {

	for _ = range t.ticker.C {
		for name, instance := range t.activeInstances {
			instance.TimeOut--
			log.Printf("Tick Instance %s time out is %d\n", name, instance.TimeOut)
			if instance.TimeOut <= 0 {
				log.Printf("Stopping Instance %s after timeout\n", name)
				instance.Stop()
				delete(t.activeInstances, name)
			}
		}
	}

}

// stop all running instances (called before exists to avoid hanging processes)
func (t *DynamicTranscodeManager) StopAll() {
	for name, instance := range t.activeInstances {
		instance.Stop()
		delete(t.activeInstances, name)
	}
}

// check if a specific tuner index is in use
func (t *DynamicTranscodeManager) IsTunerUsed(n int) bool {
	// scan all instances
	for _, instance := range t.activeInstances {
		// if one is matching return is use
		if instance.InstanceIndex == n {
			return true
		}
	}

	// return free instance
	return false
}

func (t *DynamicTranscodeManager) AllocateTuner() int {
	if len(t.tunerList) > 0 {
		for i := range t.tunerList {
			if !t.IsTunerUsed(t.tunerList[i]) {
				return t.tunerList[i]
			}
		}
	}

	for i := 0; i < t.maxTuner; i++ {
		if !t.IsTunerUsed(i) {
			return i
		}
	}
	return -1
}

// serve request from dynamic content by clients, creates a transcode instance if none is active for a request
func (t *DynamicTranscodeManager) ServeDynamicContent(w http.ResponseWriter, r *http.Request, path string) {
	// split full path
	splitPath := strings.SplitN(path, "/", 3)

	// build instance path
	instancePath := strings.Join([]string{splitPath[0], splitPath[1]}, "/")

	aliasname, aliasFound := deviceconfig.Aliases[instancePath]

	if aliasFound {
		log.Printf("Replacing reference from %s to %s\n", instancePath, aliasname)
		instancePath = aliasname
		aliasSplitPath := strings.SplitN(aliasname, "/", 2)
		splitPath[0] = aliasSplitPath[0]
		splitPath[1] = aliasSplitPath[1]
	}

	// try to lookup instance
	activeInstance, found := t.activeInstances[instancePath]

	if !found {
		log.Printf("Instance for %s not found, creating new one\n", instancePath)
		source, sourcefound := deviceconfig.Feeds[splitPath[0]]

		if !sourcefound {
			http.Error(w, "404 not found. Unknown channel", http.StatusNotFound)
			return
		}

		Index := t.AllocateTuner()
		sIndex := strconv.Itoa(Index)

		if Index < 0 {
			log.Printf("Cannot allocate tuner\n")
			http.Error(w, "429 too many request", http.StatusTooManyRequests)
			return
		}

		// create new instance
		activeInstance = new(DynamicTranscodeInstance)

		// configure instance
		activeInstance.InstanceIndex = Index

		// cleanup existing content in directory
		activeInstance.RemoveAllContent()

		// check if work directory exists
		_, error := os.Stat(sIndex)

		// create transcode directory if it does not exists
		if os.IsNotExist(error) {
			err := os.MkdirAll(sIndex, 0660)
			if err != nil {
				log.Printf("cannot create working directory for instance %d\n%s", activeInstance.InstanceIndex, err)
			}
		}

		// create parameters for tools
		activeInstance.Args = make(map[string]string)

		activeInstance.Args["source"] = source
		activeInstance.Args["program"] = splitPath[1]
		activeInstance.Args["tunerindex"] = sIndex

		// create tool for receiving
		localTunerConfig := t.configTuner
		localTunerConfig.PortOffset = (uint16)(activeInstance.InstanceIndex)
		activeInstance.Tuner = CreateCommandLineTool(localTunerConfig)

		// create tool for transcoding
		localTranscoderConfig := t.configTranscoder
		localTranscoderConfig.PortOffset = (uint16)(activeInstance.InstanceIndex)
		activeInstance.Transcoder = CreateCommandLineTool(localTranscoderConfig)

		// set start timeout before adding to list, starting requires longer timeout
		activeInstance.TimeOut = startTimeout
		// add to list of active instances
		t.activeInstances[instancePath] = activeInstance

		// link pipes
		activeInstance.Transcoder.SetInputPipe(activeInstance.Tuner.GetOutputPipe())

		activeInstance.Transcoder.Start(activeInstance.Args)
		activeInstance.Tuner.Start(activeInstance.Args)
	}

	// path to file to serve
	filePath := strings.Join([]string{strconv.Itoa(activeInstance.InstanceIndex), splitPath[2]}, "/")

	//log.Printf("accessing file %s\n", filePath)

	// check if file exists, if file is not yet present wait a bit for the transcode process to start
	fileNotExists := true
	timeOut := fileTimeout

	for fileNotExists {
		_, error := os.Stat(filePath)

		fileNotExists = os.IsNotExist(error)

		// check if file exists
		if fileNotExists {
			// run time out
			timeOut--

			if timeOut <= 0 {
				log.Printf("Timed out, File %s does not exists\n", filePath)
				http.Error(w, "404 not found.", http.StatusNotFound)
				return
			}
			// sleep a bit
			time.Sleep(fileTick)
		}
	}

	// reset timeout on this instance
	activeInstance.TimeOut = tickTimeout

	//log.Printf("Serving file %s\n", filePath)

	// serve content
	http.ServeFile(w, r, filePath)
}

package main

import (
	"log"
	"net/http"
	"os"
	"strings"
	"time"
)

type DynamicTranscodeInstance struct {
	Args map[string]string
	WorkDir string
	Tuner *CommandLineTool
	Transcoder *CommandLineTool
}

type DynamicTranscodeManager struct {
	configTuner CommandLineToolConfig
	configTranscoder CommandLineToolConfig
	activeInstances map[string]*DynamicTranscodeInstance
}

func CreateDynamicTranscode(configTuner CommandLineToolConfig, configTranscoder CommandLineToolConfig)  *DynamicTranscodeManager {
	t := new(DynamicTranscodeManager)

	t.configTuner = configTuner
	t.configTranscoder = configTranscoder
	t.activeInstances = make(map[string]*DynamicTranscodeInstance)

	return t
}

func (t *DynamicTranscodeManager) ServeDynamicContent(w http.ResponseWriter, r *http.Request, path string) {
	// split full path
	splitPath := strings.SplitN(path, "/", 3)
	// build instance path
	instancePath := strings.Join( []string{splitPath[0], splitPath[1] }, "/" )
	
	// try to lookup instance
	activeInstance, found := t.activeInstances[instancePath]

	if (!found) {
		tuneParams := strings.Split(splitPath[0], "_")
		activeInstance = new(DynamicTranscodeInstance)
		t.activeInstances[instancePath] = activeInstance
		activeInstance.WorkDir = "0"

		activeInstance.Args = make(map[string]string)
		
		activeInstance.Args["system"] = tuneParams[0]
		activeInstance.Args["modulation"] = tuneParams[1]
		activeInstance.Args["frequency"] = tuneParams[2]
		activeInstance.Args["baudrate"] = tuneParams[3]
		activeInstance.Args["polarity"] = tuneParams[4]
		activeInstance.Args["program"] = splitPath[1]
		activeInstance.Args["tunerindex"] = activeInstance.WorkDir
	
		localTunerConfig := t.configTuner
		localTunerConfig.WorkDir = activeInstance.WorkDir
		activeInstance.Tuner = CreateCommandLineTool(localTunerConfig)

		localTranscoderConfig := t.configTranscoder
		localTranscoderConfig.WorkDir = activeInstance.WorkDir
		activeInstance.Transcoder = CreateCommandLineTool(localTranscoderConfig)

		// link pipes
		activeInstance.Transcoder.SetInputPipe(activeInstance.Tuner.GetOutputPipe())

		activeInstance.Tuner.Start(activeInstance.Args)
		activeInstance.Transcoder.Start(activeInstance.Args)
	}

	filePath := strings.Join( []string { activeInstance.WorkDir, splitPath[2] }, "/")

	log.Printf("accessing file %s\n", filePath)

	fileExists := false
	timeOut := 50

	for (!fileExists) {
		_ , error := os.Stat(filePath)

		// check if file exists
		fileExists = os.IsExist(error)

		if (!fileExists) {
			// run time out
			timeOut--

			if (timeOut <=0) {
				http.Error(w, "404 not found.", http.StatusNotFound)
				return
			}
			// sleep a bit
			time.Sleep(100 * time.Millisecond)
		}
	}

	http.ServeFile(w, r, filePath)
}
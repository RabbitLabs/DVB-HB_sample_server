package main

import (
	"bufio"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"regexp"
	"time"

	"github.com/Comcast/gots/packet"
	"gopkg.in/yaml.v3"
)

type ExternConfig struct {
	Command string `yaml:"command"`
	Args    string `yaml:"args"`
}

type VirtualServiceConfig struct {
	Name string `yaml:"name"`
	LCN  int    `yaml:"lcn"`
	SID  int    `yaml:"sid"`
}

type VirtualFrequencyConfig struct {
	TuneString string                          `yaml:"tunestring"`
	File       string                          `yaml:"file"`
	Port       string                          `yaml:"port"`
	Extern     ExternConfig                    `yaml:"extern"`
	BitRate    int                             `yaml:"bitrate"`
	TSID       int                             `yaml:"tsid"`
	ONID       int                             `yaml:"onid"`
	Services   map[string]VirtualServiceConfig `yaml:"services"`
}

type VirtualTunerConfig struct {
	Description string                            `yaml:"description"`
	Provider    string                            `yaml:"provider"`
	ProviderURL string                            `yaml:"providerurl"`
	Frequencies map[string]VirtualFrequencyConfig `yaml:"frequencies"`
	Transcode   ExternConfig                      `yaml:"transcode"`
}

type VirtualTuner struct {
	// configuration of the virtual tuner (read from file)
	config VirtualTunerConfig

	// list of channel names (to keep fixed order while scanning)
	frequencynames []string
	// current position in scan
	scanfrequencyindex int
	// current active channel (tune string)
	currentfrequency *VirtualFrequencyConfig

	// current file for active channel
	currentfile *os.File
	// current connection to receive UDP packets
	currentconnection *net.UDPConn

	// using command line input
	input      exec.Cmd
	pipestdin  io.WriteCloser
	pipestdout io.ReadCloser
	pipestderr io.ReadCloser

	// the golang channel to output MPEG TS Packets
	tschannel MpegTSChannel
	// ticker to send timer event to read streams
	streamticker *time.Ticker
}

func NewVirtualTuner(ConfigFile string) (*VirtualTuner, error) {
	var vt VirtualTuner

	source, err := ioutil.ReadFile(ConfigFile)

	if err != nil {
		return nil, err
	}

	err = yaml.Unmarshal(source, &vt.config)

	if err != nil {
		return nil, err
	}

	// create a table for channel tune string
	vt.frequencynames = make([]string, 0, len(vt.config.Frequencies))

	// fill table of channel tune string
	for k := range vt.config.Frequencies {
		vt.frequencynames = append(vt.frequencynames, k)
	}

	vt.scanfrequencyindex = 0
	vt.tschannel = make(MpegTSChannel, 128)

	return &vt, nil
}

// get the channel to receive MPEG TS Packets
func (vt *VirtualTuner) GetChannel() MpegTSChannel {
	return vt.tschannel
}

// tick handler to stream a file
func (vt *VirtualTuner) filestreamer() {
	lasttime := time.Now()
	bitbudget := 0

	// receive time tick from ticker channel
	for currenttime := range vt.streamticker.C {
		// compute effective offset (tick may not occur with exact timing)
		offset := currenttime.Sub(lasttime)
		// store time for next offset
		lasttime = currenttime

		// compute the bit budget for this tick based on target bitrate and ellapsed time
		bitbudget += int(offset.Seconds() * float64(vt.currentfrequency.BitRate))

		// while there is bit budget, read some packets
		for bitbudget > (packet.PacketSize * 8) {
			readpacket := new(packet.Packet)

			readlen, err := vt.currentfile.Read(readpacket[:])

			if err != nil {
				// loop if end of file
				if err == io.EOF {
					vt.currentfile.Seek(0, 0)
				} else {
					vt.streamticker.Stop()
				}
			}

			// check size of read packet
			if readlen != packet.PacketSize {
				// assume that source file contains partial packet, loop so that next read will read EOF
				continue
			}

			// remove read packet from budget
			bitbudget -= packet.PacketSize * 8
			// forward to output channel
			vt.tschannel <- *readpacket
		}
	}
}

func (vt *VirtualTuner) udpstreamer() {
	buffer := make([]byte, 1500)

	for {
		packetsize, _, err := vt.currentconnection.ReadFrom(buffer)
		index := 0
		if err != nil {
			continue
		}

		for packetsize >= packet.PacketSize {
			readpacket := new(packet.Packet)
			copy(readpacket[:], buffer[index:index+packet.PacketSize])
			vt.tschannel <- *readpacket
			packetsize -= packet.PacketSize
			index += packet.PacketSize
		}

		if packetsize != 0 {
			log.Print("residue in UDP")
		}
	}

}

func (vt *VirtualTuner) handleConsoleReader(reader io.ReadCloser) {
	bufreader := bufio.NewReader(reader)

	for {
		str, err := bufreader.ReadString('\n')
		if err != nil {
			break
		}
		fmt.Print(str)
	}
}

func (vt *VirtualTuner) handleTSReader(reader io.ReadCloser) {
	for {
		readpacket := new(packet.Packet)

		readlen, err := reader.Read(readpacket[:])

		if err != nil {
			return
		}

		// check size of read packet
		if readlen != packet.PacketSize {
			// assume that source file contains partial packet, loop so that next read will read EOF
			continue
		}

		// forward to output channel
		vt.tschannel <- *readpacket
	}
}

// tune to a TS, true if OK
func (vt *VirtualTuner) Tune(parameters string) bool {
	var err error
	targetchannel, found := vt.config.Frequencies[parameters]

	// check if channel exists
	if !found {
		return false
	}

	// store current channel
	vt.currentfrequency = &targetchannel

	// check if source is a file
	if vt.currentfrequency.File != "" {
		// try to open TS file
		vt.currentfile, err = os.Open(vt.currentfrequency.File)

		if err != nil {
			return false
		}

		// either create a ticker or restart existing one
		if vt.streamticker != nil {
			vt.streamticker.Reset(20 * time.Millisecond)
		} else {
			vt.streamticker = time.NewTicker(20 * time.Millisecond)
		}

		// this asynchronous go routine will get timer tick and read from file
		go vt.filestreamer()

		return true
	}

	if vt.currentfrequency.Port != "" {
		var err error
		listenport := ":" + vt.currentfrequency.Port
		//vt.currentconnection, err = net.DialUDP("udp", listenport)
		addr, _ := net.ResolveUDPAddr("udp", listenport)
		vt.currentconnection, err = net.ListenUDP("udp", addr)
		err = vt.currentconnection.SetReadBuffer(2 * 1024 * 1024)

		if err != nil {
			return false
		}

		go vt.udpstreamer()

		return true
	}

	if vt.currentfrequency.Extern.Command != "" {
		var err error

		// regexep to parse argument string
		r := regexp.MustCompile("'.+'|\".+\"|\\S+")

		//	t.transcoder = *exec.Command(t.cmd, strings.Fields(args)...)  // this will split only by white space
		vt.input = *exec.Command(vt.currentfrequency.Extern.Command, r.FindAllString(vt.currentfrequency.Extern.Args, -1)...) // this will take in account quote around arguments

		// get stdin to flow data
		vt.pipestdin, err = vt.input.StdinPipe()

		if err != nil {
			log.Print(err)
			return false
		}

		// get output
		vt.pipestdout, err = vt.input.StdoutPipe()

		if err != nil {
			log.Print(err)
			return false
		}

		// dump to console
		go vt.handleTSReader(vt.pipestdout)

		// get error output
		vt.pipestderr, err = vt.input.StderrPipe()

		if err != nil {
			log.Print(err)
			return false
		}

		// dump to console
		go vt.handleConsoleReader(vt.pipestderr)

		err = vt.input.Start()

		if err != nil {
			log.Print(err)
			return false
		}

		return true
	}

	return false
}

// stop TS
func (vt *VirtualTuner) Stop() {
	// stop tuner ticker (but don't delete it)
	if vt.streamticker != nil {
		vt.streamticker.Stop()
	}

	// close file and unreference
	if vt.currentfile != nil {
		vt.currentfile.Close()
		vt.currentfile = nil
	}

	vt.currentconnection.Close()

	if vt.input.Process != nil {
		//vt.pipestdin.Write([]byte{'q'} )
		vt.pipestdin.Close()
		vt.pipestdout.Close()
		vt.pipestderr.Close()
		vt.input.Process.Kill()
		vt.input.Wait()
	}
}

// start a frequency scan, return tune string or empty on failure
func (vt *VirtualTuner) StartScan() string {
	vt.scanfrequencyindex = 0

	// if not channel registered, just stop
	if len(vt.frequencynames) == 0 {
		vt.Stop()
		return ""
	}

	// tune to first channel
	vt.Tune(vt.frequencynames[0])

	return vt.frequencynames[0]
}

// go to next frequency during a scan, return tune string or empty on failure
func (vt *VirtualTuner) ScanNext() string {
	// move to next channel
	vt.scanfrequencyindex++

	// if we got past last channel, just stop
	if vt.scanfrequencyindex >= len(vt.frequencynames) {
		vt.Stop()
		return ""
	}

	// tune to next channel
	vt.Tune(vt.frequencynames[vt.scanfrequencyindex])

	return vt.frequencynames[vt.scanfrequencyindex]
}

func (vt *VirtualTuner) GetChannelInfo() ChannelMap {
	cm := new(ChannelMap)

	cm.Description = vt.config.Description
	cm.Provider = vt.config.Provider
	cm.ProviderURL = vt.config.ProviderURL
	cm.Channels = make(map[int]Channel)

	return *cm
}

func (vt *VirtualTuner) GetChannelMap() ChannelMap {
	cm := vt.GetChannelInfo()

	for tunestring, freq := range vt.config.Frequencies {
		for sid, svc := range freq.Services {
			var newchannel Channel
			newchannel.Name = svc.Name
			newchannel.Tune = tunestring
			newchannel.Source = tunestring + "/" + sid
			newchannel.Dynamic = true
			cm.Channels[svc.LCN] = newchannel
		}
	}

	return cm
}

func (vt *VirtualTuner) ServeDynamicContent(w http.ResponseWriter, r *http.Request, path string) {

}

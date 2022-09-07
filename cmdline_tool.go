package main

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"os/exec"
	"regexp"

	"github.com/Comcast/gots/packet"
)

// these are paremeters to launch a command line tool
type CommandLineToolConfig struct {
	// command to execute for the tool
	Command string `yaml:"command"` 
	// arguments to the command
	Args    string `yaml:"args"`
	// working directory
	WorkDir string `yaml:"workdir"`
	// send data to tool using UDP socket instead of stdin (leave to 0 to use stdin)
	PortIn uint16 `yaml:"portin"`
	// get data from tool using UDP socket instead of stdout (leave to 0 to use stdout)
	PortOut uint16 `yaml:"portout"`
	// port where to send control command to
	PortCommand uint16 `yaml:"portcommand"`
	// value to add to port for this specific instance (make it easier to have several instance running)
	PortOffset uint16 `yaml:"portoffset"`
	// how to exit tool send this string to stdin to exit, if empty exits by killing process
	ExitCommand string `yaml:"exitcommand"`
	// don't print stdout
	MuteStdOut bool  `yaml:"mutestdout"`
	// push some dummy data on exit
	DummyDataOnExit bool `yaml:"dummydataonexit"`
}

// object to execute an external command tool while piping in and out data with either socket or standard IO
type CommandLineTool struct {
	// internal parameters for the tool
	config CommandLineToolConfig

	// command to call the tool
	tool       exec.Cmd
	pipestdin  io.WriteCloser
	pipestdout io.ReadCloser
	pipestderr io.ReadCloser
	// current connection to send UDP packets if required
	currentinconnection *net.UDPConn
	// current connection to receive UDP packets if required
	currentoutconnection *net.UDPConn
	// current connection to send UDP command if required
	currentcommandconnection *net.UDPConn

	// the golang channel to output MPEG TS Packets
	outChannel MpegTSChannel
}

// ======================== Various handler to process data output
// read std output from tool and print to console (can be used for std err or std out)
func (t *CommandLineTool) handleStdReader(reader io.ReadCloser) {
	bufreader := bufio.NewReader(reader)

	for {
		str, err := bufreader.ReadString('\n')
		if err != nil {
			break
		}
		if (!t.config.MuteStdOut) {
			fmt.Print(str)
		}
	}
}

// handle TS packet coming from std out
func (t *CommandLineTool) handleTSReader() {
	// i := 0
	for {
		readpacket := new(packet.Packet)

		readlen, err := t.pipestdout.Read(readpacket[:])

		if err != nil {
			return
		}

		// check size of read packet
		if readlen != packet.PacketSize {
			// assume that source file contains partial packet, loop so that next read will read EOF
			continue
		}


		//i++
		// forward to output channel	
		if (t.outChannel != nil) {
			//if ((i % 1024) == 0) { log.Print("O") }
			t.outChannel <- *readpacket
		}
	}
}

// handle socket output from tool and send to output channel if present
func (t *CommandLineTool) handleSocketReader() {
	buffer := make([]byte, 1500)

	for {
		packetsize, _, err := t.currentoutconnection.ReadFrom(buffer)
		index := 0
		if err != nil {
			continue
		}

		for packetsize >= packet.PacketSize {
			readpacket := new(packet.Packet)
			copy(readpacket[:], buffer[index:index+packet.PacketSize])
			// forward to output channel	
			if (t.outChannel != nil) {
				t.outChannel <- *readpacket
			}
			packetsize -= packet.PacketSize
			index += packet.PacketSize
		}

		if packetsize != 0 {
			log.Print("residue in UDP")
		}
	}

}

// create a new command line tool object with given configuration
func CreateCommandLineTool(config CommandLineToolConfig) *CommandLineTool {
	var err error
	t := new(CommandLineTool)
	t.config = config

	// create working directory if it does not exists
	if t.config.WorkDir != "" {
		err = os.MkdirAll(t.config.WorkDir, 0660)
		if err != nil {
			log.Printf("cannot create working directory %s for tool %s\n%s", t.config.WorkDir, t.config.Command, err)
		}
	}

	return t
}

// return output channel (create if required)
func (t *CommandLineTool) GetOutputPipe() MpegTSChannel {
	if (t.outChannel == nil) {
		t.outChannel = make(MpegTSChannel)
	}

	return t.outChannel
}

// set an existing output channel
func (t *CommandLineTool) SetOutputPipe(c MpegTSChannel)  {
	t.outChannel = c
}

// link input to an existing channel, will stop processing when channel is closed
func (t *CommandLineTool) SetInputPipe(c MpegTSChannel) {
	// launch async processing of packets from channel
	go func() { 
		// i := 0
		for pkt := range c {
			// i++
			// if ((i % 1024) == 0) { log.Print("I") }
			t.ProcessPacket(pkt)
		}

		// stop tool when channel is closed
		t.Stop()
	} ()
}

// run the tool with given parameters (as a string map)
func (t *CommandLineTool) Start(params map[string]string) error {
	var err error

	args := os.Expand(t.config.Args, func(s string) string {
		switch s {
		case "_portin_":
			return fmt.Sprintf("%d", t.config.PortIn + t.config.PortOffset)
		case "_portout_":
			return fmt.Sprintf("%d", t.config.PortOut + t.config.PortOffset)
		case "_portcommand_":
			return fmt.Sprintf("%d", t.config.PortCommand + t.config.PortOffset)
		case "_workdir_":
			return t.config.WorkDir
		default:
			return params[s]
		}
	})

	// regexep to parse argument string (to isolate quoted string)
	r := regexp.MustCompile("'.+'|\".+\"|\\S+")

	log.Printf("running command %s\nwith args = %s", t.config.Command, args)

	// parse command line into array of strings and create the command wrapper
	t.tool = *exec.Command(t.config.Command, r.FindAllString(args, -1)...) // this will take in account quote around arguments

	// get stdin to flow data
	t.pipestdin, err = t.tool.StdinPipe()

	if err != nil {
		log.Print(err)
	}

	// get output
	t.pipestdout, err = t.tool.StdoutPipe()

	if err != nil {
		return err
	}

	if t.config.PortOut != 0 {
		log.Printf("Read output from UDP port %d", t.config.PortOut + t.config.PortOffset)
		var err error
		listenport := fmt.Sprintf(":%d", t.config.PortOut + t.config.PortOffset )
		//vt.currentconnection, err = net.DialUDP("udp", listenport)
		addr, _ := net.ResolveUDPAddr("udp", listenport)
		t.currentoutconnection, err = net.ListenUDP("udp", addr)
		err = t.currentoutconnection.SetReadBuffer(2 * 1024 * 1024)

		if err != nil {
			return err
		}

		go t.handleSocketReader()

		// dump to console
		go t.handleStdReader(t.pipestdout)

	} else {
		// get date from stdout
		go t.handleTSReader()
	}

	// get error output
	t.pipestderr, err = t.tool.StderrPipe()

	if err != nil {
		return err
	}

	// dump error to console
	go t.handleStdReader(t.pipestderr)

	// if input is configured to use socket create a socket to push data
	if t.config.PortIn != 0 {
		var target net.UDPAddr
		var err error

		target.Port = int(t.config.PortIn + t.config.PortOffset)
		t.currentinconnection, err = net.DialUDP("udp", nil, &target)
		if err != nil {
			log.Printf("cannot open port %d for streaming to tool", t.config.PortIn + t.config.PortOffset)
		}

		err = t.currentinconnection.SetWriteBuffer(1024 * 1024)
	}

	// port to send command to the tool
	if t.config.PortCommand != 0 {
		var target net.UDPAddr
		var err error

		target.Port = int(t.config.PortCommand + t.config.PortOffset)
		t.currentcommandconnection, err = net.DialUDP("udp", nil, &target)
		if err != nil {
			log.Printf("cannot open port %d for command to tool", t.config.PortCommand + t.config.PortOffset)
		}
	}	

	// set directory if present
	if t.config.WorkDir != "" {
		t.tool.Dir = t.config.WorkDir
	}

	// run the tool
	err = t.tool.Start()

	// check if any error while running
	if err != nil {
		return err
	}

	// no error
	return nil
}

// process one packet of data at the input
func (t *CommandLineTool) ProcessPacket(p packet.Packet) {
	if t.currentinconnection != nil {
		t.currentinconnection.Write(p[:])
	} else {
		if t.pipestdin != nil {
			t.pipestdin.Write(p[:])
		}
	}
}

// stop the tool 
func (t *CommandLineTool) Stop() {
	log.Printf("Stopping command %s\n", t.config.Command)
	
	// close all pipes
	if (t.pipestdin != nil) {
		// if an exit command is defined, write it on stdin
		if t.config.ExitCommand != "" {
			if (t.config.PortCommand != 0) {
				log.Printf("Send exit command %s to port %d\n", t.config.Command, t.config.PortCommand + t.config.PortOffset)
				t.currentcommandconnection.Write([]byte(t.config.ExitCommand))
			} else {
				log.Printf("Send exit command %s to stdin\n", t.config.Command)
				t.pipestdin.Write([]byte(t.config.ExitCommand))
			}

			// send a few dummy packets on the TS interface to force processing of exit commmand (required by some tools)
			if (t.config.DummyDataOnExit) {
				log.Printf("Feed empty packets to stdin\n")
				dummypacket := [188]byte{ 0x47, 0x1F, 0xFF, 0x00}

				for i:=0 ; i<128 ; i++ {
					t.ProcessPacket(dummypacket)
				}
			}
		}
	}

	// check if a process has been launched
	if (t.tool.Process != nil) {
		// if no exit command is defined, just kill the process
		if t.config.ExitCommand == "" {
			log.Printf("Force fully kill command\n")
			t.tool.Process.Kill()
			//t.tool.Process.Signal(os.Interrupt)
		}

		log.Printf("Wait for command exit\n")
		// wait for tool to stop
		t.tool.Wait()
	}

	// close all pipes
	if (t.pipestdin != nil) {
		t.pipestdin.Close()
	}
	if (t.pipestdout != nil) {
		t.pipestdout.Close()
	}
	if (t.pipestderr != nil) {
		t.pipestderr.Close()
	}	

	// close existing connections
	if t.currentoutconnection != nil {
		t.currentoutconnection.Close()
	}

	if t.currentinconnection != nil {
		t.currentinconnection.Close()
	}
	
	if t.currentcommandconnection != nil {
		t.currentcommandconnection.Close()
	}

	// close output pipe
	if (t.outChannel != nil) {
		close(t.outChannel)
		t.outChannel = nil
	}

	log.Printf("Command %s stopped\n", t.config.Command)	
} 

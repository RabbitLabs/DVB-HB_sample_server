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

type CommandLineTranscoder struct {
	cmd string
	args string
	port uint16

	transcoder exec.Cmd
	pipestdin  io.WriteCloser
	pipestdout io.ReadCloser
	pipestderr io.ReadCloser
	// current connection to send UDP packets if required
	currentconnection *net.UDPConn
}

func handleReader(reader io.ReadCloser) {
	bufreader := bufio.NewReader(reader)

	for {
		str, err := bufreader.ReadString('\n')
		if err != nil {
			break
		}
		fmt.Print(str)
	}
}

func CreateCommandLineTranscoder(cmd string, args string,  port uint16) *CommandLineTranscoder {
	t := new(CommandLineTranscoder)

	t.cmd = cmd
	t.args = args
	t.port = port

	return t
}

func (t *CommandLineTranscoder) Start(outputdir string) error {
	var err error

	if outputdir != "" {
		err = os.MkdirAll(outputdir, 0660)
		if err != nil {
			log.Printf("cannot create working directory %s for transcoder\n%s", outputdir, err)
		}
	}

	args := os.Expand(t.args, func(s string) string {
		switch s {
		case "port":
			return fmt.Sprintf("%d", t.port)
		case "outputdir":
			return outputdir
		default:
			return ""
		}
	})

	// regexep to parse argument string
	r := regexp.MustCompile("'.+'|\".+\"|\\S+")

//	t.transcoder = *exec.Command(t.cmd, strings.Fields(args)...)  // this will split only by white space
	t.transcoder = *exec.Command(t.cmd, r.FindAllString(args, -1)...) // this will take in account quote around arguments

	// get stdin to flow data
	t.pipestdin, err = t.transcoder.StdinPipe()

	if err != nil {
		log.Print(err)
	}

	// get output
	t.pipestdout, err = t.transcoder.StdoutPipe()

	if err != nil {
		return err
	}

	// dump to console
	go handleReader(t.pipestdout)

	// get error output
	t.pipestderr, err = t.transcoder.StderrPipe()

	if err != nil {
		return err
	}

	// dump to console
	go handleReader(t.pipestderr)

	if t.port != 0 {
		var target net.UDPAddr
		var err error

		target.Port = int(t.port)
		t.currentconnection, err = net.DialUDP("udp", nil, &target)
		if err != nil {
			log.Printf("cannot open port %d for streaming", t.port)
		}

		err = t.currentconnection.SetWriteBuffer(1024 * 1024)
	}

	t.transcoder.Dir = outputdir
	err = t.transcoder.Start()

	if err != nil {
		return err
	}

	return nil
}

func (t *CommandLineTranscoder) ProcessPacket(p packet.Packet) {
	if t.currentconnection != nil {
		t.currentconnection.Write(p[:])
	} else {
		if (t.pipestdin != nil) {
			t.pipestdin.Write(p[:])
		}
	}
}

func (t *CommandLineTranscoder) Stop() {
	t.pipestdin.Write([]byte{'q'} )
	t.pipestdin.Close()
	t.pipestdout.Close()
	t.pipestderr.Close()
	//t.transcoder.Process.Kill()
	t.transcoder.Wait()
	if t.currentconnection != nil {
		t.currentconnection.Close()
	}
}

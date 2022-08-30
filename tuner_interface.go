package main

import (
	"github.com/Comcast/gots/packet"
)

type MpegTSChannel chan packet.Packet

type Tuner interface {
	// get the channel to receive MPEG TS Packets
	GetChannel() MpegTSChannel
	// tune to a TS, true if OK
	Tune( parameters string) bool
	// stop TS
	Stop()
	// start a frequency scan, return tune string or empty on failure
	StartScan() string
	// go to next frequency during a scan, return tune string or empty on failure
	ScanNext() string
}

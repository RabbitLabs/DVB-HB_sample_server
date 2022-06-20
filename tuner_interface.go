package main

import (
	"github.com/Comcast/gots/packet"
)

type TunerChannel chan packet.Packet

type Tuner interface {
	// get the channel to receive MPEG TS Packets
	GetChannel() TunerChannel;
	// tune to a TS, true if OK
	Tune( parameters string) bool;
	// start a frequency scan, return tune string or empty on failure
	StartScan() string;
	// go to next frequency during a scan, return tune string or empty on failure
	ScanNext() string;
}

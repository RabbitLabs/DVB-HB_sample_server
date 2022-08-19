package main

import (
	"github.com/Comcast/gots/packet"
)

// interface to parse a packet
type MpegPIDParser interface {
	ParsePacket(pkt packet.Packet)
}

// parsing 
type MpegPIDPSIParser struct {
}




type MpegDemux struct {
}
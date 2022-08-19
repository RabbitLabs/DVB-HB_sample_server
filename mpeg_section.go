package main

import (
	"github.com/Comcast/gots/packet"
)

// this object can rebuild MPEG section from MPEG packets
type MpegSectionReconstructor struct {
	currentsize int
	data []byte
}

// create new section rebuilder (max size is usually 4096 or 1024 for public syntax SI)
func NewMpegSectionReconstructor(maxsectionsize int) *MpegSectionReconstructor{
	msr := new(MpegSectionReconstructor)

	msr.currentsize = 0
	msr.data = make([]byte, maxsectionsize)
	return msr
}

func (msr *MpegSectionReconstructor) ParsePacket(pkt *packet.Packet) {
	if (msr.currentsize != 0) {

	} else {
		if (packet.PayloadUnitStartIndicator(pkt)) {
			//payload := packet.Payload(pkt)



		}	
	}

}


// get the reconstructed section
func (msr *MpegSectionReconstructor) GetSection() []byte {
	if msr.currentsize == 0 {
		return msr.data[0:0] // return empty slice
	}

	return msr.data[0:msr.currentsize-1] // return slice with section data
}
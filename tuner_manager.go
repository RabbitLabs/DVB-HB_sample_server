package main

import (
	"log"
)

type TunerManager struct {
	Name          string
	Tuners        []Tuner
	outputchannel MpegTSChannel
}

func NewTunerManager(name string) *TunerManager {
	tm := new(TunerManager)
	tm.Name = name

	return tm
}

func (tm *TunerManager) AttachTuner(tuner Tuner) {
	tm.Tuners = append(tm.Tuners, tuner)
	go tm.ReceivePackets(tuner.GetChannel())
}

func (tm *TunerManager) ReceivePackets(tc MpegTSChannel) {
	for pkt := range tc {
		pid := pkt.PID()
		switch pid {
		case 0:
			//log.Print("PAT")
			break
		default:
		}

		// forward data
		if tm.outputchannel != nil {
			tm.outputchannel <- pkt
		}
	}

	log.Print("Tunre Manager exit receive loop")
}

func (tm *TunerManager) GetChannel() MpegTSChannel {
	if tm.outputchannel == nil {
		tm.outputchannel = make(MpegTSChannel , 128)
	}

	return tm.outputchannel
}

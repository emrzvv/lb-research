package metric

type RequestRTT struct {
	ServerID int
	RTT      float64
	When     float64
}

var RTTChan chan RequestRTT

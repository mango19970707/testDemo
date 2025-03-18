package main

import (
	"testDemo/xdp/receive"
	"testDemo/xdp/send"
)

func main() {
	eno3Receiver := receive.HTTPReceiver{
		"eno3",
		1,
	}
	eno4Sender := send.HTTPSender{
		"eno4",
		1,
	}

	eno4Receiver := receive.HTTPReceiver{
		"eno4",
		1,
	}
	eno3Sender := send.HTTPSender{
		"eno3",
		1,
	}

	reqChan := make(chan []byte, 10000)
	respChan := make(chan []byte, 10000)

	// eno3 -> xdp -> eno4
	go eno3Receiver.Receive(reqChan)
	go eno4Sender.Send(reqChan)

	// eno4 -> xdp -> eno3
	go eno4Receiver.Receive(respChan)
	go eno3Sender.Send(respChan)
	select {}
}

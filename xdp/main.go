package main

import (
	"testDemo/xdp/receive"
	"testDemo/xdp/send"
)

func main() {
	receiver := receive.HTTPReceiver{}
	sender := send.HTTPSender{}
	go receiver.Receive()
	sender.Send()
	select {}
}

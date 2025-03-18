package send

import (
	"github.com/slavc/xdp"
	"github.com/vishvananda/netlink"
	"log"
)

type HTTPSender struct {
	NicName  string
	QueueNum int
}

func (s *HTTPSender) Send(msgChan <-chan []byte) error {
	link, err := netlink.LinkByName(s.NicName)
	if err != nil {
		return err
	}

	// 创建XDP程序（使用默认ebpf程序），无自定义内核代码
	program, err := xdp.NewProgram(s.QueueNum)
	if err != nil {
		return err
	}
	defer program.Close()

	// 附加到网络接口
	if err = program.Attach(link.Attrs().Index); err != nil {
		return err
	}
	defer program.Detach(link.Attrs().Index)

	xsks := make([]*xdp.Socket, s.QueueNum)
	for i := 0; i < len(xsks); i++ {
		if xsks[i], err = xdp.NewSocket(link.Attrs().Index, i, nil); err != nil {
			return err
		}
		defer xsks[i].Close()
		idx := i
		go transmit(xsks[idx], msgChan)
	}
	return nil
}

func transmit(xsk *xdp.Socket, msgChan <-chan []byte) {
	pos := 0
	batch := 1
	packets := make([][]byte, batch)
	for {
		if pos < batch {
			log.Print("send message: ")
			packets[pos] = <-msgChan
			pos++
			continue
		}
		descs := xsk.GetDescs(batch, false)
		if len(descs) == 0 {
			xsk.Poll(-1)
			continue
		}

		frameLen := 0
		for j := 0; j < len(descs); j++ {
			frameLen = copy(xsk.GetFrame(descs[j]), packets[j])
			descs[j].Len = uint32(frameLen)
		}
		xsk.Transmit(descs)
		if _, _, err := xsk.Poll(-1); err != nil {
			continue
		}

		for i := 0; i < len(descs) && i+len(descs) < batch; i++ {
			packets[i] = packets[i+len(descs)]
		}
		pos = batch - len(descs)
	}
}

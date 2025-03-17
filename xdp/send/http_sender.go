package send

import (
	"fmt"
	"github.com/slavc/xdp"
	"github.com/vishvananda/netlink"
	_chan "testDemo/xdp/chan"
)

var (
	nicName     string
	xdpQueueNum int
)

func init() {
	nicName = "eno4"
	xdpQueueNum = 1
}

type HTTPSender struct {
}

func (s *HTTPSender) Send() {
	_, err := initXdpSocket()
	if err != nil {
		fmt.Println("Fail to send:", err)
		return
	}
}

func initXdpSocket() ([]*xdp.Socket, error) {
	link, err := netlink.LinkByName(nicName)
	if err != nil {
		return nil, err
	}

	// 创建XDP程序（使用默认ebpf程序），无自定义内核代码
	program, err := xdp.NewProgram(xdpQueueNum)
	if err != nil {
		return nil, err
	}
	defer program.Close()

	// 附加到网络接口
	if err = program.Attach(link.Attrs().Index); err != nil {
		return nil, err
	}
	defer program.Detach(link.Attrs().Index)

	xsks := make([]*xdp.Socket, xdpQueueNum)
	for i := 0; i < len(xsks); i++ {
		if xsks[i], err = xdp.NewSocket(link.Attrs().Index, i, nil); err != nil {
			return nil, err
		}
		defer xsks[i].Close()
		idx := i
		go transmit(xsks[idx])
	}
	return xsks, nil
}

func transmit(xsk *xdp.Socket) {
	pos := 0
	batch := 64
	packets := make([][]byte, batch)
	for {
		if pos < batch {
			packets[pos] = <-_chan.HTTPChan
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

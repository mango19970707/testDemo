package main

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/safchain/ethtool"
	"github.com/slavc/xdp"
	"github.com/vishvananda/netlink"
)

func main() {
	var clientNic, serverNic nicInfo
	if err := clientNic.New("enp26s0f0", 1); err != nil {
		return
	}
	if err := serverNic.New("enp26s0f1", 1); err != nil {
		return
	}
	client2ServerChan := make(chan []byte, 10000)
	server2ClientChan := make(chan []byte, 10000)

	clientNic.receive(client2ServerChan)
	clientNic.send(server2ClientChan)

	serverNic.receive(server2ClientChan)
	serverNic.send(client2ServerChan)

	termChan := make(chan os.Signal)
	// 监听退出信号
	signal.Notify(termChan, syscall.SIGINT, syscall.SIGTERM)
	<-termChan

	clientNic.Program.Close()
	for i := range clientNic.socketList {
		clientNic.socketList[i].Close()
	}
	serverNic.Program.Close()
	for i := range serverNic.socketList {
		serverNic.socketList[i].Close()
	}

	fmt.Println("exit xdp program.")
}

type nicInfo struct {
	NicName    string
	QueueNum   int
	Program    *xdp.Program
	socketList []*xdp.Socket
}

func (n *nicInfo) New(nicName string, queueNum int) error {
	if len(nicName) == 0 || queueNum == 0 {
		return errors.New("XDP mode is not enabled")
	}

	n.NicName = nicName
	n.QueueNum = queueNum

	link, err := netlink.LinkByName(nicName)
	if err != nil {
		return err
	}

	// 设置MTU为3040, XDP限制
	if err = netlink.LinkSetMTU(link, 3040); err != nil {
		return err
	}

	// 开启混杂模式
	if err = netlink.SetPromiscOn(link); err != nil {
		return err
	}
	defer netlink.SetPromiscOff(link)

	// 在设置MTU之后添加通道数设置逻辑
	ethTool, err := ethtool.NewEthtool()
	if err != nil {
		return err
	}
	defer ethTool.Close()

	// 获取当前通道配置
	channels, err := ethTool.GetChannels(n.NicName)
	if err != nil {
		return err
	}

	// 只修改Combined通道数（XDP通常使用Combined队列）
	channels.MaxCombined = uint32(queueNum)

	// 设置新的通道配置
	if _, err = ethTool.SetChannels(nicName, channels); err != nil {
		return err
	}

	// 创建XDP程序（使用默认ebpf程序），无自定义内核代码
	program, err := xdp.NewProgram(queueNum)
	if err != nil {
		return err
	}
	//defer program.Close()

	// 附加到网络接口
	if err = program.Attach(link.Attrs().Index); err != nil {
		return err
	}
	defer program.Detach(link.Attrs().Index)

	// 统一创建所有队列的socket
	n.socketList = make([]*xdp.Socket, queueNum)
	for qid := 0; qid < len(n.socketList); qid++ {
		xsk, err := xdp.NewSocket(link.Attrs().Index, qid, nil)
		if err != nil {
			return err
		}
		//defer xsk.Close()

		if err = program.Register(qid, xsk.FD()); err != nil {
			return err
		}
		defer program.Unregister(qid)
		n.socketList[qid] = xsk
	}

	return nil
}

func (n *nicInfo) receive(msgChan chan<- []byte) {
	for qid, xsk := range n.socketList {
		go func(qid int, xsk *xdp.Socket) {

			for {
				// If there are any free slots on the Fill queue...
				if n := xsk.NumFreeFillSlots(); n > 0 {
					// ...then fetch up to that number of not-in-use
					// descriptors and push them onto the Fill ring queue
					// for the kernel to fill them with the received
					// frames.
					xsk.Fill(xsk.GetDescs(n, true))
				}

				// 移除调试日志减少IO开销
				// 使用非阻塞Poll(0) + 忙等待（根据实际需求调整）
				numRx, _, err := xsk.Poll(0)
				if err != nil {
					fmt.Printf("error: %v\n", err)
					return
				}

				if numRx > 0 {
					// 批量获取描述符长度（避免逐个处理）
					rxDescs := xsk.Receive(numRx)

					for i := 0; i < len(rxDescs); i++ {
						pktData := xsk.GetFrame(rxDescs[i])
						fmt.Println("receive message: ", string(pktData))
						msgChan <- bytes.Clone(pktData)
					}
				}
			}
		}(qid, xsk)
	}
}

func (n *nicInfo) send(msgChan <-chan []byte) {
	for i := 0; i < len(n.socketList); i++ {
		go transmit(n.socketList[i], msgChan)
	}
}

func transmit(xsk *xdp.Socket, msgChan <-chan []byte) {
	pos := 0
	batch := 1
	packets := make([][]byte, batch)
	for {
		if pos < batch {
			fmt.Println("send message: ")
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

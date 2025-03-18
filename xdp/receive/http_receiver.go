package receive

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/safchain/ethtool"
	"github.com/slavc/xdp"
	"github.com/vishvananda/netlink"
	"log"
)

type HTTPReceiver struct {
	NicName  string
	QueueNum int
}

func (r *HTTPReceiver) Receive(msgChan chan<- []byte) error {
	// 通过配置XDP对列数量开启，没有网卡时不开启。
	if len(r.NicName) == 0 || r.QueueNum == 0 {
		return errors.New("XDP mode is not enabled")
	}
	// TODO: 当前XDP只支持一个网卡，后续XDP以透明模式为主。配置一个网卡就是流量监听，如果配置两个就是相互转发
	link, err := netlink.LinkByName(r.NicName)
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
	channels, err := ethTool.GetChannels(r.NicName)
	if err != nil {
		return err
	}

	// 只修改Combined通道数（XDP通常使用Combined队列）
	channels.MaxCombined = uint32(r.QueueNum)

	// 设置新的通道配置
	if _, err = ethTool.SetChannels(r.NicName, channels); err != nil {
		return err
	}

	// 创建XDP程序（使用默认ebpf程序），无自定义内核代码
	program, err := xdp.NewProgram(r.QueueNum)
	if err != nil {
		return err
	}
	defer program.Close()

	// 附加到网络接口
	if err = program.Attach(link.Attrs().Index); err != nil {
		return err
	}
	defer program.Detach(link.Attrs().Index)

	// 统一创建所有队列的socket
	xskList := make([]*xdp.Socket, r.QueueNum)
	for qid := 0; qid < r.QueueNum; qid++ {
		xsk, err := xdp.NewSocket(link.Attrs().Index, qid, nil)
		if err != nil {
			return err
		}
		defer xsk.Close()

		if err = program.Register(qid, xsk.FD()); err != nil {
			return err
		}
		defer program.Unregister(qid)
		xskList[qid] = xsk
	}

	// 启动处理协程
	for qid, xsk := range xskList {
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
						log.Print("receive message: ", string(pktData))
						msgChan <- bytes.Clone(pktData)
					}
				}
			}
		}(qid, xsk)
	}

	return nil
}

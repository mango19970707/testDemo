package receive

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/safchain/ethtool"
	"github.com/slavc/xdp"
	"github.com/vishvananda/netlink"
	_chan "testDemo/xdp/chan"
	"time"
)

type HTTPReceiver struct {
}

func (r *HTTPReceiver) Receive() {

}

var (
	nicName     string
	xdpQueueNum int
)

func init() {
	nicName = "eno3"
	xdpQueueNum = 1
}

func RestartXdp() (err error) {
	// 通过配置XDP对列数量开启，没有网卡时不开启。
	if len(nicName) == 0 || xdpQueueNum == 0 {
		return errors.New("XDP mode is not enabled")
	}
	queueNum := xdpQueueNum
	// TODO: 当前XDP只支持一个网卡，后续XDP以透明模式为主。配置一个网卡就是流量监听，如果配置两个就是相互转发
	link, err := netlink.LinkByName(nicName)
	if err != nil {
		return
	}

	// 设置MTU为3040, XDP限制
	if err = netlink.LinkSetMTU(link, 3040); err != nil {
		return
	}

	// 开启混杂模式
	if err = netlink.SetPromiscOn(link); err != nil {
		return
	}
	defer netlink.SetPromiscOff(link)

	// 在设置MTU之后添加通道数设置逻辑
	ethTool, err := ethtool.NewEthtool()
	if err != nil {
		return
	}
	defer ethTool.Close()

	// 获取当前通道配置
	channels, err := ethTool.GetChannels(nicName)
	if err != nil {
		return
	}

	// 只修改Combined通道数（XDP通常使用Combined队列）
	channels.MaxCombined = uint32(xdpQueueNum)

	// 设置新的通道配置
	if _, err = ethTool.SetChannels(nicName, channels); err != nil {
		return err
	}

	// 创建XDP程序（使用默认ebpf程序），无自定义内核代码
	program, err := xdp.NewProgram(queueNum)
	if err != nil {
		return
	}
	defer program.Close()

	// 附加到网络接口
	if err = program.Attach(link.Attrs().Index); err != nil {
		return
	}
	defer program.Detach(link.Attrs().Index)

	// 统一创建所有队列的socket
	xskList := make([]*xdp.Socket, queueNum)
	for qid := 0; qid < queueNum; qid++ {
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
						_chan.HTTPChan <- bytes.Clone(pktData)
					}
				}
			}
		}(qid, xsk)
	}

	// 主线程统计输出
	ticker := time.NewTicker(time.Second * 5)
	defer ticker.Stop()
	return
}

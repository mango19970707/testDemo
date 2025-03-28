package main

import (
	"bytes"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/safchain/ethtool"
	"github.com/slavc/xdp"
	"github.com/vishvananda/netlink"
)

func main() {
	var clientNic, serverNic nicInfo
	if err := clientNic.New("enp26s0f0", 1); err != nil {
		fmt.Printf("Failed to initialize client NIC: %v\n", err)
		return
	}
	if err := serverNic.New("enp26s0f1", 1); err != nil {
		fmt.Printf("Failed to initialize server NIC: %v\n", err)
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

	// 获取网卡信息
	link, err := netlink.LinkByName(nicName)
	if err != nil {
		return fmt.Errorf("failed to get link by name: %v", err)
	}

	// 设置MTU为3040, XDP限制
	if err = netlink.LinkSetMTU(link, 3040); err != nil {
		return fmt.Errorf("failed to set MTU: %v", err)
	}
	fmt.Printf("Successfully set MTU for %s to 3040\n", nicName)

	// 开启混杂模式
	if err = netlink.SetPromiscOn(link); err != nil {
		return fmt.Errorf("failed to set promiscuous mode: %v", err)
	}
	fmt.Printf("Successfully set promiscuous mode for %s\n", nicName)
	defer func() {
		if err := netlink.SetPromiscOff(link); err != nil {
			fmt.Printf("Failed to turn off promiscuous mode: %v\n", err)
		}
	}()

	// 设置通道数
	ethTool, err := ethtool.NewEthtool()
	if err != nil {
		return fmt.Errorf("failed to create ethtool: %v", err)
	}
	defer ethTool.Close()

	channels, err := ethTool.GetChannels(n.NicName)
	if err != nil {
		return fmt.Errorf("failed to get channels: %v", err)
	}

	// 修改Combined通道数
	channels.MaxCombined = uint32(queueNum)
	if _, err := ethTool.SetChannels(nicName, channels); err != nil {
		fmt.Printf("Setting combined channels to %d for %s...\n", queueNum, nicName)
	}
	fmt.Printf("Successfully set combined channels to %d for %s\n", queueNum, nicName)

	// 创建XDP程序（使用默认ebpf程序）
	program, err := xdp.NewProgram(queueNum)
	if err != nil {
		return fmt.Errorf("failed to create XDP program: %v", err)
	}
	n.Program = program
	defer func() {
		if err := program.Close(); err != nil {
			fmt.Printf("Failed to close XDP program: %v\n", err)
		}
	}()

	// 附加XDP程序到网卡
	if err = program.Attach(link.Attrs().Index); err != nil {
		return fmt.Errorf("failed to attach XDP program to %s: %v", nicName, err)
	}
	fmt.Printf("Successfully attached XDP program to %s\n", nicName)

	// 轮询检查 XDP 是否真正生效
	for i := 0; i < 10; i++ {
		if checkXDPAttached(nicName) {
			fmt.Printf("XDP successfully attached to %s\n", nicName)
			break
		}
		time.Sleep(100 * time.Millisecond) // 每 100ms 检查一次
	}

	// 创建XDP套接字
	n.socketList = make([]*xdp.Socket, queueNum)
	for qid := 0; qid < len(n.socketList); qid++ {
		xsk, err := xdp.NewSocket(link.Attrs().Index, qid, nil)
		if err != nil {
			return fmt.Errorf("failed to create XDP socket for queue %d: %v", qid, err)
		}

		if err = program.Register(qid, xsk.FD()); err != nil {
			return fmt.Errorf("failed to register socket with XDP program: %v", err)
		}
		n.socketList[qid] = xsk
		fmt.Printf("Successfully created and registered XDP socket for queue %d\n", qid)
	}

	return nil
}

func checkXDPAttached(nicName string) bool {
	link, err := netlink.LinkByName(nicName)
	if err != nil {
		fmt.Printf("Failed to get link info for %s: %v\n", nicName, err)
		return false
	}
	return link.Attrs().Xdp != nil // 只要 XDP 不是 nil，就说明附加成功
}

func (n *nicInfo) receive(msgChan chan<- []byte) {
	for qid, xsk := range n.socketList {
		go func(qid int, xsk *xdp.Socket) {
			for {
				// 如果Fill队列有空闲位置
				if n := xsk.NumFreeFillSlots(); n > 0 {
					// 获取空闲描述符并将其推送到Fill队列
					xsk.Fill(xsk.GetDescs(n, true))
				}

				// 使用非阻塞Poll(0)来避免阻塞
				numRx, _, err := xsk.Poll(0)
				if err != nil {
					fmt.Printf("Poll error on queue %d: %v\n", qid, err)
					return
				}

				if numRx > 0 {
					// 获取接收到的描述符
					rxDescs := xsk.Receive(numRx)

					for i := 0; i < len(rxDescs); i++ {
						pktData := xsk.GetFrame(rxDescs[i])
						msgStr, _ := hex.DecodeString(string(pktData))
						fmt.Printf("%s received: %s\n", n.NicName, msgStr)
						msgChan <- bytes.Clone(pktData)
					}
				}
			}
		}(qid, xsk)
	}
}

func (n *nicInfo) send(msgChan <-chan []byte) {
	for i := 0; i < len(n.socketList); i++ {
		go func(xsk *xdp.Socket, msgChan <-chan []byte) {
			pos := 0
			batch := 1
			packets := make([][]byte, batch)
			for {
				if pos < batch {
					msg := <-msgChan
					msgStr, _ := hex.DecodeString(string(msg))
					fmt.Printf("%s send %s\n", n.NicName, msgStr)
					packets[pos] = msg
					pos++
					continue
				}
				descs := xsk.GetDescs(batch, false)
				if len(descs) == 0 {
					fmt.Println("No available descriptors, waiting for poll...")
					xsk.Poll(-1)
					continue
				}

				// 填充数据并发送
				frameLen := 0
				for j := 0; j < len(descs); j++ {
					frameLen = copy(xsk.GetFrame(descs[j]), packets[j])
					descs[j].Len = uint32(frameLen)
				}
				xsk.Transmit(descs)
				if _, _, err := xsk.Poll(-1); err != nil {
					fmt.Printf("Error during transmit poll: %v\n", err)
					continue
				}

				for i := 0; i < len(descs) && i+len(descs) < batch; i++ {
					packets[i] = packets[i+len(descs)]
				}
				pos = batch - len(descs)
			}
		}(n.socketList[i], msgChan)
	}
}

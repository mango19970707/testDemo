package main

import (
	"fmt"
	"testDemo/xdp/receive"
)

func main() {
	go func() {
		if err := receive.RestartXdp(); err != nil {
			fmt.Println(err)
		}
	}()
}

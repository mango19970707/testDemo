package main

import (
	"fmt"
	"time"
)

func main() {
	for i := 0; i < 3; i++ {
		ch[i] = make(chan struct{}, 1)
		go f(i)
	}
	ch[0] <- struct{}{}
	select {}
}

var ch = make([]chan struct{}, 3)

func f(i int) {
	for {
		<-ch[i%3]
		fmt.Println(i)
		time.Sleep(500 * time.Millisecond)
		ch[(i%3+1)%3] <- struct{}{}
	}
}

//func f1() {
//	for {
//		<-ch[1]
//		fmt.Println("1")
//		time.Sleep(100 * time.Millisecond)
//		ch[2] <- struct{}{}
//	}
//}
//
//func f2() {
//	for {
//		<-ch[2]
//		fmt.Println("2")
//		time.Sleep(100 * time.Millisecond)
//		ch[0] <- struct{}{}
//	}
//}

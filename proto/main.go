package main

import (
	"bytes"
	"fmt"
	"github.com/buger/goreplay/proto"
)

func main() {
	data := []byte("GET /eagle/unknown/req HTTP/1.1\r\nx-sw-eagle: invalid-gor-data\r\nConte\r\n\r\n123124")
	data = proto.SetHeader(data, []byte("test"), []byte("123"))
	data = FixHeader(data)
	fmt.Println(string(data))
}

func FixHeader(payload []byte) []byte {
	crlfIndex := bytes.Index(payload, []byte("\r\n\r\n"))
	var lastHeaderStart, lastHeaderEnd int
	if crlfIndex != -1 {
		if lastHeaderIndex := bytes.LastIndex(payload[:crlfIndex], []byte("\r\n")); lastHeaderIndex != -1 {
			lastHeaderStart = lastHeaderIndex
			lastHeaderEnd = crlfIndex
		}
	} else {
		if lastHeaderIndex := bytes.LastIndex(payload, []byte("\r\n")); lastHeaderIndex != -1 {
			lastHeaderStart = lastHeaderIndex
			lastHeaderEnd = len(payload)
		}
	}
	if lastHeaderEnd-lastHeaderStart == 0 {
		return payload
	}
	if !bytes.Contains(payload[lastHeaderStart:lastHeaderEnd], []byte(":")) {
		payload = append(payload[:lastHeaderStart], payload[lastHeaderEnd:]...)
	}
	return payload
}

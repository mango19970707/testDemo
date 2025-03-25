package main

import (
	"bytes"
	"fmt"
	"github.com/buger/goreplay/proto"
)

func main() {
	data := []byte("GET /eagle/unknown/req HTTP/1.1\r\nx-sw-eagle: invalid-gor-data\r\nConte: 123\n234\r\n\r\nbody")
	data = proto.SetHeader(data, []byte("test"), []byte("123"))
	//data = FixHeader(data)
	//fmt.Println(string(data))
	data = handleIncompleteHeader(data)
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

func fixHeader2(payload []byte) {
	start := proto.MIMEHeadersStartPos(payload)
	end := proto.MIMEHeadersEndPos(payload)
	if end == -1 {
		end = len(payload)
	}
	fmt.Println(string(payload[start:end]))
}

func handleIncompleteHeader(payload []byte) []byte {
	var headerStart, headerEnd int
	headerStart = proto.MIMEHeadersStartPos(payload)
	if headerEnd = proto.MIMEHeadersEndPos(payload) - 4; headerEnd < 0 {
		headerEnd = len(payload)
	}

	headers := bytes.Split(payload, []byte("\r\n"))
	buf := bytes.Buffer{}
	buf.Write(payload[:headerStart])
	for _, header := range headers {
		if kv := bytes.SplitN(header, []byte(":"), 2); len(kv) == 2 {
			kv[0] = bytes.Trim(kv[0], "\n")
			kv[1] = bytes.ReplaceAll(bytes.TrimLeft(kv[1], " "), []byte("\n"), []byte(" "))
			buf.Write(kv[0])
			buf.Write([]byte(": "))
			buf.Write(kv[1])
			buf.Write([]byte("\r\n"))
		}
	}
	if len(payload) > headerEnd {
		buf.Write(payload[headerEnd:])
	}

	return buf.Bytes()
}

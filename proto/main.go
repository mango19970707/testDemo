package main

import (
	"bytes"
	"fmt"
	"github.com/buger/goreplay/proto"
	"strconv"
)

func main() {
	data := []byte("GET /eagle/unknown/req HTTP/1.1\r\nx-sw-eagle: invalid-gor-data\r\nConte234: 123\r\n")
	data = proto.SetHeader(data, []byte("test"), []byte("123"))
	//data = FixHeader(data)
	//fmt.Println(string(data))
	data = FixContentLength(data)
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
	if headerStart = proto.MIMEHeadersStartPos(payload); headerStart <= len(payload) {
		return payload
	}
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

func FixContentLength(http []byte) []byte {
	// 修正报文Content-Length
	contentLenStr := proto.Header(http, []byte("Content-Length"))
	bodyStart := bytes.Index(http, []byte("\r\n\r\n")) + 4
	// 处理找不到分隔符的情况
	if bodyStart == 3 { // 原逻辑中找不到时返回-1 +4 = 3
		// 补充报文头结束标记
		http = append(http, []byte("\r\n\r\n")...)
		bodyStart = len(http) // 新追加的位置
	}

	if lastHeaderIndex := bytes.LastIndex(http[:bodyStart-4], []byte("\r\n")); lastHeaderIndex != -1 {
		lastHeaderStart := lastHeaderIndex
		lastHeaderEnd := bodyStart - 4
		if !bytes.Contains(http[lastHeaderStart:lastHeaderEnd], []byte(":")) {
			http = append(http[:lastHeaderStart], http[lastHeaderEnd:]...)
			bodyStart = bodyStart - lastHeaderEnd + lastHeaderStart
		}
	}

	// Content-length 不匹配: 1.chunked, 2. 报文不全有content-length, 3. 非前两种但有Body
	if len(contentLenStr) == 0 && len(proto.Header(http, []byte("Transfer-Encoding"))) > 0 {
		// 将当前请求的body部分传递给CheckChunked进行验证
		body := http[bodyStart:]
		if chunkEnd, fullChunked := CheckChunked(body); !fullChunked {
			// 裁剪不完整的chunk结尾,补充终止chunk标记
			body = append(body[:chunkEnd], []byte("0\r\n\r\n")...)
			// 更新Body
			http = append(http[:bodyStart], body...)
		}
	} else {
		contentLen, _ := strconv.Atoi(string(contentLenStr))
		bodySize := len(http) - bodyStart
		if bodySize != contentLen || len(contentLenStr) == 0 {
			http = proto.SetHeader(http, []byte("Content-Length"), []byte(strconv.Itoa(bodySize)))
		}
	}

	return http
}

func CheckChunked(bufs ...[]byte) (chunkEnd int, full bool) {
	var buf []byte
	if len(bufs) > 0 {
		buf = bufs[0]
	}

	for chunkEnd < len(buf) {
		sz := bytes.IndexByte(buf[chunkEnd:], '\r')
		if sz < 1 {
			break
		}
		// don't parse chunk extensions https://github.com/golang/go/issues/13135.
		// chunks extensions are no longer a thing, but we do check if the byte
		// following the parsed hex number is ';'
		sz += chunkEnd
		chkLen, ok := atoI(buf[chunkEnd:sz], 16)
		if !ok && bytes.IndexByte(buf[chunkEnd:sz], ';') < 1 || chkLen < 0 {
			break
		}
		sz++ // + '\n'
		// total length = SIZE + CRLF + OCTETS + CRLF
		allChunk := sz + chkLen + 2
		if allChunk >= len(buf) ||
			buf[sz]&buf[allChunk] != '\n' ||
			buf[allChunk-1] != '\r' {
			break
		}
		chunkEnd = allChunk + 1
		if chkLen == 0 {
			full = true
			break
		}
	}

	return chunkEnd, full
}

func atoI(s []byte, base int) (num int, ok bool) {
	var v int
	ok = true
	for i := 0; i < len(s); i++ {
		if s[i] > 127 {
			ok = false
			break
		}
		v = int(hexTable[s[i]])
		if v >= base || (v == 0 && s[i] != '0') {
			ok = false
			break
		}
		num = (num * base) + v
	}
	return
}

var hexTable = [128]byte{
	'0': 0,
	'1': 1,
	'2': 2,
	'3': 3,
	'4': 4,
	'5': 5,
	'6': 6,
	'7': 7,
	'8': 8,
	'9': 9,
	'A': 10,
	'a': 10,
	'B': 11,
	'b': 11,
	'C': 12,
	'c': 12,
	'D': 13,
	'd': 13,
	'E': 14,
	'e': 14,
	'F': 15,
	'f': 15,
}

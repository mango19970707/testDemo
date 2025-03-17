package main

import (
	"bufio"
	"bytes"
	"fmt"
	"github.com/valyala/fasthttp"
)

// SerializeRequest serializes a fasthttp.Request into a byte slice.
func SerializeRequest(req *fasthttp.Request) ([]byte, error) {
	var buf bytes.Buffer
	writer := bufio.NewWriter(&buf)
	if _, err := req.WriteTo(writer); err != nil {
		return nil, err
	}
	writer.Flush() // Ensure all data is written to the buffer
	return buf.Bytes(), nil
}

// DeserializeRequest deserializes a byte slice into a fasthttp.Request.
func DeserializeRequest(data []byte) (*fasthttp.Request, error) {
	req := fasthttp.AcquireRequest()
	if err := req.Read(bufio.NewReader(bytes.NewReader(data))); err != nil {
		return nil, err
	}
	return req, nil
}

func main() {
	// Create a new request
	originalReq := fasthttp.AcquireRequest()
	originalReq.SetRequestURI("http://example.com")
	originalReq.Header.SetMethod("GET")
	originalReq.Header.Set("Custom-Header", "value")
	originalReq.SetBody([]byte("request body"))

	// Serialize the request
	serializedData, err := SerializeRequest(originalReq)
	if err != nil {
		fmt.Println("Error serializing request:", err)
		return
	}

	// Deserialize the request
	deserializedReq, err := DeserializeRequest(serializedData)
	if err != nil {
		fmt.Println("Error deserializing request:", err)
		return
	}

	// Print the deserialized request details
	fmt.Println("Deserialized Request URI:", string(deserializedReq.URI().FullURI()))
	fmt.Println("Deserialized Request Method:", string(deserializedReq.Header.Method()))
	fmt.Println("Deserialized Request Header:", string(deserializedReq.Header.Peek("Custom-Header")))
	fmt.Println("Deserialized Request Body:", string(deserializedReq.Body()))

	// Release the request objects
	fasthttp.ReleaseRequest(originalReq)
	fasthttp.ReleaseRequest(deserializedReq)
}

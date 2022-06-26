package rtsp

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// https://tools.ietf.org/html/rfc2326#page-19
func readRequest(r io.Reader) (*Request, error) {
	req := new(Request)
	buf := bufio.NewReader(r)
	headers := make(map[string]string)

	// first line of the request will be the request line
	requestLine, err := buf.ReadString('\n')
	if err != nil {
		return nil, err
	}
	requestLine = strings.Trim(requestLine, "\r\n")
	requestLineParts := strings.Split(requestLine, " ")

	if len(requestLineParts) != 3 {
		return nil, fmt.Errorf("improperly formatted request line: %s", requestLine)
	}

	if req.Method, err = getMethod(requestLineParts[0]); err != nil {
		return nil, fmt.Errorf("method does exist in RTSP protocol: %s", requestLineParts[0])
	}

	req.RequestURI = requestLineParts[1]
	req.protocol = requestLineParts[2]

	// now we can read the headers.
	// we read a line until we hit the empty line
	// which indicates all the headers have been processed
	for {
		headerField, err := buf.ReadString('\n')
		if err != nil {
			return nil, err
		}
		headerField = strings.Trim(headerField, "\r\n")
		if strings.Trim(headerField, "\r\n") == "" {
			break
		}
		headerParts := strings.Split(headerField, ":")
		if len(headerParts) < 2 {
			return nil, fmt.Errorf("improper header: %s", headerField)
		}
		headers[strings.TrimSpace(headerParts[0])] = strings.TrimSpace(headerParts[1])
	}

	req.Headers = headers

	contentLength, hasBody := req.Headers["Content-Length"]
	if !hasBody {
		return req, nil
	}

	// now read the body
	length, _ := strconv.Atoi(contentLength)
	req.Body = make([]byte, length)

	// makes sure we read the full length of the content
	if _, err := io.ReadFull(buf, req.Body); err != nil {
		if !errors.Is(err, io.ErrUnexpectedEOF) && !errors.Is(err, io.EOF) {
			return nil, fmt.Errorf("error reading full body: %w", err)
		}
	}

	return req, nil
}

// TODO: writeResponse and writeRequest look very similar....
func writeResponse(w io.Writer, resp *Response) (n int, err error) {
	var buffer bytes.Buffer
	buffer.WriteString(fmt.Sprintf("%s %d %s\n", resp.protocol, resp.Status, resp.Status.String()))
	for header, value := range resp.Headers {
		buffer.WriteString(fmt.Sprintf("%s: %s\n", header, value))
	}
	if len(resp.Body) > 0 {
		buffer.WriteString(fmt.Sprintf("%s: %d\n", "Content-Length", len(resp.Body)))
	}
	buffer.WriteString("\n")

	if len(resp.Body) > 0 {
		buffer.Write(resp.Body)
	}
	return w.Write(buffer.Bytes())
}

func writeRequest(w io.Writer, request *Request) (n int, err error) {
	var buffer bytes.Buffer
	buffer.WriteString(fmt.Sprintf("%s %s %s\n", strings.ToUpper(request.Method.String()), request.RequestURI, request.protocol))
	for header, value := range request.Headers {
		buffer.WriteString(fmt.Sprintf("%s: %s\n", header, value))
	}
	if len(request.Body) > 0 {
		buffer.WriteString(fmt.Sprintf("%s: %s\n", "Content-Length", strconv.Itoa(len(request.Body))))
	}
	buffer.WriteString("\n")
	if len(request.Body) > 0 {
		buffer.Write(request.Body)
	}

	return w.Write(buffer.Bytes())
}

func readResponse(r io.Reader) (*Response, error) {
	resp := new(Response)
	buf := bufio.NewReader(r)
	headers := make(map[string]string)
	statusLine, err := buf.ReadString('\n')
	if err != nil {
		return nil, err
	}
	statusLine = strings.Trim(statusLine, "\n")
	statusLineParts := strings.Split(statusLine, " ")
	if len(statusLineParts) != 3 {
		return nil, fmt.Errorf("improperly formatted status line: %s", statusLine)
	}

	resp.protocol = statusLineParts[0]
	statusNum, err := strconv.Atoi(statusLineParts[1])
	if err != nil {
		return nil, fmt.Errorf("status not a valid integer: %s", statusLineParts[1])
	}
	status, err := getStatus(statusNum)

	if err != nil {
		return nil, fmt.Errorf("status does exist in RTSP protocol: %d", statusNum)
	}
	resp.Status = status

	// now we can read the headers.
	// we read a line until we hit the empty line
	// which indicates all the headers have been processed
	for {
		headerField, err := buf.ReadString('\n')
		if err != nil {
			return nil, err
		}
		headerField = strings.Trim(headerField, "\n")
		if strings.Trim(headerField, "\n") == "" {
			break
		}
		headerParts := strings.Split(headerField, ":")
		if len(headerParts) < 2 {
			return nil, fmt.Errorf("improper header: %s", headerField)
		}
		headers[strings.TrimSpace(headerParts[0])] = strings.TrimSpace(headerParts[1])
	}
	resp.Headers = headers

	contentLength, hasBody := resp.Headers["Content-Length"]
	if !hasBody {
		return resp, nil
	}

	// now read the body
	length, err := strconv.Atoi(contentLength)
	if err != nil {
		return nil, fmt.Errorf("unable to parse header 'Content-Length' (string: %s): %w", contentLength, err)
	}
	resp.Body = make([]byte, length)
	// makes sure we read the full length of the content
	if _, err := io.ReadFull(buf, resp.Body); err != nil {
		if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
			return resp, fmt.Errorf("error reading body into buffer: %w", err)
		}
	}

	return resp, nil
}

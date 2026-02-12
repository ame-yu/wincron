//go:build windows

package main

import (
	"bufio"
	"errors"
	"fmt"
	"net"
	"os"
	"strings"
	"time"

	"github.com/Microsoft/go-winio"
	"golang.org/x/sys/windows"
)

const wincronControlPipeServicePath = `\\.\pipe\wincron_control_service`

func wincronControlPipeUserPath() string {
	sid, err := currentProcessUserSID()
	if err != nil || sid == "" {
		return `\\.\pipe\wincron_control_user`
	}
	return `\\.\pipe\wincron_control_` + sid
}

func currentProcessUserSID() (string, error) {
	var tok windows.Token
	if err := windows.OpenProcessToken(windows.CurrentProcess(), windows.TOKEN_QUERY, &tok); err != nil {
		return "", err
	}
	defer tok.Close()

	tu, err := tok.GetTokenUser()
	if err != nil {
		return "", err
	}
	if tu == nil || tu.User.Sid == nil {
		return "", errors.New("token user sid not available")
	}
	return tu.User.Sid.String(), nil
}

func pipeSecurityDescriptor(allowAuthenticatedUsers bool) string {
	if allowAuthenticatedUsers {
		return "D:P(A;;GA;;;SY)(A;;GA;;;BA)(A;;GA;;;AU)"
	}
	sid, err := currentProcessUserSID()
	if err != nil || sid == "" {
		return "D:P(A;;GA;;;SY)(A;;GA;;;BA)(A;;GA;;;AU)"
	}
	return fmt.Sprintf("D:P(A;;GA;;;SY)(A;;GA;;;BA)(A;;GA;;;%s)", sid)
}

func startIPCServer(pipePath string, allowAuthenticatedUsers bool, handler func(ipcRequest) ipcResponse) (stop func(), err error) {
	cfg := &winio.PipeConfig{
		SecurityDescriptor: pipeSecurityDescriptor(allowAuthenticatedUsers),
	}
	l, err := winio.ListenPipe(pipePath, cfg)
	if err != nil {
		l, err = winio.ListenPipe(pipePath, nil)
		if err != nil {
			return nil, err
		}
	}

	done := make(chan struct{})
	go func() {
		for {
			conn, acceptErr := l.Accept()
			if acceptErr != nil {
				select {
				case <-done:
					return
				default:
					continue
				}
			}
			go func(c net.Conn) {
				defer c.Close()
				_ = c.SetDeadline(time.Now().Add(5 * time.Second))

				r := bufio.NewReaderSize(c, 4096)
				line, readErr := r.ReadBytes('\n')
				if readErr != nil {
					return
				}

				var req ipcRequest
				if err := unmarshalJSONLine(line, &req); err != nil {
					resp := ipcResponse{Ok: false, Error: "invalid request"}
					if b, mErr := marshalJSONLine(resp); mErr == nil {
						_, _ = c.Write(b)
					}
					return
				}
				req.Cmd = strings.ToLower(strings.TrimSpace(req.Cmd))
				resp := handler(req)
				b, mErr := marshalJSONLine(resp)
				if mErr != nil {
					return
				}
				_, _ = c.Write(b)
			}(conn)
		}
	}()

	stop = func() {
		select {
		case <-done:
			return
		default:
			close(done)
			_ = l.Close()
		}
	}
	return stop, nil
}


func sendIPCRequestToPipe(pipePath string, req ipcRequest) (ipcResponse, error) {
	timeout := 2 * time.Second
	conn, err := winio.DialPipe(pipePath, &timeout)
	if err != nil {
		return ipcResponse{}, err
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(4 * time.Second))

	b, err := marshalJSONLine(req)
	if err != nil {
		return ipcResponse{}, err
	}
	if _, err := conn.Write(b); err != nil {
		return ipcResponse{}, err
	}

	r := bufio.NewReaderSize(conn, 4096)
	line, err := r.ReadBytes('\n')
	if err != nil {
		return ipcResponse{}, err
	}

	var resp ipcResponse
	if err := unmarshalJSONLine(line, &resp); err != nil {
		return ipcResponse{}, err
	}
	return resp, nil
}

func sendIPCRequest(req ipcRequest) (ipcResponse, error) {
	paths := []string{wincronControlPipeUserPath(), wincronControlPipeServicePath}
	var lastErr error
	for _, p := range paths {
		resp, err := sendIPCRequestToPipe(p, req)
		if err == nil {
			return resp, nil
		}
		lastErr = err
	}
	return ipcResponse{}, lastErr
}

func isLikelyPipeNotRunning(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, os.ErrNotExist) {
		return true
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "file not found") || strings.Contains(msg, "cannot find") || strings.Contains(msg, "the system cannot find the file specified")
}

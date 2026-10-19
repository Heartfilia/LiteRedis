package redis

import (
	"bufio"
	"context"
	"io"
	"net"
	"strings"
	"testing"
	"time"
)

// fakeProxyStart 启动一个模拟 HTTP CONNECT 代理，respond 为对每个 CONNECT 请求的响应写法
func fakeProxyStart(t *testing.T, respond func(conn net.Conn)) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ln.Close() })
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer c.Close()
				// 读完 CONNECT 请求头
				reader := bufio.NewReader(c)
				for {
					line, err := reader.ReadString('\n')
					if err != nil || line == "\r\n" {
						break
					}
				}
				respond(c)
				// 保持连接直到对端关闭，便于成功后回放数据
				_, _ = io.Copy(io.Discard, reader)
			}(conn)
		}
	}()
	return ln.Addr().String()
}

func dialViaProxy(t *testing.T, proxyURL string) error {
	t.Helper()
	dialer, err := makeProxyDialer(proxyURL, 2*time.Second)
	if err != nil {
		return err
	}
	conn, err := dialer(context.Background(), "tcp", "127.0.0.1:6379")
	if err != nil {
		return err
	}
	defer conn.Close()
	// 成功建立后验证可读写（代理保持连接，读应超时而非立即失败）
	if err := conn.SetReadDeadline(time.Now().Add(300 * time.Millisecond)); err != nil {
		return err
	}
	buf := make([]byte, 16)
	if n, err := conn.Read(buf); err == nil && n > 0 {
		t.Logf("unexpected proxy data: %q", buf[:n])
	}
	return nil
}

func TestHTTPProxyConnectSuccess(t *testing.T) {
	addr := fakeProxyStart(t, func(conn net.Conn) {
		_, _ = conn.Write([]byte("HTTP/1.1 200 Connection established\r\n\r\n"))
	})
	if err := dialViaProxy(t, "http://"+addr); err != nil {
		t.Fatalf("expected successful CONNECT, got: %v", err)
	}
}

func TestHTTPProxyConnectSuccessFragmentedResponse(t *testing.T) {
	addr := fakeProxyStart(t, func(conn net.Conn) {
		_, _ = conn.Write([]byte("HTTP/1.1 200 Conn"))
		time.Sleep(30 * time.Millisecond)
		_, _ = conn.Write([]byte("ection established\r\n"))
		time.Sleep(30 * time.Millisecond)
		_, _ = conn.Write([]byte("\r\n"))
	})
	if err := dialViaProxy(t, "http://"+addr); err != nil {
		t.Fatalf("fragmented 200 response should succeed, got: %v", err)
	}
}

func TestHTTPProxyConnectRejectsNon200(t *testing.T) {
	addr := fakeProxyStart(t, func(conn net.Conn) {
		_, _ = conn.Write([]byte("HTTP/1.1 407 Proxy Authentication Required\r\n\r\n"))
	})
	err := dialViaProxy(t, "http://"+addr)
	if err == nil {
		t.Fatal("407 response should be rejected")
	}
	if !strings.Contains(err.Error(), "407") {
		t.Fatalf("error should contain status line, got: %v", err)
	}
}

func TestHTTPProxyConnectRejectsFake200Substring(t *testing.T) {
	// 旧实现 Contains " 200 " 会误判 "HTTP/1.1 2001 Bad"
	addr := fakeProxyStart(t, func(conn net.Conn) {
		_, _ = conn.Write([]byte("HTTP/1.1 2001 Bad\r\n\r\n"))
	})
	if err := dialViaProxy(t, "http://"+addr); err == nil {
		t.Fatal("status 2001 should be rejected")
	}
}

func TestHTTPProxyConnectHangTimeout(t *testing.T) {
	// 代理接受请求但不响应，deadline 应使拨号超时而非永久挂起
	addr := fakeProxyStart(t, func(conn net.Conn) {
		select {}
	})
	start := time.Now()
	err := dialViaProxy(t, "http://"+addr)
	if err == nil {
		t.Fatal("unresponsive proxy should fail")
	}
	if elapsed := time.Since(start); elapsed > 5*time.Second {
		t.Fatalf("connect took too long, deadline not applied: %v", elapsed)
	}
}

func TestCheckHTTPConnectResponse(t *testing.T) {
	cases := []struct {
		resp    string
		wantErr bool
	}{
		{"HTTP/1.1 200 Connection established\r\n\r\n", false},
		{"HTTP/1.0 200 ok\r\n\r\n", false},
		{"HTTP/1.1 2001 Bad\r\n\r\n", true},
		{"HTTP/1.1 407 Proxy Authentication Required\r\n\r\n", true},
		{"garbage\r\n\r\n", true},
		{"\r\n\r\n", true},
	}
	for _, tc := range cases {
		err := checkHTTPConnectResponse(tc.resp)
		if (err != nil) != tc.wantErr {
			t.Errorf("checkHTTPConnectResponse(%q) err = %v, wantErr = %v", tc.resp, err, tc.wantErr)
		}
	}
}

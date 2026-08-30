package services

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	internalredis "prerender-shield/internal/redis"

	"github.com/go-redis/redis/v8"
	"github.com/stretchr/testify/require"
)

const testRedisAddr = "localhost:6379"
const testRedisDB = 15

// newTestRedisClient 创建连接 localhost:6379 DB15 的内部 Redis 客户端
func newTestRedisClient(t *testing.T) *internalredis.Client {
	t.Helper()
	client, err := internalredis.NewClient(testRedisAddr, "", testRedisDB)
	require.NoError(t, err)
	t.Cleanup(func() { _ = client.Close() })
	return client
}

// newTestRawRedisClient 创建原始 go-redis 客户端（logging 包管理器需要原始客户端）
func newTestRawRedisClient(t *testing.T) *redis.Client {
	t.Helper()
	client := redis.NewClient(&redis.Options{
		Addr: testRedisAddr,
		DB:   testRedisDB,
	})
	require.NoError(t, client.Ping(context.Background()).Err())
	t.Cleanup(func() { _ = client.Close() })
	return client
}

// delTestKeys 清理本测试使用的 Redis 键，保证用例独立
func delTestKeys(t *testing.T, client *redis.Client, patterns ...string) {
	t.Helper()
	ctx := context.Background()
	for _, pattern := range patterns {
		keys, err := client.Keys(ctx, pattern).Result()
		require.NoError(t, err)
		if len(keys) > 0 {
			require.NoError(t, client.Del(ctx, keys...).Err())
		}
	}
}

// roundTripperFunc 函数式 http.RoundTripper，用于将 geoip/LLM 请求路由到本地假响应
type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

// httpResp 构造一个内存 http.Response
func httpResp(status int, body string) (*http.Response, error) {
	rec := httptest.NewRecorder()
	rec.Code = status
	_, _ = rec.Body.WriteString(body)
	return rec.Result(), nil
}

// failRT 始终返回错误的 RoundTripper
func failRT(r *http.Request) (*http.Response, error) {
	return nil, fmt.Errorf("transport disabled for %s", r.URL.Host)
}

// fakeRedisServer 最小 RESP2 Redis 服务端模拟器。
// 支持 LPOP/LPUSH/SELECT 等；failCmds 中的命令返回 -ERR，用于触发生产代码错误分支。
type fakeRedisServer struct {
	listener net.Listener
	mu       sync.Mutex
	lists    map[string][][]byte
	failCmds map[string]bool
}

func newFakeRedisServer(t *testing.T, failCmds map[string]bool) *fakeRedisServer {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	s := &fakeRedisServer{
		listener: ln,
		lists:    make(map[string][][]byte),
		failCmds: failCmds,
	}
	go s.serve()
	t.Cleanup(func() { _ = ln.Close() })
	return s
}

// addr 返回监听地址
func (s *fakeRedisServer) addr() string { return s.listener.Addr().String() }

// push 向指定 list 追加元素（测试内直接注入待处理数据）
func (s *fakeRedisServer) push(key string, values ...string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, v := range values {
		s.lists[key] = append(s.lists[key], []byte(v))
	}
}

func (s *fakeRedisServer) serve() {
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			return
		}
		go s.handle(conn)
	}
}

func (s *fakeRedisServer) handle(conn net.Conn) {
	defer conn.Close()
	buf := make([]byte, 0, 4096)
	tmp := make([]byte, 4096)
	for {
		args, err := s.readCommand(conn, &buf, tmp)
		if err != nil {
			return
		}
		if len(args) == 0 {
			continue
		}
		if _, err := conn.Write(s.dispatch(args)); err != nil {
			return
		}
	}
}

// readCommand 读取一个 RESP2 数组命令
func (s *fakeRedisServer) readCommand(conn net.Conn, buf *[]byte, tmp []byte) ([][]byte, error) {
	for {
		args, consumed, ok := s.tryParseCommand(*buf)
		if ok {
			*buf = (*buf)[consumed:]
			return args, nil
		}
		n, err := conn.Read(tmp)
		if err != nil {
			return nil, err
		}
		*buf = append(*buf, tmp[:n]...)
	}
}

// tryParseCommand 尝试从缓冲区解析一条完整命令；ok=false 表示数据不足
func (s *fakeRedisServer) tryParseCommand(buf []byte) ([][]byte, int, bool) {
	if len(buf) == 0 || buf[0] != '*' {
		// 跳过无法识别的行（不应发生）
		if idx := indexCRLF(buf); idx >= 0 {
			return nil, idx + 2, true
		}
		return nil, 0, false
	}
	// 解析 "*N\r\n"
	lineEnd := indexCRLF(buf)
	if lineEnd < 0 {
		return nil, 0, false
	}
	n, err := strconv.Atoi(string(buf[1:lineEnd]))
	if err != nil || n <= 0 {
		return nil, lineEnd + 2, true
	}
	pos := lineEnd + 2
	args := make([][]byte, 0, n)
	for i := 0; i < n; i++ {
		if pos >= len(buf) || buf[pos] != '$' {
			return nil, 0, false
		}
		lineEnd := indexCRLF(buf[pos:])
		if lineEnd < 0 {
			return nil, 0, false
		}
		length, err := strconv.Atoi(string(buf[pos+1 : pos+lineEnd]))
		if err != nil {
			return nil, 0, false
		}
		start := pos + lineEnd + 2
		end := start + length
		if end+2 > len(buf) {
			return nil, 0, false
		}
		args = append(args, buf[start:end])
		pos = end + 2
	}
	return args, pos, true
}

func indexCRLF(b []byte) int {
	return strings.Index(string(b), "\r\n")
}

// dispatch 处理单条命令并返回 RESP2 响应
func (s *fakeRedisServer) dispatch(args [][]byte) []byte {
	cmd := strings.ToUpper(string(args[0]))

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.failCmds[cmd] {
		return []byte("-ERR simulated failure\r\n")
	}

	switch cmd {
	case "SELECT", "AUTH", "CLIENT", "COMMAND", "PING":
		return []byte("+OK\r\n")
	case "LPOP":
		if len(args) < 2 {
			return []byte("-ERR wrong number of arguments\r\n")
		}
		key := string(args[1])
		list := s.lists[key]
		if len(list) == 0 {
			return []byte("$-1\r\n") // RESP2 nil bulk → go-redis redis.Nil
		}
		item := list[0]
		s.lists[key] = list[1:]
		return []byte(fmt.Sprintf("$%d\r\n%s\r\n", len(item), item))
	case "GET":
		if len(args) < 2 {
			return []byte("-ERR wrong number of arguments\r\n")
		}
		return []byte("$-1\r\n")
	case "LRANGE":
		return []byte("*0\r\n")
	case "LPUSH", "RPUSH":
		if len(args) < 2 {
			return []byte("-ERR wrong number of arguments\r\n")
		}
		key := string(args[1])
		for _, v := range args[2:] {
			s.lists[key] = append(s.lists[key], v)
		}
		return []byte(fmt.Sprintf(":%d\r\n", len(args)-2))
	default:
		// 其余写命令（ZADD/ZREM/SADD/EXPIRE 等）一律成功
		return []byte("+OK\r\n")
	}
}

// waitForCondition 轮询等待条件满足，超时则 Fatal
func waitForCondition(t *testing.T, timeout time.Duration, check func() bool, msg string) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if check() {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatal(msg)
}

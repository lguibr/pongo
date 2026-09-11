package test

import (
	"encoding/json"
	"fmt"
	"github.com/lguibr/pongo/game"
	bollywood "github.com/lguibr/pongo/internal/actor"
	"github.com/lguibr/pongo/server"
	"github.com/lguibr/pongo/utils"
	"golang.org/x/net/websocket"
	"net/http/httptest"
	"os"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// TestPerformanceRooms drives the real wire protocol and drains every client.
func TestPerformanceRooms(t *testing.T) { runPerformanceRooms(t) }

func runPerformanceRooms(t *testing.T) {
	idle := os.Getenv("PONGO_IDLE") == "1"
	count := 4
	if n, e := strconv.Atoi(os.Getenv("PONGO_CLIENTS")); e == nil && n > 0 {
		count = n
	}
	cfg := utils.DefaultConfig()
	// Keep rooms alive for a comparable fixed workload, without random powerups.
	cfg.GridBrickMinLife = 100000
	cfg.GridBrickMaxLife = 100000
	cfg.PowerUpChance = 0
	e := bollywood.NewEngine()
	manager := e.Spawn(bollywood.NewProps(game.NewRoomManagerProducer(e, cfg)))
	s := httptest.NewServer(websocket.Handler(server.New(e, manager).HandleSubscribe()))
	defer s.Close()
	defer e.Shutdown(5 * time.Second)
	url := "ws" + strings.TrimPrefix(s.URL, "http")
	var clients []*websocket.Conn
	defer func() {
		for _, c := range clients {
			_ = c.Close()
		}
	}()
	var wg sync.WaitGroup
	var bytesRead, frames, positions atomic.Int64
	var stopped atomic.Bool
	var errs atomic.Int64
	var mu sync.Mutex
	var gaps []float64
	var code string
	for i := 0; i < count; i++ {
		ws, err := websocket.Dial(url, "", "http://localhost/")
		if err != nil {
			t.Fatal(err)
		}
		clients = append(clients, ws)
		request := map[string]interface{}{"sessionId": fmt.Sprintf("perf-%d", i)}
		if i%4 == 0 {
			request["messageType"] = "createRoom"
			request["isPublic"] = true
		} else {
			request["messageType"] = "joinRoom"
			request["code"] = code
		}
		if err := websocket.JSON.Send(ws, request); err != nil {
			t.Fatal(err)
		}
		if err := ws.SetReadDeadline(time.Now().Add(5 * time.Second)); err != nil {
			t.Fatal(err)
		}
		for {
			var raw []byte
			if err := websocket.Message.Receive(ws, &raw); err != nil {
				t.Fatalf("admission %d: %v", i, err)
			}
			var h struct {
				MessageType string `json:"messageType"`
				Code        string `json:"code"`
			}
			if err := json.Unmarshal(raw, &h); err != nil {
				t.Fatal(err)
			}
			if h.Code != "" {
				code = h.Code
			}
			if h.MessageType == "playerAssignment" {
				break
			}
		}
		_ = ws.SetReadDeadline(time.Time{})
		wg.Add(1)
		go func(c *websocket.Conn) {
			defer wg.Done()
			var last time.Time
			for {
				var raw []byte
				if err := websocket.Message.Receive(c, &raw); err != nil {
					if !stopped.Load() {
						errs.Add(1)
					}
					return
				}
				bytesRead.Add(int64(len(raw)))
				frames.Add(1)
				if strings.Contains(string(raw), "ballPositionUpdate") {
					positions.Add(1)
					now := time.Now()
					if !last.IsZero() {
						mu.Lock()
						gaps = append(gaps, float64(now.Sub(last).Microseconds())/1000)
						mu.Unlock()
					}
					last = now
				}
			}
		}(ws)
	}
	if !idle {
		for _, c := range clients {
			if err := websocket.JSON.Send(c, map[string]interface{}{"messageType": "playerReady", "isReady": true}); err != nil {
				t.Fatal(err)
			}
		}
	}
	time.Sleep(4 * time.Second)
	runtime.GC()
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	startBytes, startFrames, startPositions := bytesRead.Load(), frames.Load(), positions.Load()
	start := time.Now()
	tick := time.NewTicker(100 * time.Millisecond)
	for time.Since(start) < 5*time.Second {
		<-tick.C
		for i, c := range clients {
			d := "ArrowLeft"
			if i%2 == 0 {
				d = "ArrowRight"
			}
			if err := websocket.JSON.Send(c, map[string]interface{}{"messageType": "direction", "direction": d}); err != nil {
				t.Fatal(err)
			}
		}
	}
	tick.Stop()
	elapsed := time.Since(start).Seconds()
	runtime.ReadMemStats(&after)
	mu.Lock()
	sort.Float64s(gaps)
	p95 := 0.0
	if len(gaps) > 0 {
		p95 = gaps[(len(gaps)-1)*95/100]
	}
	mu.Unlock()
	t.Logf("PERFORMANCE clients=%d bytes_per_client_s=%.0f frames_per_client_s=%.2f position_frames_per_client_s=%.2f alloc_bytes_s=%.0f allocs_s=%.0f heap_bytes=%d goroutines=%d position_gap_p95_ms=%.2f errors=%d", count, float64(bytesRead.Load()-startBytes)/elapsed/float64(count), float64(frames.Load()-startFrames)/elapsed/float64(count), float64(positions.Load()-startPositions)/elapsed/float64(count), float64(after.TotalAlloc-before.TotalAlloc)/elapsed, float64(after.Mallocs-before.Mallocs)/elapsed, after.HeapAlloc, runtime.NumGoroutine(), p95, errs.Load())
	if errs.Load() != 0 {
		t.Errorf("unexpected read failures: %d", errs.Load())
	}
	if !idle && positions.Load()-startPositions < int64(count*100) {
		t.Error("insufficient gameplay frames")
	}
	stopped.Store(true)
	for _, c := range clients {
		_ = c.Close()
	}
	wg.Wait()
}

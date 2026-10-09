package main

import (
	"encoding/json"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

type Server struct {
	mu      sync.Mutex
	rooms   map[string]*Room
	clients map[*Client]bool
}
type Client struct {
	conn   *websocket.Conn
	send   chan []byte
	room   *Room
	player *Player
	host   bool
}
type Message struct {
	Type     string   `json:"type"`
	Room     string   `json:"room"`
	Token    string   `json:"token"`
	Name     string   `json:"name"`
	Group    string   `json:"group"`
	Settings Settings `json:"settings"`
	Text     string   `json:"text"`
	ID       string   `json:"id"`
	Seq      int      `json:"t"`
	Stroke   int      `json:"s"`
	Color    string   `json:"c"`
	Width    int      `json:"w"`
	Pts      []int    `json:"pts"`
}

func NewServer() *Server { return &Server{rooms: map[string]*Room{}, clients: map[*Client]bool{}} }
func (c *Client) emitRaw(b []byte) {
	select {
	case c.send <- b:
	default:
		_ = c.conn.Close()
	}
}
func (c *Client) emit(v any) {
	b, _ := json.Marshal(v)
	c.emitRaw(b)
}

// canSee：誰能看到某位玩家的畫布。描述者與白板看得到所有人；
// 分組模式全房都看得到輪到那組的畫；揭曉答案後所有人都看得到。
func (r *Room) canSee(c *Client, owner *Player) bool {
	t := r.Turn
	if t == nil || !r.isDrawer(owner) {
		return false
	}
	if c.host || t.Phase == "reveal" {
		return true
	}
	if c.player == nil {
		return false
	}
	return c.player == owner || r.Settings.Mode == "team" || c.player.ID == t.DescriberID
}
func (s *Server) broadcast(r *Room) {
	r.HostOnline = false
	for _, p := range r.Players {
		p.Online = false
	}
	for c := range s.clients {
		if c.room == r {
			if c.host {
				r.HostOnline = true
			} else if c.player != nil {
				c.player.Online = true
			}
		}
	}
	for c := range s.clients {
		if c.room == r {
			var me, prompt any
			if c.player != nil {
				me = map[string]any{"id": c.player.ID, "name": c.player.Name, "group": c.player.Group}
			}
			if t := r.Turn; t != nil && t.Phase == "drawing" && r.Status == "playing" &&
				(c.host || (c.player != nil && c.player.ID == t.DescriberID)) {
				prompt = prompts[t.prompt]
			}
			c.emit(map[string]any{"type": "state", "room": r, "me": me, "host": c.host, "prompt": prompt, "now": nowMs()})
		}
	}
}
func (s *Server) sendCanvases(c *Client) {
	r := c.room
	if r == nil || r.Turn == nil {
		return
	}
	canvases := map[string][]*Stroke{}
	for _, p := range r.Players {
		if len(p.strokes) > 0 && r.canSee(c, p) {
			canvases[p.ID] = p.strokes
		}
	}
	c.emit(map[string]any{"type": "canvases", "t": r.Turn.Seq, "canvases": canvases})
}
func (s *Server) forward(r *Room, owner *Player, v any) {
	b, _ := json.Marshal(v)
	for c := range s.clients {
		if c.room == r && c.player != owner && r.canSee(c, owner) {
			c.emitRaw(b)
		}
	}
}

// commit 在每次狀態變動後廣播；回合剛揭曉時補送所有畫布，並重新設定倒數計時。
func (s *Server) commit(r *Room, prevSeq int, prevPhase string) {
	s.broadcast(r)
	if t := r.Turn; t != nil && t.Phase == "reveal" && (t.Seq != prevSeq || prevPhase != "reveal") {
		for c := range s.clients {
			if c.room == r {
				s.sendCanvases(c)
			}
		}
	}
	s.arm(r)
}
func (s *Server) arm(r *Room) {
	if r.timer != nil {
		r.timer.Stop()
		r.timer = nil
	}
	t := r.Turn
	if r.Status != "playing" || t == nil || t.Phase != "drawing" || t.EndsAt == 0 {
		return
	}
	seq := t.Seq
	r.timer = time.AfterFunc(time.Until(time.UnixMilli(t.EndsAt)), func() {
		s.mu.Lock()
		defer s.mu.Unlock()
		if r.Status != "playing" || r.Turn == nil || r.Turn.Seq != seq || r.Turn.Phase != "drawing" {
			return
		}
		r.reveal()
		s.commit(r, seq, "drawing")
	})
}
func (s *Server) enter(c *Client, m Message) error {
	var r *Room
	if m.Type == "create" {
		if len(s.rooms) >= 500 {
			return errMsg("目前房間已滿，請稍後再試")
		}
		id := strings.ToUpper(token()[:6])
		for s.rooms[id] != nil {
			id = strings.ToUpper(token()[:6])
		}
		var err error
		if r, err = newRoom(id, m.Settings); err != nil {
			return err
		}
		s.rooms[id] = r
		c.host = true
	} else {
		r = s.rooms[strings.ToUpper(strings.TrimSpace(m.Room))]
		if r == nil {
			return errMsg("找不到房間，可能已過期或伺服器已重啟")
		}
		if m.Type == "resume" {
			if m.Token == r.HostToken {
				c.host = true
			} else {
				for _, p := range r.Players {
					if p.Token == m.Token {
						c.player = p
						break
					}
				}
				if c.player == nil {
					return errMsg("無法恢復身分，請重新加入")
				}
			}
		} else {
			if len(r.Players) >= 300 {
				return errMsg("房間已滿（300 人）")
			}
			var name string
			var err error
			if r.Settings.JoinMode == "auto" {
				name, err = r.randomNickname()
			} else {
				name, err = cleanName(m.Name)
			}
			if err != nil {
				return err
			}
			group := ""
			if r.Settings.Mode == "team" {
				if !r.validGroup(m.Group) {
					return errMsg("請先選擇組別")
				}
				group = m.Group
			}
			c.player = &Player{ID: token()[:12], Name: name, Token: token(), Group: group}
			r.Players = append(r.Players, c.player)
		}
	}
	c.room = r
	r.Updated = time.Now()
	credential := r.HostToken
	if c.player != nil {
		credential = c.player.Token
	}
	c.emit(map[string]any{"type": "session", "room": r.ID, "token": credential, "host": c.host})
	s.broadcast(r)
	s.sendCanvases(c)
	return nil
}

type errMsg string

func (e errMsg) Error() string { return string(e) }

func (s *Server) handle(c *Client, m Message) {
	s.mu.Lock()
	defer s.mu.Unlock()
	fail := func(err error) { c.emit(map[string]any{"type": "error", "message": err.Error()}) }
	if m.Type == "create" || m.Type == "join" || m.Type == "resume" {
		if c.room != nil {
			fail(errMsg("已經加入房間"))
			return
		}
		if err := s.enter(c, m); err != nil {
			fail(err)
		}
		return
	}
	r := c.room
	if r == nil {
		fail(errMsg("請先加入房間"))
		return
	}
	r.Updated = time.Now()
	// 筆跡是高頻訊息：只轉送給看得到的人，不廣播整包狀態。
	switch m.Type {
	case "ink", "undo", "clear":
		if c.player == nil {
			return
		}
		var err error
		if m.Type == "ink" {
			var st *Stroke
			if st, err = r.ink(c.player, m.Seq, m.Stroke, m.Color, m.Width, m.Pts); err == nil {
				s.forward(r, c.player, map[string]any{
					"type": "ink", "p": c.player.ID, "t": m.Seq, "s": st.ID, "c": st.Color, "w": st.Width, "pts": m.Pts,
				})
			}
		} else if err = r.editInk(c.player, m.Seq, m.Type); err == nil {
			s.forward(r, c.player, map[string]any{"type": "canvas", "p": c.player.ID, "t": m.Seq, "strokes": c.player.strokes})
		}
		if err != nil && err != errStale {
			fail(err)
		}
		return
	}
	prevSeq, prevPhase := 0, ""
	if r.Turn != nil {
		prevSeq, prevPhase = r.Turn.Seq, r.Turn.Phase
	}
	judge := c.host || (c.player != nil && r.Turn != nil && c.player.ID == r.Turn.DescriberID)
	var err error
	switch m.Type {
	case "leave":
		if c.player == nil || r.Status != "lobby" {
			err = errMsg("遊戲開始後不能退出重選")
			break
		}
		for i, p := range r.Players {
			if p == c.player {
				r.Players = append(r.Players[:i], r.Players[i+1:]...)
				break
			}
		}
		c.room, c.player = nil, nil
		c.emit(map[string]any{"type": "left"})
	case "start":
		if !c.host {
			err = errMsg("只有主持人可以開始")
		} else {
			err = r.start()
		}
	case "guess":
		if c.player == nil {
			err = errMsg("只有玩家可以猜答案")
		} else {
			err = r.guess(c.player, m.Text)
		}
	case "award":
		if !judge {
			err = errMsg("只有描述者或白板可以判定答對")
		} else {
			err = r.award(m.ID)
		}
	case "skip":
		if !judge {
			err = errMsg("只有描述者或白板可以換題")
		} else {
			err = r.skipPrompt()
		}
	case "end_turn":
		if !judge {
			err = errMsg("只有描述者或白板可以結束本回合")
		} else {
			err = r.endTurn()
		}
	case "next":
		if !judge {
			err = errMsg("只有描述者或白板可以進入下一回合")
		} else {
			err = r.advance()
		}
	case "end":
		if !c.host {
			err = errMsg("只有主持人可以結束遊戲")
		} else {
			err = r.finish()
		}
	case "reset":
		if !c.host {
			err = errMsg("只有主持人可以再玩一場")
		} else {
			err = r.reset()
		}
	default:
		err = errMsg("不支援的操作")
	}
	if err != nil {
		fail(err)
		return
	}
	s.commit(r, prevSeq, prevPhase)
}
func (s *Server) serveWS(w http.ResponseWriter, r *http.Request) {
	up := websocket.Upgrader{CheckOrigin: func(req *http.Request) bool {
		origin := req.Header.Get("Origin")
		if origin == "" {
			return true
		}
		u, err := url.Parse(origin)
		return err == nil && (u.Host == req.Host || origin == strings.TrimRight(os.Getenv("FRONTEND_URL"), "/"))
	}}
	conn, err := up.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	c := &Client{conn: conn, send: make(chan []byte, 256)}
	s.mu.Lock()
	s.clients[c] = true
	s.mu.Unlock()
	go func() {
		ticker := time.NewTicker(25 * time.Second)
		defer ticker.Stop()
		defer conn.Close()
		for {
			select {
			case msg, ok := <-c.send:
				if !ok {
					return
				}
				conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
				if conn.WriteMessage(websocket.TextMessage, msg) != nil {
					return
				}
			case <-ticker.C:
				conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
				if conn.WriteMessage(websocket.PingMessage, nil) != nil {
					return
				}
			}
		}
	}()
	defer func() {
		s.mu.Lock()
		delete(s.clients, c)
		close(c.send)
		if c.room != nil {
			s.broadcast(c.room)
		}
		s.mu.Unlock()
		conn.Close()
	}()
	conn.SetReadLimit(64000)
	conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	conn.SetPongHandler(func(string) error { return conn.SetReadDeadline(time.Now().Add(60 * time.Second)) })
	for {
		var m Message
		if err := conn.ReadJSON(&m); err != nil {
			return
		}
		s.handle(c, m)
	}
}
func (s *Server) router() *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	router.Use(gin.Recovery())
	router.GET("/api/health", func(c *gin.Context) { c.JSON(200, gin.H{"status": "ok", "prompts": len(prompts)}) })
	router.GET("/api/config", func(c *gin.Context) {
		c.JSON(200, gin.H{"frontendUrl": os.Getenv("FRONTEND_URL"), "prompts": len(prompts)})
	})
	router.GET("/api/rooms/:id", func(c *gin.Context) {
		s.mu.Lock()
		defer s.mu.Unlock()
		room := s.rooms[strings.ToUpper(strings.TrimSpace(c.Param("id")))]
		if room == nil {
			c.JSON(http.StatusNotFound, gin.H{"message": "找不到房間"})
			return
		}
		counts := map[string]int{}
		for _, p := range room.Players {
			counts[p.Group]++
		}
		c.JSON(http.StatusOK, gin.H{
			"mode": room.Settings.Mode, "joinMode": room.Settings.JoinMode,
			"groups": room.Settings.Groups, "status": room.Status, "groupCounts": counts,
		})
	})
	router.GET("/ws", func(c *gin.Context) { s.serveWS(c.Writer, c.Request) })
	static := os.Getenv("STATIC_DIR")
	if static == "" {
		static = "../frontend/dist"
	}
	router.NoRoute(func(c *gin.Context) {
		if strings.HasPrefix(c.Request.URL.Path, "/api/") {
			c.Status(404)
			return
		}
		path := filepath.Join(static, filepath.Clean("/"+c.Request.URL.Path))
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			if strings.Contains(path, "/assets/") {
				c.Header("Cache-Control", "public, max-age=31536000, immutable")
			}
			c.File(path)
			return
		}
		c.Header("Cache-Control", "no-store")
		c.File(filepath.Join(static, "index.html"))
	})
	return router
}
func main() {
	s := NewServer()
	go func() {
		for range time.Tick(10 * time.Minute) {
			s.mu.Lock()
			for id, r := range s.rooms {
				active := false
				for c := range s.clients {
					if c.room == r {
						active = true
						break
					}
				}
				if !active && time.Since(r.Updated) > 2*time.Hour {
					if r.timer != nil {
						r.timer.Stop()
					}
					delete(s.rooms, id)
				}
			}
			s.mu.Unlock()
		}
	}()
	port := os.Getenv("PORT")
	if port == "" {
		port = "8083"
	}
	log.Printf("Draw & Guess listening on :%s (%d prompts)", port, len(prompts))
	log.Fatal(s.router().Run(":" + port))
}

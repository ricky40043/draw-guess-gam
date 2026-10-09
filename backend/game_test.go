package main

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

func TestPromptBank(t *testing.T) {
	if len(prompts) < 100 || len(prompts) > 300 {
		t.Fatalf("題庫數量 %d 不在 100–300", len(prompts))
	}
	seen := map[string]string{}
	for _, p := range prompts {
		if p.Name == "" || !strings.Contains(p.SVG, "<") || strings.Contains(p.SVG, "\"") {
			t.Fatalf("題目格式錯誤：%+v", p)
		}
		for _, n := range append([]string{p.Name}, p.Aliases...) {
			key := normalize(n)
			if other, ok := seen[key]; ok && other != p.Name {
				t.Fatalf("%q 同時屬於 %s 與 %s", n, other, p.Name)
			}
			seen[key] = p.Name
		}
	}
}

func TestMatches(t *testing.T) {
	p := Prompt{Name: "車子", Aliases: []string{"汽車", "UFO"}}
	for _, ok := range []string{"車子", " 車 子！", "汽車", "ufo", "ＵＦＯ"} {
		if !p.matches(ok) {
			t.Fatalf("%q 應該算對", ok)
		}
	}
	for _, bad := range []string{"", "車", "是車子嗎", "！！"} {
		if p.matches(bad) {
			t.Fatalf("%q 不應該算對", bad)
		}
	}
}

func soloRoom(t *testing.T, names ...string) *Room {
	r, err := newRoom("ROOM01", Settings{Mode: "solo", Rounds: 3, Seconds: 60})
	if err != nil {
		t.Fatal(err)
	}
	for _, n := range names {
		r.Players = append(r.Players, &Player{ID: n, Name: n, Online: true})
	}
	return r
}

func TestValidate(t *testing.T) {
	bad := []Settings{
		{Mode: "x", Rounds: 3}, {Rounds: 0}, {Rounds: 101}, {Rounds: 3, Seconds: 10},
		{Mode: "team", Rounds: 3, Groups: 1}, {Mode: "team", Rounds: 3, Groups: 9},
	}
	for _, s := range bad {
		if validate(&s) == nil {
			t.Fatalf("應該拒絕 %+v", s)
		}
	}
	s := Settings{Rounds: 5, Groups: 4}
	if err := validate(&s); err != nil || s.Mode != "solo" || s.Groups != 0 || s.JoinMode != "name" {
		t.Fatalf("預設值錯誤 %+v %v", s, err)
	}
}

func TestSoloScoringAndRotation(t *testing.T) {
	r := soloRoom(t, "a", "b", "c", "d", "e")
	if err := r.start(); err != nil {
		t.Fatal(err)
	}
	if r.Turn.DescriberID != "a" {
		t.Fatalf("第一位描述者應該是 a，得到 %s", r.Turn.DescriberID)
	}
	answer := prompts[r.Turn.prompt].Name
	if err := r.guess(r.player("a"), answer); err == nil {
		t.Fatal("描述者不能猜")
	}
	if err := r.guess(r.player("b"), "絕對不是答案"); err != nil {
		t.Fatal(err)
	}
	if err := r.guess(r.player("b"), answer); err == nil {
		t.Fatal("冷卻時間內不能再猜")
	}
	r.player("b").lastGuess = time.Time{}
	if err := r.guess(r.player("b"), answer); err != nil {
		t.Fatal(err)
	}
	if err := r.award("c"); err != nil {
		t.Fatal(err)
	}
	if err := r.guess(r.player("d"), answer); err != nil {
		t.Fatal(err)
	}
	if r.Turn.Phase != "drawing" {
		t.Fatal("還有人沒答對，不應結束")
	}
	if err := r.skipPrompt(); err == nil {
		t.Fatal("有人答對後不能換題")
	}
	if err := r.guess(r.player("e"), answer); err != nil {
		t.Fatal(err)
	}
	if r.Turn.Phase != "reveal" || r.Turn.Answer != answer {
		t.Fatal("全員答對應該揭曉")
	}
	want := map[string]int{"a": 1, "b": 3, "c": 2, "d": 1, "e": 1}
	for id, score := range want {
		if got := r.player(id).Score; got != score {
			t.Fatalf("%s 分數 %d，應為 %d", id, got, score)
		}
	}
	if !r.Turn.Guesses[len(r.Turn.Guesses)-3].ByJudge {
		t.Fatal("白板判定應該標記 byJudge")
	}
	for _, g := range r.Turn.Guesses {
		if g.Correct && g.Text != "" {
			t.Fatal("答對的訊息不能洩漏答案")
		}
	}
	if err := r.advance(); err != nil || r.Turn.DescriberID != "b" || r.Turn.Number != 2 {
		t.Fatalf("應輪到 b 描述第 2 回合：%v %+v", err, r.Turn)
	}
	r.player("c").Online = false
	r.endTurn()
	r.advance()
	if r.Turn.DescriberID != "d" {
		t.Fatalf("離線的 c 應被跳過，得到 %s", r.Turn.DescriberID)
	}
	r.endTurn()
	if err := r.advance(); err != nil || r.Status != "finished" {
		t.Fatal("3 回合後應結束")
	}
	if err := r.reset(); err != nil || r.Status != "lobby" || r.player("a").Score != 0 {
		t.Fatal("再玩一場應歸零")
	}
}

func TestTeamMode(t *testing.T) {
	r, _ := newRoom("ROOM02", Settings{Mode: "team", Rounds: 4, Groups: 3})
	add := func(id, g string) {
		r.Players = append(r.Players, &Player{ID: id, Name: id, Group: g, Online: true})
	}
	add("a1", "A組")
	if r.start() == nil {
		t.Fatal("只有一組不能開始")
	}
	add("b1", "B組")
	if r.start() == nil {
		t.Fatal("沒有任何一組有兩人不能開始")
	}
	add("a2", "A組")
	add("a3", "A組")
	add("b2", "B組")
	if err := r.start(); err != nil {
		t.Fatal(err)
	}
	if r.Turn.DescriberID != "a1" || r.Turn.Group != "A組" {
		t.Fatalf("應由 A組 a1 開始：%+v", r.Turn)
	}
	if !r.isDrawer(r.player("a2")) || r.isDrawer(r.player("b1")) || r.isDrawer(r.player("a1")) {
		t.Fatal("只有 A 組非描述者要畫")
	}
	// 別組搶答：搶答隊 +3，畫的那隊（描述者）+2，回合立即結束
	if err := r.guess(r.player("b1"), prompts[r.Turn.prompt].Name); err != nil {
		t.Fatal(err)
	}
	if r.Turn.Phase != "reveal" || r.player("b1").Score != 3 || r.player("a1").Score != 2 {
		t.Fatal("分組搶答計分錯誤")
	}
	r.advance()
	if r.Turn.DescriberID != "b1" {
		t.Fatalf("應輪到 B組 b1，得到 %s", r.Turn.DescriberID)
	}
	if err := r.award("b2"); err != nil {
		t.Fatal(err)
	}
	// 同組猜對：猜對 +3、描述者 +2，該隊共拿 5 分
	if r.player("b2").Score != 3 || r.player("b1").Score != 3+2 {
		t.Fatal("同組猜對應讓該隊拿 5 分")
	}
	r.advance()
	if r.Turn.DescriberID != "a2" {
		t.Fatalf("C 組沒人應跳過，A 組換 a2，得到 %s", r.Turn.DescriberID)
	}
}

func TestInk(t *testing.T) {
	r := soloRoom(t, "a", "b")
	r.start()
	b, a := r.player("b"), r.player("a")
	seq := r.Turn.Seq
	if _, err := r.ink(a, seq, 1, "#1f2a24", 8, []int{1, 2}); err == nil {
		t.Fatal("描述者不能畫")
	}
	if _, err := r.ink(b, seq-1, 1, "#1f2a24", 8, []int{1, 2}); err != errStale {
		t.Fatal("舊回合筆跡要丟棄")
	}
	for _, bad := range [][]int{{}, {1}, {1, 1001}, {-1, 0}} {
		if _, err := r.ink(b, seq, 1, "#1f2a24", 8, bad); err == nil {
			t.Fatalf("應拒絕 %v", bad)
		}
	}
	if _, err := r.ink(b, seq, 1, "#123456", 8, []int{1, 2}); err == nil {
		t.Fatal("應拒絕調色盤以外的顏色")
	}
	r.ink(b, seq, 1, "#1f2a24", 8, []int{1, 2})
	r.ink(b, seq, 1, "#e4572e", 30, []int{3, 4})
	r.ink(b, seq, 2, "#e4572e", 30, []int{5, 6})
	if len(b.strokes) != 2 || len(b.strokes[0].Pts) != 4 || b.strokes[0].Color != "#1f2a24" || b.inkInts != 6 {
		t.Fatalf("筆畫合併錯誤 %+v", b.strokes)
	}
	r.editInk(b, seq, "undo")
	if len(b.strokes) != 1 || b.inkInts != 4 {
		t.Fatal("復原錯誤")
	}
	r.skipPrompt()
	if b.strokes != nil || r.Turn.Seq == seq {
		t.Fatal("換題要清空畫布並換序號")
	}
}

// ---- WebSocket 整合測試 ----

type wsClient struct {
	t    *testing.T
	conn *websocket.Conn
}

func dial(t *testing.T, srv *httptest.Server) *wsClient {
	conn, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(srv.URL, "http")+"/ws", nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close() })
	return &wsClient{t, conn}
}
func (w *wsClient) send(v map[string]any) {
	if err := w.conn.WriteJSON(v); err != nil {
		w.t.Fatal(err)
	}
}

// wait 讀到指定類型的訊息為止（略過其他訊息）。
func (w *wsClient) wait(kind string) map[string]any {
	w.conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	for {
		var m map[string]any
		if err := w.conn.ReadJSON(&m); err != nil {
			w.t.Fatalf("等待 %s 失敗：%v", kind, err)
		}
		if m["type"] == "error" && kind != "error" {
			w.t.Fatalf("收到錯誤：%v", m["message"])
		}
		if m["type"] == kind {
			return m
		}
	}
}

// quiet 確認短時間內沒有收到指定類型的訊息。讀取逾時後連線無法再讀，之後只能送出。
func (w *wsClient) quiet(kind string) {
	w.conn.SetReadDeadline(time.Now().Add(300 * time.Millisecond))
	for {
		var m map[string]any
		if err := w.conn.ReadJSON(&m); err != nil {
			return
		}
		if m["type"] == kind {
			w.t.Fatalf("不應收到 %s：%v", kind, m)
		}
	}
}

func TestWebSocketFlow(t *testing.T) {
	s := NewServer()
	srv := httptest.NewServer(s.router())
	defer srv.Close()

	host := dial(t, srv)
	host.send(map[string]any{"type": "create", "settings": map[string]any{"mode": "solo", "rounds": 2, "seconds": 0}})
	session := host.wait("session")
	room, hostToken := session["room"].(string), session["token"].(string)

	var players []*wsClient
	for _, n := range []string{"小明", "小華", "小美"} {
		p := dial(t, srv)
		p.send(map[string]any{"type": "join", "room": room, "name": n})
		p.wait("session")
		players = append(players, p)
	}
	host.send(map[string]any{"type": "start"})
	describer, drawer, other := players[0], players[1], players[2]

	st := describer.wait("state")
	for st["room"].(map[string]any)["status"] != "playing" {
		st = describer.wait("state")
	}
	prompt, _ := st["prompt"].(map[string]any)
	if prompt == nil || prompt["svg"] == "" {
		t.Fatal("描述者應拿到題目")
	}
	answer := prompt["name"].(string)
	seq := st["room"].(map[string]any)["turn"].(map[string]any)["seq"]

	ds := drawer.wait("state")
	for ds["room"].(map[string]any)["status"] != "playing" {
		ds = drawer.wait("state")
	}
	if ds["prompt"] != nil || strings.Contains(mustJSON(ds), answer) {
		t.Fatal("畫畫的人不能拿到答案")
	}

	drawer.send(map[string]any{"type": "ink", "t": seq, "s": 1, "c": "#1f2a24", "w": 8, "pts": []int{10, 10, 20, 20}})
	ink := describer.wait("ink")
	if ink["p"] == nil || len(ink["pts"].([]any)) != 4 {
		t.Fatalf("描述者應收到筆跡 %v", ink)
	}
	host.wait("ink")
	other.quiet("ink") // 個人模式：其他畫畫的人看不到別人的畫

	// 白板重新整理後應補回目前的畫布
	reconnect := dial(t, srv)
	reconnect.send(map[string]any{"type": "resume", "room": room, "token": hostToken})
	if got := reconnect.wait("canvases")["canvases"].(map[string]any); len(got) != 1 {
		t.Fatalf("重連應補回畫布 %v", got)
	}

	drawer.send(map[string]any{"type": "guess", "text": answer})
	other.send(map[string]any{"type": "guess", "text": answer})
	canv := drawer.wait("canvases") // 全員答對 → 揭曉，所有人收到畫布
	if len(canv["canvases"].(map[string]any)) != 1 {
		t.Fatalf("揭曉時應收到所有畫布 %v", canv)
	}
	describer.send(map[string]any{"type": "next"})
	st = host.wait("state")
	for st["room"].(map[string]any)["turn"].(map[string]any)["number"].(float64) != 2 {
		st = host.wait("state")
	}
	turn := st["room"].(map[string]any)["turn"].(map[string]any)
	if turn["describerName"] != "小華" || st["prompt"] == nil {
		t.Fatalf("第 2 回合應由小華描述，白板也拿得到題目 %v", turn)
	}
	describer.send(map[string]any{"type": "skip"})
	if describer.wait("error")["message"] == "" {
		t.Fatal("非描述者不能換題")
	}
}

func TestTimerReveals(t *testing.T) {
	s := NewServer()
	r := soloRoom(t, "a", "b")
	r.Settings.Seconds = 30
	r.start()
	r.Turn.EndsAt = nowMs() + 50
	s.mu.Lock()
	s.arm(r)
	s.mu.Unlock()
	time.Sleep(200 * time.Millisecond)
	s.mu.Lock()
	defer s.mu.Unlock()
	if r.Turn.Phase != "reveal" {
		t.Fatal("時間到應自動揭曉")
	}
}

func mustJSON(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}

func TestRoomID(t *testing.T) {
	for i := 0; i < 500; i++ {
		id := newRoomID()
		if len(id) != 4 || strings.Trim(id, "0123456789") != "" {
			t.Fatalf("房號應為 4 位數字，得到 %q", id)
		}
	}
}

func TestRejoinSameRoom(t *testing.T) {
	s := NewServer()
	srv := httptest.NewServer(s.router())
	defer srv.Close()
	host := dial(t, srv)
	host.send(map[string]any{"type": "create", "settings": map[string]any{"rounds": 2}})
	room := host.wait("session")["room"].(string)

	first := dial(t, srv)
	first.send(map[string]any{"type": "join", "room": room, "name": "小明"})
	token := first.wait("session")["token"].(string)
	id := first.wait("state")["me"].(map[string]any)["id"]

	// 同一個身分還在別的分頁上線：軟性恢復要被拒絕，不能搶走
	other := dial(t, srv)
	other.send(map[string]any{"type": "resume", "room": room, "token": token, "soft": true})
	if e := other.wait("error"); e["code"] != "in_use" {
		t.Fatalf("應回 in_use：%v", e)
	}

	// 關掉分頁後再回來：回到同一個房間、同一個身分，不會多一個玩家
	first.conn.Close()
	time.Sleep(100 * time.Millisecond)
	back := dial(t, srv)
	back.send(map[string]any{"type": "resume", "room": room, "token": token, "soft": true})
	back.wait("session")
	st := back.wait("state")
	if st["me"].(map[string]any)["id"] != id || len(st["room"].(map[string]any)["players"].([]any)) != 1 {
		t.Fatalf("應恢復同一位玩家且沒有重複：%v", st["room"])
	}

	// 白板離開再回來
	host.conn.Close()
	hostBack := dial(t, srv)
	hostBack.send(map[string]any{"type": "resume", "room": room, "token": s.rooms[room].HostToken, "soft": true})
	if hostBack.wait("session")["host"] != true {
		t.Fatal("白板應能回到房間")
	}

	// 房間不在了
	gone := dial(t, srv)
	gone.send(map[string]any{"type": "resume", "room": "0000", "token": token, "soft": true})
	if msg := gone.wait("error")["message"].(string); !strings.Contains(msg, "找不到房間") {
		t.Fatalf("房間不存在應明確告知：%s", msg)
	}
}

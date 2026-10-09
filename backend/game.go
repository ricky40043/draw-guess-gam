package main

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"math/big"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	canvasSize    = 1000 // 筆跡座標正規化到 0–1000
	maxMessagePts = 400  // 單則訊息最多點數
	maxInkInts    = 60000
	maxStrokes    = 800
	maxFeed       = 80
	guessCooldown = 600 * time.Millisecond
)

var palette = map[string]bool{
	"#1f2a24": true, "#e4572e": true, "#2e86de": true, "#2fa24f": true,
	"#f2b705": true, "#8e44ad": true, "#ffffff": true,
}

// errStale 表示筆跡屬於已結束的回合，靜默丟棄即可。
var errStale = errors.New("stale")

type Settings struct {
	Mode     string `json:"mode"`    // solo | team
	Rounds   int    `json:"rounds"`  // 總回合數，每回合換一位描述者
	Seconds  int    `json:"seconds"` // 0 = 不限時
	Groups   int    `json:"groups"`
	JoinMode string `json:"joinMode"`
}
type Stroke struct {
	ID    int    `json:"id"`
	Color string `json:"c"`
	Width int    `json:"w"`
	Pts   []int  `json:"pts"`
}
type Player struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Token     string `json:"-"`
	Group     string `json:"group,omitempty"`
	Score     int    `json:"score"`
	Described int    `json:"described"`
	Online    bool   `json:"online"`
	strokes   []*Stroke
	inkInts   int
	lastGuess time.Time
}
type Guess struct {
	ID       int    `json:"id"`
	PlayerID string `json:"playerId"`
	Name     string `json:"name"`
	Group    string `json:"group,omitempty"`
	Text     string `json:"text,omitempty"`
	Correct  bool   `json:"correct"`
	ByJudge  bool   `json:"byJudge,omitempty"`
	Points   int    `json:"points,omitempty"`
}
type Turn struct {
	Seq           int      `json:"seq"`
	Number        int      `json:"number"`
	DescriberID   string   `json:"describerId"`
	DescriberName string   `json:"describerName"`
	Group         string   `json:"group,omitempty"`
	Phase         string   `json:"phase"` // drawing | reveal
	StartedAt     int64    `json:"startedAt"`
	EndsAt        int64    `json:"endsAt"`
	Correct       []string `json:"correct"`
	Guesses       []*Guess `json:"guesses"`
	Answer        string   `json:"answer,omitempty"`
	AnswerSVG     string   `json:"answerSvg,omitempty"`
	Skips         int      `json:"skips"`
	prompt        int
}
type Room struct {
	ID          string    `json:"id"`
	Settings    Settings  `json:"settings"`
	Status      string    `json:"status"` // lobby | playing | finished
	Players     []*Player `json:"players"`
	Turn        *Turn     `json:"turn,omitempty"`
	Round       int       `json:"round"`
	HostOnline  bool      `json:"hostOnline"`
	HostToken   string    `json:"-"`
	Updated     time.Time `json:"-"`
	deck        []int
	deckPos     int
	seq         int
	guessSeq    int
	soloNext    int
	groupCursor int
	groupNext   map[string]int
	timer       *time.Timer
}

var nicknameAdjectives = []string{
	"開心的", "勇敢的", "溫柔的", "好奇的", "活潑的",
	"冷靜的", "聰明的", "可愛的", "害羞的", "熱情的",
	"悠閒的", "認真的", "幸運的", "淘氣的", "善良的",
	"生氣的", "憂鬱的", "閃亮的", "神秘的", "愛笑的",
}

var nicknameAnimals = []string{
	"獅子", "羚羊", "水獺", "企鵝", "長頸鹿", "大象",
	"狐狸", "浣熊", "兔子", "無尾熊", "熊貓", "海豚",
	"海豹", "河馬", "斑馬", "袋鼠", "刺蝟", "松鼠",
	"貓頭鷹", "老虎", "花豹", "狼", "梅花鹿", "羊駝",
	"樹懶", "北極熊", "紅鶴", "鸚鵡", "烏龜", "鯨魚",
}

func token() string {
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b)
}
func nowMs() int64 { return time.Now().UnixMilli() }
func randomIndex(length int) int {
	v, err := rand.Int(rand.Reader, big.NewInt(int64(length)))
	if err != nil {
		panic(err)
	}
	return int(v.Int64())
}
func cleanName(s string) (string, error) {
	s = strings.TrimSpace(s)
	if s == "" || utf8.RuneCountInString(s) > 20 {
		return "", errors.New("請輸入 1–20 個字的名字")
	}
	return s, nil
}
func groupLabel(index int) string {
	return string(rune('A'+index)) + "組"
}
func validate(s *Settings) error {
	if s.Mode == "" {
		s.Mode = "solo"
	}
	if s.Mode != "solo" && s.Mode != "team" {
		return errors.New("遊戲模式設定錯誤")
	}
	if s.JoinMode == "" {
		s.JoinMode = "name"
	}
	if s.JoinMode != "name" && s.JoinMode != "auto" {
		return errors.New("加入方式設定錯誤")
	}
	if s.Rounds < 1 || s.Rounds > 100 {
		return errors.New("回合數需介於 1–100")
	}
	if s.Seconds != 0 && (s.Seconds < 30 || s.Seconds > 600) {
		return errors.New("每回合秒數需介於 30–600，或選不限時")
	}
	if s.Mode == "team" {
		if s.Groups < 2 || s.Groups > 8 {
			return errors.New("分組數需介於 2–8 組")
		}
	} else {
		s.Groups = 0
	}
	return nil
}
func newRoom(id string, s Settings) (*Room, error) {
	if err := validate(&s); err != nil {
		return nil, err
	}
	return &Room{
		ID: id, Settings: s, Status: "lobby", Players: []*Player{},
		HostToken: token(), Updated: time.Now(), groupNext: map[string]int{},
	}, nil
}
func (r *Room) randomNickname() (string, error) {
	used := make(map[string]bool, len(r.Players))
	for _, player := range r.Players {
		used[player.Name] = true
	}
	total := len(nicknameAdjectives) * len(nicknameAnimals)
	start := randomIndex(total)
	for offset := 0; offset < total; offset++ {
		combination := (start + offset) % total
		name := nicknameAdjectives[combination/len(nicknameAnimals)] + nicknameAnimals[combination%len(nicknameAnimals)]
		if !used[name] {
			return name, nil
		}
	}
	return "", errors.New("自動暱稱已用完，請建立新房間")
}
func (r *Room) validGroup(group string) bool {
	for i := 0; i < r.Settings.Groups; i++ {
		if group == groupLabel(i) {
			return true
		}
	}
	return false
}
func (r *Room) player(id string) *Player {
	for _, p := range r.Players {
		if p.ID == id {
			return p
		}
	}
	return nil
}
func (r *Room) drawPrompt() int {
	if r.deckPos >= len(r.deck) {
		r.deck = make([]int, len(prompts))
		for i := range r.deck {
			r.deck[i] = i
		}
		for i := len(r.deck) - 1; i > 0; i-- {
			j := randomIndex(i + 1)
			r.deck[i], r.deck[j] = r.deck[j], r.deck[i]
		}
		r.deckPos = 0
	}
	r.deckPos++
	return r.deck[r.deckPos-1]
}

// isDrawer：個人模式除描述者外都要畫；分組模式只有輪到的那組（除描述者）要畫。
func (r *Room) isDrawer(p *Player) bool {
	t := r.Turn
	if t == nil || p.ID == t.DescriberID {
		return false
	}
	return r.Settings.Mode == "solo" || p.Group == t.Group
}
func (t *Turn) isCorrect(id string) bool {
	for _, c := range t.Correct {
		if c == id {
			return true
		}
	}
	return false
}
func (r *Room) groupMembers() map[string][]*Player {
	members := map[string][]*Player{}
	for _, p := range r.Players {
		members[p.Group] = append(members[p.Group], p)
	}
	return members
}
func (r *Room) canStart() error {
	if r.Settings.Mode == "solo" {
		if len(r.Players) < 2 {
			return errors.New("至少需要 2 位玩家才能開始")
		}
		return nil
	}
	groups, hasPair := 0, false
	for _, ms := range r.groupMembers() {
		groups++
		hasPair = hasPair || len(ms) >= 2
	}
	if groups < 2 || !hasPair {
		return errors.New("至少需要 2 組有人，且至少一組有 2 人以上")
	}
	return nil
}

// pickDescriber 優先挑在線的人；個人模式依加入順序輪流，分組模式各組輪流、組內再輪流。
func (r *Room) pickDescriber() *Player {
	for pass := 0; pass < 2; pass++ {
		if r.Settings.Mode == "solo" {
			n := len(r.Players)
			for k := 0; k < n; k++ {
				i := (r.soloNext + k) % n
				if p := r.Players[i]; pass == 1 || p.Online {
					r.soloNext = i + 1
					return p
				}
			}
			continue
		}
		members := r.groupMembers()
		for k := 0; k < r.Settings.Groups; k++ {
			gi := (r.groupCursor + k) % r.Settings.Groups
			label := groupLabel(gi)
			ms := members[label]
			online := 0
			for _, m := range ms {
				if m.Online {
					online++
				}
			}
			if len(ms) < 2 || (pass == 0 && online < 2) {
				continue
			}
			start := r.groupNext[label]
			for j := 0; j < len(ms); j++ {
				idx := (start + j) % len(ms)
				if pass == 1 || ms[idx].Online {
					r.groupNext[label] = idx + 1
					r.groupCursor = gi + 1
					return ms[idx]
				}
			}
		}
	}
	return nil
}
func (r *Room) clearInk() {
	r.seq++
	for _, p := range r.Players {
		p.strokes = nil
		p.inkInts = 0
	}
}
func (r *Room) beginTurn(number int, d *Player) {
	r.clearInk()
	now := nowMs()
	t := &Turn{
		Seq: r.seq, Number: number, DescriberID: d.ID, DescriberName: d.Name, Group: d.Group,
		Phase: "drawing", StartedAt: now, Correct: []string{}, Guesses: []*Guess{}, prompt: r.drawPrompt(),
	}
	if r.Settings.Seconds > 0 {
		t.EndsAt = now + int64(r.Settings.Seconds)*1000
	}
	d.Described++
	r.Turn = t
}
func (r *Room) start() error {
	if r.Status != "lobby" {
		return errors.New("遊戲已經開始")
	}
	if err := r.canStart(); err != nil {
		return err
	}
	r.soloNext, r.groupCursor, r.groupNext = 0, 0, map[string]int{}
	for _, p := range r.Players {
		p.Score, p.Described = 0, 0
	}
	d := r.pickDescriber()
	if d == nil {
		return errors.New("沒有可以擔任描述者的玩家")
	}
	r.Status = "playing"
	r.beginTurn(1, d)
	return nil
}
func (r *Room) drawing() (*Turn, error) {
	if r.Status != "playing" || r.Turn == nil || r.Turn.Phase != "drawing" {
		return nil, errors.New("本回合已結束")
	}
	return r.Turn, nil
}
func (r *Room) addFeed(g *Guess) {
	r.guessSeq++
	g.ID = r.guessSeq
	t := r.Turn
	t.Guesses = append(t.Guesses, g)
	if len(t.Guesses) > maxFeed {
		t.Guesses = t.Guesses[len(t.Guesses)-maxFeed:]
	}
}
func (r *Room) guess(p *Player, text string) error {
	t, err := r.drawing()
	if err != nil {
		return err
	}
	if p.ID == t.DescriberID {
		return errors.New("描述者不能猜答案")
	}
	if t.isCorrect(p.ID) {
		return errors.New("你已經答對了")
	}
	text = strings.TrimSpace(text)
	if text == "" || utf8.RuneCountInString(text) > 30 {
		return errors.New("請輸入 1–30 個字")
	}
	if time.Since(p.lastGuess) < guessCooldown {
		return errors.New("猜太快了，稍等一下")
	}
	p.lastGuess = time.Now()
	if prompts[t.prompt].matches(text) {
		r.markCorrect(p, false)
		return nil
	}
	r.addFeed(&Guess{PlayerID: p.ID, Name: p.Name, Group: p.Group, Text: text})
	return nil
}

// award：描述者或白板判定某位玩家答對（給用講的猜中的人）。
func (r *Room) award(id string) error {
	t, err := r.drawing()
	if err != nil {
		return err
	}
	p := r.player(id)
	if p == nil {
		return errors.New("找不到這位玩家")
	}
	if p.ID == t.DescriberID {
		return errors.New("描述者不能得這個分")
	}
	if t.isCorrect(p.ID) {
		return errors.New("這位玩家已經答對了")
	}
	r.markCorrect(p, true)
	return nil
}

// markCorrect 計分：
//   - 個人：依答對順序 3／2／1 分，只要有人答對，描述者 +1（每回合只加一次）。
//   - 分組：第一個答對就結束本回合，答對者 +3、描述者 +2。
//     同組答對 → 該隊共 5 分；別組搶答 → 搶答隊 3 分、畫的那隊 2 分。
func (r *Room) markCorrect(p *Player, judge bool) {
	t := r.Turn
	t.Correct = append(t.Correct, p.ID)
	d := r.player(t.DescriberID)
	points, bonus := 3, 2
	if r.Settings.Mode == "solo" {
		points = 4 - len(t.Correct)
		if points < 1 {
			points = 1
		}
		bonus = 0
		if len(t.Correct) == 1 {
			bonus = 1
		}
	}
	p.Score += points
	if d != nil {
		d.Score += bonus
	}
	r.addFeed(&Guess{PlayerID: p.ID, Name: p.Name, Group: p.Group, Correct: true, ByJudge: judge, Points: points})
	if r.Settings.Mode == "team" || r.everyoneCorrect() {
		r.reveal()
	}
}
func (r *Room) everyoneCorrect() bool {
	t := r.Turn
	for _, p := range r.Players {
		if p.ID != t.DescriberID && p.Online && !t.isCorrect(p.ID) {
			return false
		}
	}
	return true
}
func (r *Room) reveal() {
	t := r.Turn
	t.Phase = "reveal"
	t.EndsAt = 0
	t.Answer = prompts[t.prompt].Name
	t.AnswerSVG = prompts[t.prompt].SVG
}
func (r *Room) endTurn() error {
	if _, err := r.drawing(); err != nil {
		return err
	}
	r.reveal()
	return nil
}
func (r *Room) skipPrompt() error {
	t, err := r.drawing()
	if err != nil {
		return err
	}
	if len(t.Correct) > 0 {
		return errors.New("已經有人答對，不能換題")
	}
	r.clearInk()
	t.Seq = r.seq
	t.prompt = r.drawPrompt()
	t.Skips++
	t.Guesses = []*Guess{}
	t.StartedAt = nowMs()
	if r.Settings.Seconds > 0 {
		t.EndsAt = t.StartedAt + int64(r.Settings.Seconds)*1000
	}
	return nil
}
func (r *Room) advance() error {
	t := r.Turn
	if r.Status != "playing" || t == nil || t.Phase != "reveal" {
		return errors.New("請先結束本回合")
	}
	if t.Number >= r.Settings.Rounds {
		r.Status = "finished"
		return nil
	}
	d := r.pickDescriber()
	if d == nil {
		r.Status = "finished"
		return nil
	}
	r.beginTurn(t.Number+1, d)
	return nil
}
func (r *Room) finish() error {
	if r.Status != "playing" {
		return errors.New("遊戲尚未開始")
	}
	if r.Turn != nil && r.Turn.Phase == "drawing" {
		r.reveal()
	}
	r.Status = "finished"
	return nil
}
func (r *Room) reset() error {
	if r.Status != "finished" {
		return errors.New("遊戲結束後才能再玩一場")
	}
	r.Status = "lobby"
	r.Round++
	r.Turn = nil
	r.clearInk()
	for _, p := range r.Players {
		p.Score, p.Described = 0, 0
	}
	return nil
}

func (r *Room) ink(p *Player, seq, id int, color string, width int, pts []int) (*Stroke, error) {
	t := r.Turn
	if t == nil || r.Status != "playing" || t.Phase != "drawing" || seq != t.Seq {
		return nil, errStale
	}
	if !r.isDrawer(p) {
		return nil, errors.New("這回合不是你畫")
	}
	if len(pts) == 0 || len(pts)%2 != 0 || len(pts) > maxMessagePts*2 {
		return nil, errors.New("筆跡格式錯誤")
	}
	for _, v := range pts {
		if v < 0 || v > canvasSize {
			return nil, errors.New("筆跡超出畫布")
		}
	}
	if p.inkInts+len(pts) > maxInkInts {
		return nil, errors.New("畫面太複雜了，請清除後再畫")
	}
	var last *Stroke
	if n := len(p.strokes); n > 0 {
		last = p.strokes[n-1]
	}
	if last == nil || last.ID != id {
		if last != nil && id < last.ID {
			return nil, errStale
		}
		if len(p.strokes) >= maxStrokes {
			return nil, errors.New("筆畫太多了，請清除後再畫")
		}
		if !palette[color] || width < 4 || width > 80 {
			return nil, errors.New("畫筆設定錯誤")
		}
		last = &Stroke{ID: id, Color: color, Width: width}
		p.strokes = append(p.strokes, last)
	}
	last.Pts = append(last.Pts, pts...)
	p.inkInts += len(pts)
	return last, nil
}
func (r *Room) editInk(p *Player, seq int, op string) error {
	t := r.Turn
	if t == nil || r.Status != "playing" || t.Phase != "drawing" || seq != t.Seq {
		return errStale
	}
	if !r.isDrawer(p) {
		return errors.New("這回合不是你畫")
	}
	if op == "clear" {
		p.strokes, p.inkInts = nil, 0
	} else if n := len(p.strokes); n > 0 {
		p.inkInts -= len(p.strokes[n-1].Pts)
		p.strokes = p.strokes[:n-1]
	}
	return nil
}

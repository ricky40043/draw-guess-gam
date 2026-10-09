import { defineStore } from "pinia";
import { computed, reactive, ref } from "vue";

export interface Settings {
  mode: "solo" | "team";
  rounds: number;
  seconds: number;
  groups: number;
  joinMode: "auto" | "name";
}
export interface Player {
  id: string;
  name: string;
  group?: string;
  score: number;
  described: number;
  online: boolean;
}
export interface Guess {
  id: number;
  playerId: string;
  name: string;
  group?: string;
  text?: string;
  correct: boolean;
  byJudge?: boolean;
  points?: number;
}
export interface Turn {
  seq: number;
  number: number;
  describerId: string;
  describerName: string;
  group?: string;
  phase: "drawing" | "reveal";
  startedAt: number;
  endsAt: number;
  correct: string[];
  guesses: Guess[];
  answer?: string;
  answerSvg?: string;
  skips: number;
}
export interface Room {
  id: string;
  settings: Settings;
  status: "lobby" | "playing" | "finished";
  players: Player[];
  turn?: Turn;
  round: number;
  hostOnline: boolean;
}
export interface Stroke {
  id: number;
  c: string;
  w: number;
  pts: number[];
}
interface Session {
  room: string;
  token: string;
  host: boolean;
}

const FLUSH_MS = 60;
const MAX_BATCH = 380; // 點數，後端上限 400

export const useGame = defineStore("game", () => {
  const room = ref<Room | null>(null),
    me = ref<{ id: string; name: string; group?: string } | null>(null),
    host = ref(false),
    prompt = ref<{ name: string; svg: string } | null>(null),
    connected = ref(false),
    ready = ref(false),
    error = ref(""),
    busy = ref(false),
    fatal = ref(false),
    clockOffset = ref(0),
    now = ref(Date.now()),
    // 每位玩家的畫布版本號：數字變了就重畫
    inkTick = reactive<Record<string, number>>({});
  // 筆跡資料量大，不放進 Vue 的響應式系統
  const canvases = new Map<string, Stroke[]>();
  let canvasSeq = -1;
  let socket: WebSocket | undefined,
    session: Session | undefined,
    retry: ReturnType<typeof setTimeout> | undefined,
    pendingTimer: ReturnType<typeof setTimeout> | undefined,
    flushTimer: ReturnType<typeof setTimeout> | undefined,
    outgoing: { s: number; c: string; w: number; pts: number[] } | null = null,
    leaveReturnUrl = "/";

  setInterval(() => (now.value = Date.now()), 250);

  const turn = computed(() => room.value?.turn ?? null);
  const isDescriber = computed(
    () => !!me.value && turn.value?.describerId === me.value.id,
  );
  const isJudge = computed(() => host.value || isDescriber.value);
  function isDrawer(p: { id: string; group?: string }) {
    const t = turn.value;
    if (!t || !room.value || p.id === t.describerId) return false;
    return room.value.settings.mode === "solo" || p.group === t.group;
  }
  const amDrawer = computed(() => !!me.value && isDrawer(me.value));
  const drawers = computed(() =>
    (room.value?.players ?? []).filter((p) => isDrawer(p)),
  );
  const remaining = computed(() => {
    const t = turn.value;
    if (!t || t.phase !== "drawing" || !t.endsAt) return null;
    return Math.max(0, Math.ceil((t.endsAt - (now.value + clockOffset.value)) / 1000));
  });

  function bump(id: string) {
    inkTick[id] = (inkTick[id] ?? 0) + 1;
  }
  function strokesOf(id: string) {
    return canvases.get(id) ?? [];
  }
  function resetCanvases(seq: number) {
    canvasSeq = seq;
    canvases.clear();
    outgoing = null;
    for (const id of Object.keys(inkTick)) bump(id);
  }
  function appendInk(id: string, s: number, c: string, w: number, pts: number[]) {
    let list = canvases.get(id);
    if (!list) canvases.set(id, (list = []));
    const last = list[list.length - 1];
    if (last && last.id === s) last.pts.push(...pts);
    else list.push({ id: s, c, w, pts: [...pts] });
    bump(id);
  }

  function settle() {
    busy.value = false;
    clearTimeout(pendingTimer);
  }
  function storageKey(s: { room: string; host: boolean }) {
    return `draw:${s.room}:${s.host ? "host" : "player"}`;
  }
  function connect() {
    socket = new WebSocket(
      `${location.protocol === "https:" ? "wss" : "ws"}://${location.host}/ws`,
    );
    socket.onopen = () => {
      connected.value = true;
      if (session) socket!.send(JSON.stringify({ type: "resume", ...session }));
      else ready.value = true;
    };
    socket.onclose = () => {
      connected.value = false;
      ready.value = false;
      settle();
      retry = setTimeout(connect, 1500);
    };
    socket.onerror = () => {
      error.value = "連線中斷，正在重新連線…";
    };
    socket.onmessage = (event) => {
      const data = JSON.parse(event.data);
      switch (data.type) {
        case "ink":
          if (data.t === canvasSeq) appendInk(data.p, data.s, data.c, data.w, data.pts);
          return;
        case "canvas":
          if (data.t === canvasSeq) {
            canvases.set(data.p, data.strokes ?? []);
            bump(data.p);
          }
          return;
        case "canvases":
          if (data.t !== canvasSeq) resetCanvases(data.t);
          for (const [id, strokes] of Object.entries(data.canvases as Record<string, Stroke[]>)) {
            // 自己正在畫的那一筆以本機為準，避免重連時被舊資料蓋掉
            if (id === me.value?.id && canvases.has(id)) continue;
            canvases.set(id, strokes);
            bump(id);
          }
          return;
        case "left":
          if (session) sessionStorage.removeItem(storageKey(session));
          session = undefined;
          settle();
          location.href = leaveReturnUrl;
          return;
        case "error":
          error.value = data.message;
          settle();
          if (session && !ready.value) {
            fatal.value = true;
            sessionStorage.removeItem(storageKey(session));
            session = undefined;
            ready.value = true;
          }
          return;
        case "session":
          session = { room: data.room, token: data.token, host: data.host };
          sessionStorage.setItem(storageKey(session), JSON.stringify(session));
          history.replaceState({}, "", `/?${data.host ? "host" : "room"}=${data.room}`);
          return;
        case "state": {
          const seq = data.room.turn?.seq ?? -1;
          if (seq !== canvasSeq) resetCanvases(seq);
          room.value = data.room;
          me.value = data.me;
          host.value = data.host;
          prompt.value = data.prompt;
          clockOffset.value = data.now - Date.now();
          ready.value = true;
          error.value = "";
          settle();
        }
      }
    };
  }
  function init() {
    const q = new URLSearchParams(location.search);
    const id = q.get("host") || q.get("room");
    if (id) {
      try {
        session =
          JSON.parse(sessionStorage.getItem(storageKey({ room: id, host: q.has("host") })) || "null") ||
          undefined;
      } catch {
        session = undefined;
      }
    }
    connect();
  }
  function send(data: object) {
    if (!ready.value || socket?.readyState !== WebSocket.OPEN) {
      error.value = "請等待連線恢復後再試";
      return false;
    }
    if (busy.value) return false;
    busy.value = true;
    error.value = "";
    socket.send(JSON.stringify(data));
    pendingTimer = setTimeout(() => {
      settle();
      error.value = "尚未收到確認，正在重新連線取得最新進度";
      socket?.close();
    }, 10000);
    return true;
  }
  // 筆跡與猜題不走 busy 鎖，否則畫畫時按鈕會一直卡住
  function sendRaw(data: object) {
    if (socket?.readyState === WebSocket.OPEN) socket.send(JSON.stringify(data));
  }
  function flush() {
    clearTimeout(flushTimer);
    flushTimer = undefined;
    if (!outgoing || !outgoing.pts.length) return;
    sendRaw({ type: "ink", t: canvasSeq, ...outgoing });
    outgoing = { ...outgoing, pts: [] };
  }

  // ---- 自己畫畫 ----
  function beginStroke(c: string, w: number, x: number, y: number) {
    if (!me.value) return;
    flush();
    const list = strokesOf(me.value.id);
    const s = (list[list.length - 1]?.id ?? 0) + 1;
    outgoing = { s, c, w, pts: [] };
    addPoint(x, y, true);
  }
  function addPoint(x: number, y: number, first = false) {
    if (!me.value || !outgoing) return;
    const pt = [Math.round(Math.min(1000, Math.max(0, x))), Math.round(Math.min(1000, Math.max(0, y)))];
    const list = strokesOf(me.value.id);
    const last = list[list.length - 1];
    if (!first && last) {
      const n = last.pts.length;
      if (Math.abs(last.pts[n - 2] - pt[0]) + Math.abs(last.pts[n - 1] - pt[1]) < 4) return;
    }
    appendInk(me.value.id, outgoing.s, outgoing.c, outgoing.w, pt);
    outgoing.pts.push(...pt);
    if (outgoing.pts.length >= MAX_BATCH * 2) flush();
    else if (!flushTimer) flushTimer = setTimeout(flush, FLUSH_MS);
  }
  function endStroke() {
    flush();
    outgoing = null;
  }
  function editInk(op: "undo" | "clear") {
    if (!me.value) return;
    endStroke();
    const list = strokesOf(me.value.id);
    canvases.set(me.value.id, op === "clear" ? [] : list.slice(0, -1));
    bump(me.value.id);
    sendRaw({ type: op, t: canvasSeq });
  }
  function guess(text: string) {
    error.value = "";
    sendRaw({ type: "guess", text });
  }

  function leave() {
    clearTimeout(retry);
    clearTimeout(pendingTimer);
    if (session) {
      sessionStorage.removeItem(storageKey(session));
      session = undefined;
    }
    if (socket) {
      socket.onclose = null;
      socket.close();
    }
    location.href = "/";
  }
  function leaveRoom(returnUrl: string) {
    leaveReturnUrl = returnUrl;
    send({ type: "leave" });
  }
  return {
    room, me, host, prompt, connected, ready, error, busy, fatal, now, inkTick,
    turn, isDescriber, isJudge, amDrawer, drawers, remaining,
    isDrawer, strokesOf, init, send, guess, beginStroke, addPoint, endStroke, editInk,
    leave, leaveRoom,
  };
});

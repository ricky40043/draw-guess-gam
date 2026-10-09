<script setup lang="ts">
import { computed, ref, watch } from "vue";
import { useGame } from "../store";
import { ding } from "../sound";
import CanvasCard from "../components/CanvasCard.vue";
import GuessBox from "../components/GuessBox.vue";
import GuessFeed from "../components/GuessFeed.vue";
import InkCanvas from "../components/InkCanvas.vue";
import PromptArt from "../components/PromptArt.vue";
import Scoreboard from "../components/Scoreboard.vue";
import TimerBadge from "../components/TimerBadge.vue";

const game = useGame();
const room = computed(() => game.room!);
const turn = computed(() => game.turn!);
const team = computed(() => room.value.settings.mode === "team");
const reveal = computed(() => turn.value.phase === "reveal");
const lastRound = computed(() => turn.value.number >= room.value.settings.rounds);
const role = computed(() => {
  if (game.host) return "host";
  if (game.isDescriber) return "describer";
  if (game.amDrawer) return "drawer";
  return "watcher";
});
const zoomId = ref<string | null>(null);
const zoomPlayer = computed(() => room.value.players.find((p) => p.id === zoomId.value) ?? null);
const peek = ref(false);
const teammates = computed(() => game.drawers.filter((p) => p.id !== game.me?.id));
const correctNames = computed(() =>
  turn.value.correct.map((id) => room.value.players.find((p) => p.id === id)?.name ?? "?"),
);

// 畫筆
const colors = [
  { v: "#1f2a24", n: "黑" },
  { v: "#e4572e", n: "紅" },
  { v: "#2e86de", n: "藍" },
  { v: "#2fa24f", n: "綠" },
  { v: "#f2b705", n: "黃" },
  { v: "#8e44ad", n: "紫" },
];
const sizes = [
  { v: 8, n: "細" },
  { v: 16, n: "中" },
  { v: 32, n: "粗" },
];
const color = ref(colors[0].v);
const size = ref(sizes[0].v);
const eraser = ref(false);
const confirmClear = ref(false);
function clearAll() {
  if (!confirmClear.value) {
    confirmClear.value = true;
    setTimeout(() => (confirmClear.value = false), 2500);
    return;
  }
  confirmClear.value = false;
  game.editInk("clear");
}

watch(
  () => turn.value.correct.length,
  (n, old) => {
    if (n > (old ?? 0)) ding(turn.value.correct[n - 1] === game.me?.id);
  },
);
watch(
  () => turn.value.seq,
  () => {
    zoomId.value = null;
    peek.value = false;
    eraser.value = false;
  },
);
</script>
<template>
  <main class="game" :class="[`role-${role}`, { reveal }]">
    <!-- 狀態列 -->
    <header class="statusbar">
      <span class="round">第 <b>{{ turn.number }}</b> / {{ room.settings.rounds }} 回合</span>
      <span class="who">
        🎙 <b>{{ turn.describerName }}</b>
        <small v-if="team" class="tag">{{ turn.group }}</small>
        {{ reveal ? "描述完畢" : "描述中" }}
      </span>
      <TimerBadge v-if="!reveal" />
      <span v-else class="timer done">揭曉</span>
    </header>

    <!-- ========== 揭曉 ========== -->
    <section v-if="reveal" class="reveal-stage">
      <div class="answer-card">
        <PromptArt v-if="turn.answerSvg" :svg="turn.answerSvg" :label="turn.answer" />
        <div>
          <p class="eyebrow">答案是</p>
          <h2 class="answer">{{ turn.answer }}</h2>
          <p v-if="correctNames.length" class="muted">答對：{{ correctNames.join("、") }}</p>
          <p v-else class="muted">這題沒人猜到 😅</p>
          <button
            v-if="game.isJudge"
            class="primary big"
            :disabled="game.busy"
            @click="game.send({ type: 'next' })"
          >
            {{ lastRound ? "看最終結果 🏆" : "下一回合 →" }}
          </button>
          <p v-else class="muted small">等 {{ turn.describerName }} 或白板按「下一回合」</p>
        </div>
      </div>
      <div class="gallery">
        <CanvasCard v-for="p in game.drawers" :key="p.id" :player="p" @zoom="zoomId = $event" />
      </div>
      <aside class="side"><Scoreboard /></aside>
    </section>

    <!-- ========== 白板 ========== -->
    <section v-else-if="role === 'host'" class="host-stage">
      <div class="host-tools">
        <button class="ghost" @pointerdown="peek = true" @pointerup="peek = false" @pointerleave="peek = false">
          {{ peek ? game.prompt?.name : "按住偷看答案" }}
        </button>
        <button class="ghost" :disabled="game.busy || turn.correct.length > 0" @click="game.send({ type: 'skip' })">
          換一題
        </button>
        <button class="ghost" :disabled="game.busy" @click="game.send({ type: 'end_turn' })">結束本回合</button>
      </div>
      <div class="canvas-grid" :class="{ few: game.drawers.length <= 4 }">
        <CanvasCard v-for="p in game.drawers" :key="p.id" :player="p" can-award @zoom="zoomId = $event" />
      </div>
      <aside class="side">
        <h3>猜題區</h3>
        <GuessFeed :guesses="turn.guesses" :limit="14" />
        <h3>{{ team ? "各組分數" : "排行榜" }}</h3>
        <Scoreboard :limit="10" />
      </aside>
    </section>

    <!-- ========== 描述者 ========== -->
    <section v-else-if="role === 'describer'" class="describer-stage">
      <div class="prompt-card">
        <PromptArt v-if="game.prompt" :svg="game.prompt.svg" :label="game.prompt.name" />
        <div>
          <p class="eyebrow">你的題目</p>
          <h2>{{ game.prompt?.name }}</h2>
          <p class="muted small">只能用講的形容「怎麼畫」，不能說出答案！</p>
          <div class="btn-row">
            <button class="ghost" :disabled="game.busy || turn.correct.length > 0" @click="game.send({ type: 'skip' })">
              換一題
            </button>
            <button class="ghost" :disabled="game.busy" @click="game.send({ type: 'end_turn' })">結束本回合</button>
          </div>
        </div>
      </div>
      <p class="hint">有人用講的猜中？點他畫面下的「判定答對」。</p>
      <div class="canvas-grid two">
        <CanvasCard v-for="p in game.drawers" :key="p.id" :player="p" can-award @zoom="zoomId = $event" />
      </div>
      <GuessFeed :guesses="turn.guesses" :limit="10" />
    </section>

    <!-- ========== 畫畫的人 ========== -->
    <section v-else-if="role === 'drawer'" class="drawer-stage">
      <div class="pad">
        <InkCanvas
          v-if="game.me"
          :player-id="game.me.id"
          editable
          :color="eraser ? '#ffffff' : color"
          :width="eraser ? 48 : size"
        />
      </div>
      <div class="toolbar" role="toolbar" aria-label="畫筆">
        <button
          v-for="c in colors"
          :key="c.v"
          class="swatch"
          :class="{ on: !eraser && color === c.v }"
          :style="{ background: c.v }"
          :aria-label="c.n"
          @click="(color = c.v), (eraser = false)"
        />
        <span class="sep" />
        <button
          v-for="s in sizes"
          :key="s.v"
          class="size"
          :class="{ on: !eraser && size === s.v }"
          :aria-label="`${s.n}筆`"
          @click="(size = s.v), (eraser = false)"
        >
          <i :style="{ width: s.v / 2 + 4 + 'px', height: s.v / 2 + 4 + 'px' }" />
        </button>
        <span class="sep" />
        <button class="tool" :class="{ on: eraser }" @click="eraser = !eraser">橡皮擦</button>
        <button class="tool" @click="game.editInk('undo')">↶ 復原</button>
        <button class="tool" :class="{ danger: confirmClear }" @click="clearAll">
          {{ confirmClear ? "確定清除？" : "清除" }}
        </button>
      </div>
      <GuessBox />
      <div v-if="team && teammates.length" class="mates">
        <p class="muted small">組員的畫</p>
        <div class="mate-strip">
          <CanvasCard v-for="p in teammates" :key="p.id" :player="p" @zoom="zoomId = $event" />
        </div>
      </div>
      <GuessFeed :guesses="turn.guesses" :limit="5" />
    </section>

    <!-- ========== 旁觀搶答（分組賽其他組） ========== -->
    <section v-else class="watch-stage">
      <p class="hint big">
        <template v-if="team"><b>{{ turn.group }}</b> 正在畫，看圖、聽描述，搶先猜出來！</template>
        <template v-else>這回合你在旁邊看，一起猜吧！</template>
      </p>
      <GuessBox />
      <div class="canvas-grid two">
        <CanvasCard v-for="p in game.drawers" :key="p.id" :player="p" @zoom="zoomId = $event" />
      </div>
      <GuessFeed :guesses="turn.guesses" :limit="8" />
    </section>

    <!-- 放大檢視 -->
    <div v-if="zoomPlayer" class="overlay" @click.self="zoomId = null">
      <div class="zoom" role="dialog" aria-modal="true" :aria-label="`${zoomPlayer.name} 的畫`">
        <header>
          <b>{{ zoomPlayer.name }}</b>
          <button class="close" aria-label="關閉" @click="zoomId = null">×</button>
        </header>
        <InkCanvas :player-id="zoomPlayer.id" />
        <button
          v-if="game.isJudge && !reveal && !turn.correct.includes(zoomPlayer.id)"
          class="primary big"
          :disabled="game.busy"
          @click="game.send({ type: 'award', id: zoomPlayer.id }), (zoomId = null)"
        >
          判定 {{ zoomPlayer.name }} 答對
        </button>
      </div>
    </div>
  </main>
</template>

<script setup lang="ts">
import { computed, onMounted, ref, watch } from "vue";
import QRCode from "qrcode";
import { useGame } from "./store";
import SetupView from "./views/SetupView.vue";
import JoinView from "./views/JoinView.vue";
import LobbyView from "./views/LobbyView.vue";
import GameView from "./views/GameView.vue";
import FinishedView from "./views/FinishedView.vue";

const game = useGame();
const params = new URLSearchParams(location.search);
const joinRoomId = (params.get("room") || "").toUpperCase();
const baseUrl = ref(location.origin);
const promptCount = ref(0);
const qr = ref("");
const joinUrl = computed(() => (game.room ? `${baseUrl.value}/?room=${game.room.id}` : ""));
const status = computed(() => game.room?.status);

watch(
  joinUrl,
  async (url) => {
    if (!url) return;
    qr.value = await QRCode.toDataURL(url, { width: 640, margin: 1, color: { dark: "#231f20", light: "#ffffff" } });
  },
  { immediate: true },
);
onMounted(async () => {
  game.init();
  try {
    const config = await (await fetch("/api/config")).json();
    if (config.frontendUrl) baseUrl.value = config.frontendUrl.replace(/\/$/, "");
    promptCount.value = config.prompts;
  } catch {
    /* 用目前網址 */
  }
});
function exit() {
  if (game.host && status.value === "playing" && !confirmExit.value) {
    confirmExit.value = true;
    setTimeout(() => (confirmExit.value = false), 3000);
    return;
  }
  if (game.host && status.value === "playing") game.send({ type: "end" });
  else game.leave();
}
const confirmExit = ref(false);
// 離開回首頁：保留身分，之後重新掃碼或從首頁「回到房間」會回到同一個房間、同一個身分
const confirmLeave = ref(false);
function leaveToHome() {
  if (!confirmLeave.value) {
    confirmLeave.value = true;
    setTimeout(() => (confirmLeave.value = false), 3000);
    return;
  }
  game.leave(true);
}
const roomGone = computed(() => game.error.includes("找不到房間"));
</script>
<template>
  <div class="app" :class="{ wide: game.host, fit: game.host && status === 'lobby' }">
    <header class="topbar">
      <a class="logo" href="/" aria-label="你畫我猜 首頁"><span class="logo-mark">✎</span>你畫我猜</a>
      <span v-if="game.room" class="room-code">ROOM <b>{{ game.room.id }}</b></span>
      <span class="spacer" />
      <span v-if="!game.connected" class="conn">連線中…</span>
      <span v-if="game.me" class="me-chip">{{ game.me.name }}<small v-if="game.me.group"> · {{ game.me.group }}</small></span>
      <button v-if="game.host && status === 'playing'" class="text-btn" @click="exit">
        {{ confirmExit ? "確定提前結束？" : "結束遊戲" }}
      </button>
      <button v-if="game.room" class="ghost small-btn" :class="{ warn: confirmLeave }" @click="leaveToHome">
        {{ confirmLeave ? "再按一次離開" : "離開 ↩" }}
      </button>
    </header>
    <p v-if="game.error && !game.fatal" class="toast" role="alert">{{ game.error }}</p>

    <template v-if="!game.room">
      <main v-if="game.fatal" class="panel center lonely">
        <div class="avatar-blob">{{ roomGone ? "🏚️" : "🤔" }}</div>
        <h2>{{ roomGone ? "這個房間已經不存在了" : "無法回到房間" }}</h2>
        <p class="muted">
          {{ roomGone ? "房間可能已結束、閒置太久被清除，或伺服器重新啟動過。" : game.error }}
        </p>
        <div class="btn-row center">
          <a v-if="joinRoomId && !roomGone" class="primary big" :href="`/?room=${joinRoomId}`">重新加入</a>
          <a :class="roomGone ? 'primary big' : 'ghost'" href="/">回首頁</a>
        </div>
      </main>
      <p v-else-if="!game.ready" class="muted center lonely">連線中…</p>
      <JoinView v-else-if="joinRoomId" :room-id="joinRoomId" />
      <SetupView v-else :prompt-count="promptCount" />
    </template>
    <LobbyView v-else-if="status === 'lobby'" :join-url="joinUrl" :qr="qr" />
    <template v-else-if="status === 'playing' && game.turn">
      <GameView />
      <aside v-if="game.host" class="mini-qr" :title="joinUrl">
        <img v-if="qr" :src="qr" alt="加入 QR Code" />
        <span>中途加入</span>
      </aside>
    </template>
    <FinishedView v-else-if="status === 'finished'" />
  </div>
</template>

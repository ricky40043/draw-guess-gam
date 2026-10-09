<script setup lang="ts">
import { computed, ref } from "vue";
import { useGame } from "../store";
const props = defineProps<{ joinUrl: string; qr: string }>();
const game = useGame();
const room = computed(() => game.room!);
const groups = computed(() =>
  room.value.settings.mode === "team"
    ? Array.from({ length: room.value.settings.groups }, (_, i) => {
        const label = String.fromCharCode(65 + i) + "組";
        return { label, members: room.value.players.filter((p) => p.group === label) };
      })
    : [{ label: "", members: room.value.players }],
);
const secondsText = computed(() => {
  const s = room.value.settings.seconds;
  return s ? (s % 60 ? `${s} 秒` : `${s / 60} 分鐘`) : "不限時";
});
const copied = ref<"" | "ok" | "fail">("");
const linkInput = ref<HTMLInputElement>();
// 區網 http 網址不是安全來源，navigator.clipboard 會是 undefined，
// 所以退回「選取輸入框文字 + execCommand」的舊做法。
async function copy() {
  let ok = false;
  try {
    if (navigator.clipboard && window.isSecureContext) {
      await navigator.clipboard.writeText(props.joinUrl);
      ok = true;
    }
  } catch {
    ok = false;
  }
  if (!ok && linkInput.value) {
    linkInput.value.focus();
    linkInput.value.select();
    try {
      ok = document.execCommand("copy");
    } catch {
      ok = false;
    }
  }
  copied.value = ok ? "ok" : "fail";
  setTimeout(() => (copied.value = ""), 2500);
}
</script>
<template>
  <main v-if="game.host" class="lobby-host">
    <section class="qr-panel">
      <p class="eyebrow">掃描加入 · ROOM {{ room.id }}</p>
      <img v-if="qr" :src="qr" alt="加入房間的 QR Code" class="qr" />
      <div class="link-row">
        <input
          ref="linkInput"
          class="link-text"
          :value="joinUrl"
          readonly
          aria-label="加入連結"
          @focus="($event.target as HTMLInputElement).select()"
        />
        <button class="primary" @click="copy">{{ copied === "ok" ? "已複製 ✓" : "複製連結" }}</button>
      </div>
      <p v-if="copied === 'fail'" class="muted small">瀏覽器不讓自動複製，連結已幫你選取，請按 ⌘C / Ctrl+C。</p>
      <ul class="rule-chips">
        <li>{{ room.settings.mode === "team" ? `分組賽 · ${room.settings.groups} 組` : "個人賽" }}</li>
        <li>{{ room.settings.rounds }} 回合</li>
        <li>每回合 {{ secondsText }}</li>
      </ul>
      <p class="muted small score-rule">
        <template v-if="room.settings.mode === 'team'">
          計分：第一個猜對就結束本回合；同組猜對該隊 +5，別組搶答則搶答隊 +3、畫的那隊 +2
        </template>
        <template v-else>計分：依猜對順序 3／2／1 分；只要有人猜對，描述者 +1</template>
      </p>
    </section>
    <section class="roster">
      <header>
        <h2>已加入 <b>{{ room.players.length }}</b> 人</h2>
        <button class="primary big" :disabled="game.busy" @click="game.send({ type: 'start' })">開始遊戲 ▶</button>
      </header>
      <div class="roster-groups" :class="{ team: room.settings.mode === 'team' }">
        <div v-for="g in groups" :key="g.label" class="roster-group">
          <h3 v-if="g.label">{{ g.label }} <small>{{ g.members.length }} 人</small></h3>
          <ul>
            <li v-for="p in g.members" :key="p.id" :class="{ offline: !p.online }">{{ p.name }}</li>
            <li v-if="!g.members.length" class="empty">等待加入…</li>
          </ul>
        </div>
      </div>
    </section>
  </main>
  <main v-else class="lobby-player">
    <div class="panel center">
      <p class="eyebrow">ROOM {{ room.id }}</p>
      <div class="avatar-blob">✏️</div>
      <h2>{{ game.me?.name }}</h2>
      <p v-if="game.me?.group" class="tag big">{{ game.me.group }}</p>
      <p class="muted">已加入！看著白板，等主持人開始。</p>
      <p class="muted small">目前 {{ room.players.length }} 人 · {{ room.settings.rounds }} 回合</p>
      <button
        v-if="room.settings.mode === 'team'"
        class="ghost"
        :disabled="game.busy"
        @click="game.leaveRoom(joinUrl)"
      >
        選錯組了？重新選組
      </button>
    </div>
  </main>
</template>

<script setup lang="ts">
import { onMounted, ref } from "vue";
import { useGame } from "../store";
const props = defineProps<{ roomId: string }>();
const game = useGame();
const info = ref<{ mode: string; joinMode: string; groups: number; status: string; groupCounts: Record<string, number> } | null>(null);
const missing = ref(false);
const name = ref(localStorage.getItem("draw:name") ?? "");
const group = ref("");
onMounted(async () => {
  try {
    const res = await fetch(`/api/rooms/${encodeURIComponent(props.roomId)}`);
    if (!res.ok) throw new Error();
    info.value = await res.json();
  } catch {
    missing.value = true;
  }
});
function label(i: number) {
  return String.fromCharCode(65 + i) + "組";
}
function join() {
  try {
    localStorage.setItem("draw:name", name.value.trim());
  } catch {
    /* 無痕模式 */
  }
  game.send({ type: "join", room: props.roomId, name: name.value, group: group.value });
}
</script>
<template>
  <main class="join">
    <div v-if="missing" class="panel center">
      <h2>找不到房間 {{ roomId }}</h2>
      <p class="muted">房間可能已過期，請再掃一次 QR Code。</p>
      <a class="primary big" href="/">回首頁</a>
    </div>
    <form v-else-if="info" class="panel" @submit.prevent="join">
      <p class="eyebrow">ROOM {{ roomId }}</p>
      <h2>加入你畫我猜</h2>
      <p v-if="info.status !== 'lobby'" class="notice">遊戲進行中，加入後從下一回合開始參與。</p>
      <label v-if="info.joinMode === 'name'" class="field">
        <span>你的名字</span>
        <input v-model="name" maxlength="20" required placeholder="大家會看到這個名字" autocomplete="nickname" />
      </label>
      <p v-else class="muted">系統會幫你取一個可愛的暱稱。</p>
      <fieldset v-if="info.mode === 'team'">
        <legend>選擇組別</legend>
        <div class="group-grid">
          <label v-for="i in info.groups" :key="i" class="group-pick" :class="{ on: group === label(i - 1) }">
            <input v-model="group" type="radio" :value="label(i - 1)" required />
            <b>{{ label(i - 1) }}</b>
            <small>{{ info.groupCounts[label(i - 1)] ?? 0 }} 人</small>
          </label>
        </div>
      </fieldset>
      <button
        class="primary big"
        :disabled="game.busy || !game.ready || (info.mode === 'team' && !group) || (info.joinMode === 'name' && !name.trim())"
      >
        加入遊戲 →
      </button>
    </form>
    <p v-else class="muted center">讀取房間中…</p>
  </main>
</template>

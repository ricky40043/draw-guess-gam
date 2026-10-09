<script setup lang="ts">
import { computed, onMounted } from "vue";
import confetti from "canvas-confetti";
import { useGame } from "../store";
const game = useGame();
const room = computed(() => game.room!);
const team = computed(() => room.value.settings.mode === "team");
const players = computed(() => [...room.value.players].sort((a, b) => b.score - a.score));
const teams = computed(() =>
  Array.from({ length: room.value.settings.groups }, (_, i) => {
    const label = String.fromCharCode(65 + i) + "組";
    const members = room.value.players.filter((p) => p.group === label).sort((a, b) => b.score - a.score);
    return { label, members, score: members.reduce((s, p) => s + p.score, 0) };
  })
    .filter((t) => t.members.length)
    .sort((a, b) => b.score - a.score),
);
// 同分同名次
function rankOf(list: { score: number }[], i: number) {
  return list.findIndex((x) => x.score === list[i].score) + 1;
}
const podium = computed(() => (team.value ? teams.value : players.value).slice(0, 3));
onMounted(() => {
  if (matchMedia("(prefers-reduced-motion: reduce)").matches) return;
  confetti({ particleCount: 140, spread: 80, origin: { y: 0.4 } });
});
</script>
<template>
  <main class="finished">
    <p class="eyebrow">GAME OVER</p>
    <h1>{{ team ? "冠軍隊伍" : "最佳畫家" }}</h1>
    <ol class="podium">
      <li v-for="(x, i) in podium" :key="'label' in x ? x.label : x.id" :class="`place-${rankOf(podium, i)}`">
        <span class="medal">{{ ["🥇", "🥈", "🥉"][rankOf(podium, i) - 1] }}</span>
        <b>{{ "label" in x ? x.label : x.name }}</b>
        <span class="pts">{{ x.score }} 分</span>
      </li>
    </ol>
    <section class="final-table">
      <template v-if="team">
        <div v-for="(t, i) in teams" :key="t.label" class="final-team">
          <h3>{{ rankOf(teams, i) }}. {{ t.label }} <b>{{ t.score }} 分</b></h3>
          <p class="muted">{{ t.members.map((m) => `${m.name} ${m.score}`).join("　") }}</p>
        </div>
      </template>
      <ol v-else class="player-scores">
        <li v-for="(p, i) in players" :key="p.id" :class="{ me: p.id === game.me?.id }">
          <span class="rank">{{ rankOf(players, i) }}</span><span class="pname">{{ p.name }}</span><b>{{ p.score }}</b>
        </li>
      </ol>
    </section>
    <div v-if="game.host" class="btn-row center">
      <button class="primary big" :disabled="game.busy" @click="game.send({ type: 'reset' })">再玩一場 ↻</button>
      <button class="ghost" @click="game.leave()">建立新房間</button>
    </div>
    <p v-else class="muted center">等主持人決定要不要再玩一場…</p>
  </main>
</template>

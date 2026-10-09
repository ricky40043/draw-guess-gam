<script setup lang="ts">
import { computed } from "vue";
import { useGame } from "../store";
const props = defineProps<{ limit?: number }>();
const game = useGame();
const players = computed(() =>
  [...(game.room?.players ?? [])].sort((a, b) => b.score - a.score).slice(0, props.limit ?? 999),
);
const teams = computed(() => {
  const room = game.room;
  if (!room || room.settings.mode !== "team") return [];
  return Array.from({ length: room.settings.groups }, (_, i) => {
    const label = String.fromCharCode(65 + i) + "組";
    const members = room.players.filter((p) => p.group === label);
    return { label, members, score: members.reduce((s, p) => s + p.score, 0) };
  })
    .filter((t) => t.members.length)
    .sort((a, b) => b.score - a.score);
});
</script>
<template>
  <div class="scoreboard">
    <ol v-if="teams.length" class="team-scores">
      <li v-for="t in teams" :key="t.label" :class="{ active: game.turn?.group === t.label }">
        <span class="team-name">{{ t.label }}</span>
        <span class="team-members">{{ t.members.map((m) => m.name).join("、") }}</span>
        <b>{{ t.score }}</b>
      </li>
    </ol>
    <ol v-else class="player-scores">
      <li
        v-for="(p, i) in players"
        :key="p.id"
        :class="{ me: p.id === game.me?.id, offline: !p.online, describer: p.id === game.turn?.describerId }"
      >
        <span class="rank">{{ i + 1 }}</span>
        <span class="pname">{{ p.name }}<small v-if="p.id === game.turn?.describerId"> 🎙</small></span>
        <b>{{ p.score }}</b>
      </li>
    </ol>
  </div>
</template>

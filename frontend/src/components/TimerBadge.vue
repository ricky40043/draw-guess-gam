<script setup lang="ts">
import { computed } from "vue";
import { useGame } from "../store";
const game = useGame();
const text = computed(() => {
  const r = game.remaining;
  if (r === null) return "不限時";
  return `${Math.floor(r / 60)}:${String(r % 60).padStart(2, "0")}`;
});
</script>
<template>
  <span
    class="timer"
    :class="{ urgent: game.remaining !== null && game.remaining <= 10 }"
    role="timer"
    >⏱ {{ text }}</span
  >
</template>

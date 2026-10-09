<script setup lang="ts">
import { computed, ref } from "vue";
import { useGame } from "../store";
const game = useGame();
const text = ref("");
const done = computed(() => !!game.me && !!game.turn?.correct.includes(game.me.id));
function submit() {
  const v = text.value.trim();
  if (!v) return;
  game.guess(v);
  text.value = "";
}
</script>
<template>
  <div v-if="done" class="guess-done">🎉 你答對了！繼續畫，或幫大家加油</div>
  <form v-else class="guess-box" @submit.prevent="submit">
    <input
      v-model="text"
      maxlength="30"
      placeholder="猜猜看是什麼？"
      enterkeyhint="send"
      autocomplete="off"
      aria-label="輸入答案"
    />
    <button class="primary" :disabled="!text.trim()">猜！</button>
  </form>
</template>

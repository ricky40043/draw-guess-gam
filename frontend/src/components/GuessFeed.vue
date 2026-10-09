<script setup lang="ts">
import { computed } from "vue";
import type { Guess } from "../store";
const props = defineProps<{ guesses: Guess[]; limit?: number }>();
const list = computed(() => [...props.guesses].reverse().slice(0, props.limit ?? 40));
</script>
<template>
  <ol class="feed" aria-live="polite">
    <li v-if="!list.length" class="feed-empty">還沒有人猜，大膽喊出來！</li>
    <li v-for="g in list" :key="g.id" :class="{ correct: g.correct }">
      <b>{{ g.name }}</b>
      <small v-if="g.group" class="tag">{{ g.group }}</small>
      <template v-if="g.correct">
        {{ g.byJudge ? "被判定答對" : "答對了" }}！<em>+{{ g.points }}</em>
      </template>
      <span v-else class="guess-text">{{ g.text }}</span>
    </li>
  </ol>
</template>

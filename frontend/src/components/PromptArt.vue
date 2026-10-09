<script setup lang="ts">
// 題目 SVG 來自伺服器內建題庫（prompts.json），不含使用者輸入
defineProps<{ svg: string; label?: string }>();
</script>
<template>
  <svg
    class="prompt-art"
    viewBox="0 0 100 100"
    fill="none"
    stroke="currentColor"
    stroke-width="3"
    stroke-linecap="round"
    stroke-linejoin="round"
    role="img"
    :aria-label="label"
    v-html="svg"
  />
</template>

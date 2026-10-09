<script setup lang="ts">
import { computed } from "vue";
import { useGame, type Player } from "../store";
import InkCanvas from "./InkCanvas.vue";
const props = defineProps<{ player: Player; canAward?: boolean }>();
const emit = defineEmits<{ zoom: [id: string] }>();
const game = useGame();
const correct = computed(() => !!game.turn?.correct.includes(props.player.id));
</script>
<template>
  <figure class="canvas-card" :class="{ correct, offline: !player.online }">
    <button class="canvas-zoom" :aria-label="`放大 ${player.name} 的畫`" @click="emit('zoom', player.id)">
      <InkCanvas :player-id="player.id" />
    </button>
    <figcaption>
      <span class="cname">{{ player.name }}</span>
      <span v-if="correct" class="badge ok">✓ 答對</span>
      <button
        v-else-if="canAward && game.turn?.phase === 'drawing'"
        class="award"
        :disabled="game.busy"
        @click="game.send({ type: 'award', id: player.id })"
      >
        判定答對
      </button>
    </figcaption>
  </figure>
</template>

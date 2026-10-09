<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref, watch } from "vue";
import { useGame, type Stroke } from "../store";

const props = defineProps<{
  playerId: string;
  editable?: boolean;
  color?: string;
  width?: number;
}>();
const game = useGame();
const canvas = ref<HTMLCanvasElement>();
let ctx: CanvasRenderingContext2D | null = null;
let scale = 1;
// 增量繪製：記住上次畫到哪一筆、哪一點
let drawnList: Stroke[] | null = null;
let drawnStrokes = 0;
let drawnPts = 0;
let frame = 0;
let observer: ResizeObserver | undefined;
let drawing = false;

function styleFor(s: Stroke) {
  ctx!.strokeStyle = s.c;
  ctx!.fillStyle = s.c;
  ctx!.lineWidth = s.w * scale;
}
function segment(s: Stroke, from: number) {
  const p = s.pts;
  if (from === 0) {
    if (p.length === 2) {
      ctx!.beginPath();
      ctx!.arc(p[0] * scale, p[1] * scale, (s.w * scale) / 2, 0, Math.PI * 2);
      ctx!.fill();
      return;
    }
    from = 2;
  }
  if (from >= p.length) return;
  ctx!.beginPath();
  ctx!.moveTo(p[from - 2] * scale, p[from - 1] * scale);
  for (let i = from; i < p.length; i += 2) ctx!.lineTo(p[i] * scale, p[i + 1] * scale);
  ctx!.stroke();
}
function paint() {
  frame = 0;
  if (!ctx || !canvas.value) return;
  const list = game.strokesOf(props.playerId);
  if (list !== drawnList || list.length < drawnStrokes) {
    ctx.fillStyle = "#fff";
    ctx.fillRect(0, 0, canvas.value.width, canvas.value.height);
    drawnList = list;
    drawnStrokes = 0;
    drawnPts = 0;
  }
  for (let i = Math.max(0, drawnStrokes - 1); i < list.length; i++) {
    const s = list[i];
    const from = i === drawnStrokes - 1 ? drawnPts : 0;
    styleFor(s);
    segment(s, from);
    drawnPts = s.pts.length;
  }
  drawnStrokes = list.length;
}
function schedule(full = false) {
  if (full) drawnList = null;
  if (!frame) frame = requestAnimationFrame(paint);
}
function resize() {
  const el = canvas.value;
  if (!el) return;
  const size = Math.round(el.clientWidth * Math.min(2, devicePixelRatio || 1));
  if (size && el.width !== size) {
    el.width = el.height = size;
    scale = size / 1000;
    ctx = el.getContext("2d");
    if (ctx) {
      ctx.lineCap = "round";
      ctx.lineJoin = "round";
    }
    schedule(true);
  }
}
watch(() => game.inkTick[props.playerId], () => schedule());
watch(() => props.playerId, () => schedule(true));
onMounted(() => {
  resize();
  observer = new ResizeObserver(resize);
  observer.observe(canvas.value!);
});
onBeforeUnmount(() => {
  observer?.disconnect();
  cancelAnimationFrame(frame);
  if (drawing) game.endStroke();
});

function point(e: PointerEvent) {
  const r = canvas.value!.getBoundingClientRect();
  return [((e.clientX - r.left) * 1000) / r.width, ((e.clientY - r.top) * 1000) / r.height];
}
function down(e: PointerEvent) {
  if (!props.editable || (e.pointerType === "mouse" && e.button !== 0)) return;
  e.preventDefault();
  drawing = true;
  canvas.value!.setPointerCapture(e.pointerId);
  const [x, y] = point(e);
  game.beginStroke(props.color ?? "#1f2a24", props.width ?? 10, x, y);
}
function move(e: PointerEvent) {
  if (!drawing) return;
  const events = e.getCoalescedEvents?.() ?? [e];
  for (const ev of events.length ? events : [e]) {
    const [x, y] = point(ev);
    game.addPoint(x, y);
  }
}
function up() {
  if (!drawing) return;
  drawing = false;
  game.endStroke();
}
</script>
<template>
  <canvas
    ref="canvas"
    class="ink"
    :class="{ editable }"
    @pointerdown="down"
    @pointermove="move"
    @pointerup="up"
    @pointercancel="up"
    @lostpointercapture="up"
  />
</template>

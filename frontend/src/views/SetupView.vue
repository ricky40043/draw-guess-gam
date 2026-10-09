<script setup lang="ts">
import { reactive, ref } from "vue";
import { useGame, type Settings } from "../store";
defineProps<{ promptCount: number }>();
const game = useGame();
const settings = reactive<Settings>({ mode: "solo", rounds: 10, seconds: 120, groups: 2, joinMode: "name" });
const code = ref("");
const secondOptions = [
  { v: 60, label: "60 秒" },
  { v: 90, label: "90 秒" },
  { v: 120, label: "2 分鐘" },
  { v: 180, label: "3 分鐘" },
  { v: 0, label: "不限時" },
];
function create() {
  game.send({ type: "create", settings: { ...settings, rounds: Number(settings.rounds) } });
}
function join() {
  const id = code.value.trim().toUpperCase();
  if (id) location.href = `/?room=${encodeURIComponent(id)}`;
}
</script>
<template>
  <main class="setup">
    <section class="hero">
      <p class="eyebrow">DRAW · DESCRIBE · GUESS</p>
      <h1>你<span class="hl">說</span>我畫<br />大家<span class="hl">猜</span></h1>
      <p class="lead">
        描述者看著題目用嘴巴形容，其他人用手機照著畫，邊畫邊猜。內建 {{ promptCount || "100+" }} 張簡單線條題目，掃 QR Code 就能玩。
      </p>
      <ol class="how">
        <li><b>1</b>白板電腦建立房間，大家掃 QR Code 加入</li>
        <li><b>2</b>輪到的人看題目、只能用講的描述</li>
        <li><b>3</b>其他人照著畫，打字或喊出答案</li>
      </ol>
    </section>
    <form class="panel" @submit.prevent="create">
      <h2>建立房間</h2>
      <fieldset>
        <legend>遊戲模式</legend>
        <div class="choice-row">
          <label class="choice" :class="{ on: settings.mode === 'solo' }">
            <input v-model="settings.mode" type="radio" value="solo" />
            <b>個人賽</b><small>輪流描述，其他人全部一起畫、一起搶答</small>
          </label>
          <label class="choice" :class="{ on: settings.mode === 'team' }">
            <input v-model="settings.mode" type="radio" value="team" />
            <b>分組賽</b><small>各組輪流，組員畫、其他組看畫搶答</small>
          </label>
        </div>
      </fieldset>
      <div class="field-grid">
        <label class="field">
          <span>回合數 <small>（每回合換一人描述）</small></span>
          <input v-model.number="settings.rounds" type="number" min="1" max="100" required />
        </label>
        <label v-if="settings.mode === 'team'" class="field">
          <span>分幾組</span>
          <select v-model.number="settings.groups">
            <option v-for="n in 7" :key="n" :value="n + 1">{{ n + 1 }} 組</option>
          </select>
        </label>
      </div>
      <fieldset>
        <legend>每回合時間</legend>
        <div class="pill-row">
          <label v-for="o in secondOptions" :key="o.v" class="pill" :class="{ on: settings.seconds === o.v }">
            <input v-model="settings.seconds" type="radio" :value="o.v" />{{ o.label }}
          </label>
        </div>
      </fieldset>
      <fieldset>
        <legend>加入方式</legend>
        <div class="pill-row">
          <label class="pill" :class="{ on: settings.joinMode === 'name' }">
            <input v-model="settings.joinMode" type="radio" value="name" />輸入名字
          </label>
          <label class="pill" :class="{ on: settings.joinMode === 'auto' }">
            <input v-model="settings.joinMode" type="radio" value="auto" />隨機暱稱
          </label>
        </div>
      </fieldset>
      <button class="primary big" :disabled="game.busy || !game.ready">
        {{ game.ready ? "建立房間 →" : "連線中…" }}
      </button>
      <p class="muted small">建立房間的這台裝置就是「白板」：顯示 QR Code、所有人的畫和分數。</p>
    </form>
    <form class="join-code" @submit.prevent="join">
      <label for="code">已經有房號？</label>
      <input id="code" v-model="code" maxlength="6" placeholder="輸入 6 碼房號" autocapitalize="characters" />
      <button class="ghost">加入</button>
    </form>
  </main>
</template>

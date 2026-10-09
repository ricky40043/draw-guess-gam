let ctx: AudioContext | undefined;
// 答對時的「叮」聲；瀏覽器需使用者互動後才允許播放，失敗就安靜略過
export function ding(high = false) {
  try {
    ctx ??= new AudioContext();
    const t = ctx.currentTime;
    for (const [i, f] of (high ? [784, 1047, 1319] : [880, 1175]).entries()) {
      const o = ctx.createOscillator();
      const g = ctx.createGain();
      o.frequency.value = f;
      g.gain.setValueAtTime(0.0001, t + i * 0.09);
      g.gain.exponentialRampToValueAtTime(0.2, t + i * 0.09 + 0.02);
      g.gain.exponentialRampToValueAtTime(0.0001, t + i * 0.09 + 0.35);
      o.connect(g).connect(ctx.destination);
      o.start(t + i * 0.09);
      o.stop(t + i * 0.09 + 0.4);
    }
  } catch {
    /* 沒有音效也能玩 */
  }
}

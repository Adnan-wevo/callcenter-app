/**
 * A synthesized "ring ring" — two short tones, a pause, repeat — so the
 * incoming-call popup has an audio cue without shipping an audio asset.
 * Nothing in this repo or heal-crm's own Modules/SoftPhone carries a
 * ringtone file (confirmed by search), so this generates one with the Web
 * Audio API instead of inventing an asset dependency.
 *
 * Browsers refuse to run an `AudioContext` before a user gesture has
 * touched the page at least once (the autoplay policy) — `AppComponent`
 * primes a shared context on the first `pointerdown` for exactly this
 * reason (see its own comment); `start()` here reuses that context if one
 * was already primed, or creates its own otherwise (falling silent only if
 * the browser is still refusing playback, never throwing).
 */
export class Ringtone {
  private ctx: AudioContext | null = null;
  private timer: ReturnType<typeof setInterval> | null = null;

  start(): void {
    if (this.timer) {
      return; // already ringing
    }
    this.ctx ??= sharedAudioContext();
    this.playPulse();
    // A ring-ring-...pause... cadence: two ~200ms tones close together,
    // then quiet, repeating every 2s — the classic double-ring shape.
    this.timer = setInterval(() => this.playPulse(), 2000);
  }

  stop(): void {
    if (this.timer) {
      clearInterval(this.timer);
      this.timer = null;
    }
  }

  private playPulse(): void {
    const ctx = this.ctx;
    if (!ctx || ctx.state !== 'running') {
      return;
    }
    this.tone(ctx, ctx.currentTime, 0.22);
    this.tone(ctx, ctx.currentTime + 0.32, 0.22);
  }

  private tone(ctx: AudioContext, startAt: number, duration: number): void {
    const osc = ctx.createOscillator();
    const gain = ctx.createGain();
    osc.type = 'sine';
    osc.frequency.value = 440;
    // Ramp up/down rather than a hard on/off — avoids an audible click at
    // each tone's edge.
    gain.gain.setValueAtTime(0, startAt);
    gain.gain.linearRampToValueAtTime(0.15, startAt + 0.02);
    gain.gain.linearRampToValueAtTime(0, startAt + duration);
    osc.connect(gain).connect(ctx.destination);
    osc.start(startAt);
    osc.stop(startAt + duration);
  }
}

let shared: AudioContext | null = null;

/** The one `AudioContext` the app primes on first user gesture (see
 * AppComponent) and every `Ringtone` instance then reuses — creating a
 * fresh, still-suspended context per popup would just ring silently. */
export function sharedAudioContext(): AudioContext {
  shared ??= new AudioContext();
  return shared;
}

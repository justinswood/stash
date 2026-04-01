import videojs, { VideoJsPlayer } from "video.js";

interface SwipeSeekOptions {
  seekSeconds?: number;
  threshold?: number;
}

class SwipeSeekPlugin extends videojs.getPlugin("plugin") {
  private startX = 0;
  private startY = 0;
  private isSwiping = false;
  private seekSeconds: number;
  private threshold: number;
  private overlay: HTMLDivElement | null = null;

  constructor(player: VideoJsPlayer, options?: SwipeSeekOptions) {
    super(player);
    this.seekSeconds = options?.seekSeconds ?? 10;
    this.threshold = options?.threshold ?? 30;

    player.ready(() => {
      this.setupOverlay();
      this.setupTouchListeners();
    });
  }

  private setupOverlay() {
    this.overlay = document.createElement("div");
    this.overlay.className = "vjs-swipe-seek-overlay";
    this.overlay.style.cssText =
      "display:none;position:absolute;top:50%;left:50%;transform:translate(-50%,-50%);" +
      "background:rgba(0,0,0,0.7);color:#fff;padding:0.5rem 1rem;border-radius:20px;" +
      "font-size:1.25rem;font-weight:600;z-index:10;pointer-events:none;";
    this.player.el().appendChild(this.overlay);
  }

  private setupTouchListeners() {
    const el = this.player.el();

    el.addEventListener("touchstart", (e: Event) => {
      const touch = (e as TouchEvent).touches[0];
      this.startX = touch.clientX;
      this.startY = touch.clientY;
      this.isSwiping = false;
    }, { passive: true });

    el.addEventListener("touchmove", (e: Event) => {
      const touch = (e as TouchEvent).touches[0];
      const dx = touch.clientX - this.startX;
      const dy = touch.clientY - this.startY;

      // Only activate if predominantly horizontal
      if (Math.abs(dx) > this.threshold && Math.abs(dx) > Math.abs(dy) * 1.5) {
        this.isSwiping = true;
        const seconds = dx > 0 ? this.seekSeconds : -this.seekSeconds;
        this.showOverlay(seconds);
      }
    }, { passive: true });

    el.addEventListener("touchend", () => {
      if (this.isSwiping) {
        const currentTime = this.player.currentTime() || 0;
        const duration = this.player.duration() || 0;
        // Determine direction from last known position
        const overlay = this.overlay;
        if (overlay && overlay.dataset.seconds) {
          const seconds = parseFloat(overlay.dataset.seconds);
          const newTime = Math.max(0, Math.min(currentTime + seconds, duration));
          this.player.currentTime(newTime);
        }
        this.hideOverlay();
      }
      this.isSwiping = false;
    }, { passive: true });
  }

  private showOverlay(seconds: number) {
    if (!this.overlay) return;
    const sign = seconds > 0 ? "+" : "";
    this.overlay.textContent = `${sign}${seconds}s`;
    this.overlay.dataset.seconds = String(seconds);
    this.overlay.style.display = "block";
  }

  private hideOverlay() {
    if (!this.overlay) return;
    this.overlay.style.display = "none";
  }
}

videojs.registerPlugin("swipeSeek", SwipeSeekPlugin);

/* eslint-disable @typescript-eslint/naming-convention */
declare module "video.js" {
  interface VideoJsPlayer {
    swipeSeek: () => SwipeSeekPlugin;
  }
}

export default SwipeSeekPlugin;

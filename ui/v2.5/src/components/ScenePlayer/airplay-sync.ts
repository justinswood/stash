import videojs, { VideoJsPlayer } from "video.js";

const POLL_INTERVAL_MS = 250;

class AirPlaySyncPlugin extends videojs.getPlugin("plugin") {
  private intervalId: number | undefined;
  private lastCurrentTime = 0;
  private videoEl: HTMLVideoElement | undefined;
  private boundOnWirelessChange = () => this.onWirelessChange();

  constructor(player: VideoJsPlayer) {
    super(player);

    player.ready(() => {
      /* eslint-disable-next-line @typescript-eslint/no-explicit-any */
      const tech = (player as any).tech({ IWillNotUseThisInPlugins: true });
      const techEl = tech?.el?.();
      if (!(techEl instanceof HTMLVideoElement)) return;
      this.videoEl = techEl;

      if (!("webkitCurrentPlaybackTargetIsWireless" in this.videoEl)) return;

      this.videoEl.addEventListener(
        "webkitcurrentplaybacktargetiswirelesschanged",
        this.boundOnWirelessChange
      );

      player.on("seeked", () => {
        if (this.isWireless()) {
          this.videoEl?.play().catch(() => {
            /* receiver may already be playing; ignore */
          });
        }
      });
    });

    player.on("dispose", () => this.cleanup());
  }

  private isWireless(): boolean {
    /* eslint-disable-next-line @typescript-eslint/no-explicit-any */
    return !!(this.videoEl as any)?.webkitCurrentPlaybackTargetIsWireless;
  }

  private onWirelessChange() {
    if (this.isWireless()) this.startPolling();
    else this.stopPolling();
  }

  private startPolling() {
    if (this.intervalId !== undefined || !this.videoEl) return;
    this.lastCurrentTime = this.videoEl.currentTime;
    this.intervalId = window.setInterval(() => {
      if (!this.videoEl) return;
      const t = this.videoEl.currentTime;
      if (t !== this.lastCurrentTime) {
        this.lastCurrentTime = t;
        this.videoEl.dispatchEvent(new Event("timeupdate"));
      }
    }, POLL_INTERVAL_MS);
  }

  private stopPolling() {
    if (this.intervalId !== undefined) {
      window.clearInterval(this.intervalId);
      this.intervalId = undefined;
    }
  }

  private cleanup() {
    this.stopPolling();
    if (this.videoEl) {
      this.videoEl.removeEventListener(
        "webkitcurrentplaybacktargetiswirelesschanged",
        this.boundOnWirelessChange
      );
    }
  }
}

videojs.registerPlugin("airPlaySync", AirPlaySyncPlugin);

/* eslint-disable @typescript-eslint/naming-convention */
declare module "video.js" {
  interface VideoJsPlayer {
    airPlaySync: () => AirPlaySyncPlugin;
  }
  interface VideoJsPlayerPluginOptions {
    airPlaySync?: {};
  }
}

export default AirPlaySyncPlugin;

import videojs, { VideoJsPlayer } from "video.js";

class MediaSessionPlugin extends videojs.getPlugin("plugin") {
  constructor(player: VideoJsPlayer) {
    super(player);

    player.ready(() => {
      player.addClass("vjs-media-session");
      this.setActionHandlers();
    });

    player.on("play", () => {
      this.updatePlaybackState();
    });

    player.on("pause", () => {
      this.updatePlaybackState();
    });
    player.on("timeupdate", () => {
      this.updatePlaybackState();
    });
    this.updatePlaybackState();
  }

  // manually set poster since it's only set on useEffect
  public setMetadata(title: string, artist: string, poster: string): void {
    if ("mediaSession" in navigator) {
      navigator.mediaSession.metadata = new MediaMetadata({
        title,
        artist,
        artwork: [
          {
            src: poster || this.player.poster() || "",
            type: "image/jpeg",
          },
        ],
      });
    }
  }

  private updatePlaybackState(): void {
    if ("mediaSession" in navigator) {
      const playbackState = this.player.paused() ? "paused" : "playing";
      navigator.mediaSession.playbackState = playbackState;

      // Update position state for lock screen seek bar
      const duration = this.player.duration();
      if (duration && isFinite(duration) && "setPositionState" in navigator.mediaSession) {
        try {
          navigator.mediaSession.setPositionState({
            duration,
            playbackRate: this.player.playbackRate() || 1,
            position: Math.min(this.player.currentTime() || 0, duration),
          });
        } catch {
          // ignore errors from invalid state
        }
      }
    }
  }

  private setActionHandlers(): void {
    // method initialization
    navigator.mediaSession.setActionHandler("play", () => {
      this.player.play();
    });
    navigator.mediaSession.setActionHandler("pause", () => {
      this.player.pause();
    });
    navigator.mediaSession.setActionHandler("nexttrack", () => {
      this.player.skipButtons()?.handleForward();
    });
    navigator.mediaSession.setActionHandler("previoustrack", () => {
      this.player.skipButtons()?.handleBackward();
    });
    navigator.mediaSession.setActionHandler("seekto", (details) => {
      if (details.seekTime !== undefined) {
        this.player.currentTime(details.seekTime);
        this.updatePlaybackState();
      }
    });
  }
}

videojs.registerPlugin("mediaSession", MediaSessionPlugin);

/* eslint-disable @typescript-eslint/naming-convention */
declare module "video.js" {
  interface VideoJsPlayer {
    mediaSession: () => MediaSessionPlugin;
  }
}

export default MediaSessionPlugin;

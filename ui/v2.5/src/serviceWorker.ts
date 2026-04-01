import { Workbox } from "workbox-window";

let wb: Workbox | undefined;

export function register(onUpdate?: () => void) {
  if (import.meta.env.PROD && "serviceWorker" in navigator) {
    wb = new Workbox("/sw.js");

    wb.addEventListener("waiting", () => {
      if (onUpdate) onUpdate();
    });

    wb.register();
  }
}

export function skipWaiting() {
  wb?.messageSkipWaiting();
}

export function unregister() {
  if ("serviceWorker" in navigator) {
    navigator.serviceWorker.ready.then((reg) => reg.unregister());
  }
}

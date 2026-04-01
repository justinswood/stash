import React, { useEffect, useState } from "react";
import { Button } from "react-bootstrap";
import { Icon } from "src/components/Shared/Icon";
import { faTimes, faDownload } from "@fortawesome/free-solid-svg-icons";
import { isPlatformUniquelyRenderedByApple } from "src/utils/apple";
import localForage from "localforage";
import "./PWAInstallPrompt.scss";

const DISMISS_KEY = "pwa-install-prompt-dismissed";
const DISMISS_DAYS = 30;

interface BeforeInstallPromptEvent extends Event {
  prompt(): Promise<void>;
  userChoice: Promise<{ outcome: "accepted" | "dismissed" }>;
}

export const PWAInstallPrompt: React.FC = () => {
  const [deferredPrompt, setDeferredPrompt] =
    useState<BeforeInstallPromptEvent | null>(null);
  const [showBanner, setShowBanner] = useState(false);
  const [isApple] = useState(isPlatformUniquelyRenderedByApple);
  const [isInstalled, setIsInstalled] = useState(false);

  useEffect(() => {
    // Check if already running as installed PWA
    if (window.matchMedia("(display-mode: standalone)").matches) {
      setIsInstalled(true);
      return;
    }

    // Check if dismissed recently
    localForage.getItem<number>(DISMISS_KEY).then((dismissed) => {
      if (dismissed) {
        const daysSince = (Date.now() - dismissed) / (1000 * 60 * 60 * 24);
        if (daysSince < DISMISS_DAYS) return;
      }
      // Show banner for iOS (manual instructions) or wait for beforeinstallprompt
      if (isApple) {
        setShowBanner(true);
      }
    });

    const handler = (e: Event) => {
      e.preventDefault();
      setDeferredPrompt(e as BeforeInstallPromptEvent);
      setShowBanner(true);
    };

    window.addEventListener("beforeinstallprompt", handler);
    return () => window.removeEventListener("beforeinstallprompt", handler);
  }, [isApple]);

  const handleInstall = async () => {
    if (deferredPrompt) {
      await deferredPrompt.prompt();
      const { outcome } = await deferredPrompt.userChoice;
      if (outcome === "accepted") {
        setShowBanner(false);
      }
      setDeferredPrompt(null);
    }
  };

  const handleDismiss = () => {
    setShowBanner(false);
    localForage.setItem(DISMISS_KEY, Date.now());
  };

  if (isInstalled || !showBanner) return null;

  return (
    <div className="pwa-install-banner">
      <div className="pwa-install-banner-content">
        <Icon icon={faDownload} className="pwa-install-icon" />
        {isApple && !deferredPrompt ? (
          <span className="pwa-install-text">
            Install Stash: tap{" "}
            <strong>
              Share <span style={{ fontSize: "1.1em" }}>&#x2191;</span>
            </strong>{" "}
            then <strong>Add to Home Screen</strong>
          </span>
        ) : (
          <span className="pwa-install-text">
            Install Stash for a better experience
          </span>
        )}
      </div>
      <div className="pwa-install-banner-actions">
        {deferredPrompt && (
          <Button
            variant="primary"
            size="sm"
            onClick={handleInstall}
            className="pwa-install-btn"
          >
            Install
          </Button>
        )}
        <button className="pwa-install-dismiss" onClick={handleDismiss}>
          <Icon icon={faTimes} />
        </button>
      </div>
    </div>
  );
};

import React, { useEffect, useState } from "react";
import { Button } from "react-bootstrap";
import { skipWaiting } from "src/serviceWorker";
import "./PWAUpdatePrompt.scss";

export const PWAUpdatePrompt: React.FC = () => {
  const [showUpdate, setShowUpdate] = useState(false);

  useEffect(() => {
    const handler = () => setShowUpdate(true);
    window.addEventListener("sw-update-available", handler);
    return () => window.removeEventListener("sw-update-available", handler);
  }, []);

  const handleUpdate = () => {
    skipWaiting();
    window.location.reload();
  };

  if (!showUpdate) return null;

  return (
    <div className="pwa-update-toast">
      <span>A new version of Stash is available</span>
      <Button variant="primary" size="sm" onClick={handleUpdate}>
        Update
      </Button>
    </div>
  );
};

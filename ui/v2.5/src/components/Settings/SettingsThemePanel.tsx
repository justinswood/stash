import React from "react";

// The Refract theme's settings UI (accent swatches, card rating style, scene
// card style, lite mode, custom logo, view minimiser, …) is rendered into the
// mount element below by the bundled theme script (public/theme/refract.js),
// which reuses the exact same controls that previously lived in the plugin's
// settings card. This panel is just the host.
export const SettingsThemePanel: React.FC = () => {
  return (
    <>
      <h4>Theme</h4>
      <p className="text-muted">
        Customise the look of Stash — accent colour, card style, lite mode and
        more. Settings are saved per browser and apply instantly.
      </p>
      <div id="refract-theme-settings-root" />
    </>
  );
};

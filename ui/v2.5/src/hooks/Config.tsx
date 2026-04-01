import React, { useMemo } from "react";
import * as GQL from "src/core/generated-graphql";

export interface IContext {
  configuration: GQL.ConfigDataFragment;
}

// Full configuration context - use sparingly, prefer specific contexts below
export const ConfigurationContext = React.createContext<IContext | null>(null);

// Stable context for general/interface settings that rarely change
const GeneralConfigContext = React.createContext<
  GQL.ConfigDataFragment["general"] | null
>(null);

// UI config context for frequently-changing display preferences
const UIConfigContext = React.createContext<
  GQL.ConfigDataFragment["ui"] | null
>(null);

export const useConfigurationContext = () => {
  const context = React.useContext(ConfigurationContext);

  if (context === null) {
    throw new Error(
      "useConfigurationContext must be used within a ConfigurationProvider"
    );
  }

  return context;
};

export const useConfigurationContextOptional = () => {
  return React.useContext(ConfigurationContext);
};

// Use for general settings (stashes, auth, etc.) - stable, rarely re-renders
export const useGeneralConfig = () => {
  return React.useContext(GeneralConfigContext);
};

// Use for UI display settings - may change more frequently
export const useUIConfig = () => {
  return React.useContext(UIConfigContext);
};

export const ConfigurationProvider: React.FC<IContext> = ({
  configuration,
  children,
}) => {
  // Memoize sub-contexts to prevent re-renders when unrelated config changes
  const general = useMemo(
    () => configuration.general,
    [configuration.general]
  );
  const ui = useMemo(() => configuration.ui, [configuration.ui]);

  return (
    <ConfigurationContext.Provider
      value={{
        configuration,
      }}
    >
      <GeneralConfigContext.Provider value={general}>
        <UIConfigContext.Provider value={ui}>
          {children}
        </UIConfigContext.Provider>
      </GeneralConfigContext.Provider>
    </ConfigurationContext.Provider>
  );
};

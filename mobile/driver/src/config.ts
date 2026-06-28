import Constants from "expo-constants";

// In Expo dev the Metro bundler's hostUri is "<lan-ip>:<port>" — the same
// machine (and LAN) running the backend — so we target it automatically. For a
// standalone build, set `extra.backendHost` in app.json (or your own IP here).
function detectHost(): string {
  const extra = (Constants.expoConfig?.extra ?? {}) as { backendHost?: string };
  if (extra.backendHost) return extra.backendHost;
  const hostUri = Constants.expoConfig?.hostUri;
  if (hostUri) return hostUri.split(":")[0];
  return "localhost";
}

const HOST = detectHost();
export const API_BASE = `http://${HOST}:8080`;
export const WS_BASE = `ws://${HOST}:8090`;

// Default map view: central Lagos.
export const DEFAULT_REGION = {
  latitude: 6.5244,
  longitude: 3.3792,
  latitudeDelta: 0.03,
  longitudeDelta: 0.03,
};

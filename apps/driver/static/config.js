// Resolves backend URLs from the current host, with optional query overrides:
//   ?host=1.2.3.4&apiPort=8080&wsPort=8090
const params = new URLSearchParams(location.search);
const host = params.get("host") || location.hostname || "localhost";
const apiPort = params.get("apiPort") || "8080";
const wsPort = params.get("wsPort") || "8090";

export const API = `http://${host}:${apiPort}`;
export const WS = `ws://${host}:${wsPort}`;

// Default map centre: central Lagos.
export const DEFAULT_CENTER = [6.5244, 3.3792];

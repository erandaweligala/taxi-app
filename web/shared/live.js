// connectLive opens the gateway WebSocket for a user and invokes onMessage with
// each parsed notification. It auto-reconnects with a short backoff so the demo
// survives the gateway restarting. Returns a handle with a close() method.
import { WS } from "./config.js";

export function connectLive(userId, { onMessage, onStatus } = {}) {
  let ws;
  let closed = false;
  let backoff = 1000;

  function open() {
    ws = new WebSocket(`${WS}/ws?user_id=${encodeURIComponent(userId)}`);
    ws.onopen = () => {
      backoff = 1000;
      onStatus && onStatus("connected");
    };
    ws.onmessage = (ev) => {
      try {
        onMessage && onMessage(JSON.parse(ev.data));
      } catch {
        /* ignore non-JSON frames */
      }
    };
    ws.onclose = () => {
      onStatus && onStatus("disconnected");
      if (!closed) setTimeout(open, (backoff = Math.min(backoff * 2, 8000)));
    };
    ws.onerror = () => ws.close();
  }

  open();
  return {
    close() {
      closed = true;
      ws && ws.close();
    },
  };
}

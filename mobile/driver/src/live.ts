import { useEffect, useRef, useState } from "react";
import { WS_BASE } from "./config";

export type LiveNotification = {
  user_id: string;
  kind: string;
  trip_id?: string;
  message: string;
  at: number;
};

export type LiveStatus = "connected" | "disconnected";

// useLive maintains a WebSocket to the gateway for a user, reconnecting with
// backoff, and invokes onMessage for each notification. The same per-user
// pub/sub channel drives both apps.
export function useLive(
  userId: string,
  onMessage: (n: LiveNotification) => void,
): LiveStatus {
  const [status, setStatus] = useState<LiveStatus>("disconnected");
  const cb = useRef(onMessage);
  cb.current = onMessage;

  useEffect(() => {
    let ws: WebSocket | null = null;
    let closed = false;
    let backoff = 1000;
    let timer: ReturnType<typeof setTimeout>;

    const open = () => {
      ws = new WebSocket(`${WS_BASE}/ws?user_id=${encodeURIComponent(userId)}`);
      ws.onopen = () => {
        backoff = 1000;
        setStatus("connected");
      };
      ws.onmessage = (e) => {
        try {
          cb.current(JSON.parse(e.data));
        } catch {
          /* ignore non-JSON frames */
        }
      };
      ws.onclose = () => {
        setStatus("disconnected");
        if (!closed) timer = setTimeout(open, (backoff = Math.min(backoff * 2, 8000)));
      };
      ws.onerror = () => ws?.close();
    };

    open();
    return () => {
      closed = true;
      clearTimeout(timer);
      ws?.close();
    };
  }, [userId]);

  return status;
}

import { useEffect, useRef, useState } from 'react';
import type { FlowEvent } from '../api/types';

const MAX_EVENTS = 100;
const MIN_BACKOFF = 1000;
const MAX_BACKOFF = 5000;

interface UseFlowEventsResult {
  events: FlowEvent[];
  connected: boolean;
}

export function useFlowEvents(flowId: string, enabled: boolean): UseFlowEventsResult {
  const [events, setEvents] = useState<FlowEvent[]>([]);
  const [connected, setConnected] = useState(false);
  const unmounted = useRef(false);
  const backoff = useRef(MIN_BACKOFF);

  useEffect(() => {
    if (!enabled) return;
    unmounted.current = false;

    let ws: WebSocket | null = null;
    let timeoutId: ReturnType<typeof setTimeout> | null = null;

    const connect = () => {
      if (unmounted.current) return;
      const proto = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
      const url = `${proto}//${window.location.host}/api/flows/${flowId}/events`;
      ws = new WebSocket(url);

      ws.onopen = () => {
        if (!unmounted.current) {
          setConnected(true);
          backoff.current = MIN_BACKOFF;
        }
      };

      ws.onmessage = (e: MessageEvent) => {
        if (unmounted.current) return;
        try {
          const ev = JSON.parse(e.data as string) as FlowEvent;
          setEvents((prev) => {
            const next = [ev, ...prev];
            return next.length > MAX_EVENTS ? next.slice(0, MAX_EVENTS) : next;
          });
        } catch {
          // ignore malformed messages
        }
      };

      ws.onclose = () => {
        if (unmounted.current) return;
        setConnected(false);
        const delay = backoff.current;
        backoff.current = Math.min(backoff.current * 2, MAX_BACKOFF);
        timeoutId = setTimeout(connect, delay);
      };

      ws.onerror = () => {
        ws?.close();
      };
    };

    connect();

    return () => {
      unmounted.current = true;
      if (timeoutId !== null) clearTimeout(timeoutId);
      ws?.close();
    };
  }, [flowId, enabled]);

  return { events, connected };
}

import { useEffect, useState } from 'react';

export type SseEvent = { type: string; data: unknown };

export function useSse(url: string, max = 200): SseEvent[] {
  const [events, setEvents] = useState<SseEvent[]>([]);

  useEffect(() => {
    const es = new EventSource(url);

    const push = (type: string, raw: string) => {
      try {
        const data = JSON.parse(raw);
        setEvents((prev) => {
          const next = [...prev, { type, data }];
          return next.length > max ? next.slice(next.length - max) : next;
        });
      } catch {
        // ignore malformed frames
      }
    };

    const onTelemetry = (ev: MessageEvent) => push('telemetry', ev.data);
    const onFired = (ev: MessageEvent) => push('fired', ev.data);
    const onRules = (ev: MessageEvent) => push('rules_changed', ev.data);

    es.addEventListener('telemetry', onTelemetry);
    es.addEventListener('fired', onFired);
    es.addEventListener('rules_changed', onRules);

    return () => {
      es.removeEventListener('telemetry', onTelemetry);
      es.removeEventListener('fired', onFired);
      es.removeEventListener('rules_changed', onRules);
      es.close();
    };
  }, [url, max]);

  return events;
}

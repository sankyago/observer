import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { renderHook, act } from '@testing-library/react';
import { useSse } from './useSse';

type Listener = (ev: MessageEvent) => void;

class FakeEventSource {
  static last: FakeEventSource | null = null;
  listeners = new Map<string, Listener>();
  url: string;
  constructor(url: string) {
    this.url = url;
    FakeEventSource.last = this;
  }
  addEventListener(type: string, fn: Listener) { this.listeners.set(type, fn); }
  removeEventListener(type: string, fn: Listener) {
    if (this.listeners.get(type) === fn) this.listeners.delete(type);
  }
  close() {}
  fire(type: string, data: string) {
    this.listeners.get(type)?.({ data } as MessageEvent);
  }
}

describe('useSse', () => {
  beforeEach(() => {
    vi.stubGlobal('EventSource', FakeEventSource as unknown as typeof EventSource);
  });
  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it('parses telemetry frames', () => {
    const { result } = renderHook(() => useSse('/api/v1/stream'));
    act(() => {
      FakeEventSource.last!.fire('telemetry', JSON.stringify({ x: 1 }));
    });
    expect(result.current).toEqual([{ type: 'telemetry', data: { x: 1 } }]);
  });
});

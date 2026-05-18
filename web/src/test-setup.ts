import "@testing-library/jest-dom";

// Mock EventSource for jsdom (not available natively)
class MockEventSource {
  url: string;
  onopen: (() => void) | null = null;
  onerror: ((e: Event) => void) | null = null;
  onmessage: ((e: MessageEvent) => void) | null = null;
  eventListeners: Map<string, EventListenerOrEventListenerObject> = new Map();
  static readonly CONNECTING = 0;
  static readonly OPEN = 1;
  static readonly CLOSED = 2;
  readyState: number = MockEventSource.CONNECTING;

  constructor(url: string) {
    this.url = url;
  }

  addEventListener(type: string, listener: EventListenerOrEventListenerObject) {
    this.eventListeners.set(type, listener);
  }

  close() {
    this.readyState = MockEventSource.CLOSED;
  }

  dispatchEvent(event: Event): boolean {
    return true;
  }
}

// @ts-expect-error - Mock EventSource for test environment
globalThis.EventSource = MockEventSource;

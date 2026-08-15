export function isTrustedCanvasMessage(event: Pick<MessageEvent, 'origin' | 'source'>, frameWindow: Window | null): boolean {
  return Boolean(frameWindow && event.origin === window.location.origin && event.source === frameWindow)
}

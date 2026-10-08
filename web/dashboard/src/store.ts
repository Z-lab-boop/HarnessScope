import type { UIState } from "./types.js";
export type Listener = (state: Readonly<UIState>) => void;
export class Store {
  private listeners = new Set<Listener>();
  constructor(private value: UIState) {}
  get(): Readonly<UIState> { return this.value; }
  update(change: Partial<UIState>): void {
    this.value = { ...this.value, ...change };
    for (const listener of this.listeners) listener(this.value);
  }
  subscribe(listener: Listener): () => void {
    this.listeners.add(listener);
    return () => this.listeners.delete(listener);
  }
}

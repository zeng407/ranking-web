// An in-memory Web Storage for the test run.
//
// WHY THIS FILE EXISTS. The suite runs with environment: 'node' and used to reach the
// global localStorage that Node itself provides. That global is experimental and needs
// --localstorage-file to point somewhere real; on Node 25 the object is present but
// every method on it is undefined when the flag has no valid path, so `localStorage`
// is truthy, feature detection passes, and the first call fails with
// "localStorage.clear is not a function". Reproduced outside vitest with a bare
// `node -e`, so it is the runtime, not the test runner.
//
// Rather than pass a flag and write a file, the tests get their own implementation.
// The suite is about application behaviour and has no business depending on which Node
// experiments happen to be enabled — and an on-disk store shared between runs would
// leak state from one test file into the next.

class MemoryStorage implements Storage {
  private entries = new Map<string, string>()

  get length(): number {
    return this.entries.size
  }

  key(index: number): string | null {
    return [...this.entries.keys()][index] ?? null
  }

  getItem(key: string): string | null {
    // null rather than undefined for a missing key: code written against the real API
    // checks for null, and undefined would slip past that.
    return this.entries.has(key) ? (this.entries.get(key) as string) : null
  }

  setItem(key: string, value: string): void {
    // Coerced, because the real Storage stores strings and callers pass numbers.
    this.entries.set(String(key), String(value))
  }

  removeItem(key: string): void {
    this.entries.delete(String(key))
  }

  clear(): void {
    this.entries.clear()
  }
}

// defineProperty rather than assignment: Node's own localStorage is a getter on the
// global, and `globalThis.localStorage = ...` does not replace it.
for (const name of ['localStorage', 'sessionStorage'] as const) {
  Object.defineProperty(globalThis, name, {
    configurable: true,
    writable: true,
    value: new MemoryStorage(),
  })
}

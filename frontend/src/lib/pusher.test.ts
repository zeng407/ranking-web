// @vitest-environment happy-dom

import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { pusherURL, subscribe, type PusherOptions, type PusherState } from './pusher'

const options: PusherOptions = { key: 'app-key', host: 'soketi.test', port: 6001, secure: false }

/** A WebSocket stand-in that lets a test drive both directions of the protocol. */
class FakeSocket {
  static instances: FakeSocket[] = []

  readyState = 1 // OPEN, so send() is exercised.
  sent: string[] = []
  closed = false
  onopen: (() => void) | null = null
  onmessage: ((event: MessageEvent) => void) | null = null
  onclose: (() => void) | null = null
  onerror: (() => void) | null = null

  constructor(readonly url: string) {
    FakeSocket.instances.push(this)
  }

  send(data: string): void {
    this.sent.push(data)
  }

  close(): void {
    this.closed = true
  }

  /** Simulates a server frame. */
  deliver(frame: unknown): void {
    this.onmessage?.({ data: JSON.stringify(frame) } as MessageEvent)
  }

  deliverRaw(data: string): void {
    this.onmessage?.({ data } as MessageEvent)
  }

  established(activityTimeoutSeconds = 120): void {
    this.onopen?.()
    // The real server sends data as a JSON STRING here.
    this.deliver({
      event: 'pusher:connection_established',
      data: JSON.stringify({ socket_id: '1.1', activity_timeout: activityTimeoutSeconds }),
    })
  }

  get frames(): Array<{ event: string; data: unknown }> {
    return this.sent.map((raw) => JSON.parse(raw))
  }
}

function factory(url: string): WebSocket {
  return new FakeSocket(url) as unknown as WebSocket
}

/**
 * The nth socket the client opened.
 *
 * A helper rather than an index expression at every call site: the project compiles with
 * noUncheckedIndexedAccess, and a missing socket is a test failure worth naming rather than
 * a non-null assertion repeated thirty times.
 */
function socketAt(index: number): FakeSocket {
  const socket = FakeSocket.instances[index]
  if (!socket) throw new Error(`expected a socket at index ${index}, saw ${FakeSocket.instances.length}`)
  return socket
}

describe('pusher', () => {
  beforeEach(() => {
    FakeSocket.instances = []
    vi.useFakeTimers()
  })

  afterEach(() => {
    vi.useRealTimers()
  })

  it('builds a protocol 7 url', () => {
    expect(pusherURL(options)).toBe(
      'ws://soketi.test:6001/app/app-key?protocol=7&client=js&version=8.0.0')
    expect(pusherURL({ ...options, secure: true })).toMatch(/^wss:/)
    expect(pusherURL({ ...options, cluster: 'eu' })).toContain('ws-eu.pusher.com')
  })

  /**
   * THE FIRST PROTOCOL TRAP. An outbound frame's `data` must be an OBJECT. Sending it as a
   * JSON string makes Soketi read `.channel` off a primitive, and the uncaught TypeError
   * takes the whole node process down — which surfaces as "connection refused" and reads
   * like a firewall problem.
   */
  it('subscribes with data as an object, not a string', () => {
    subscribe(options, 'game-room.abc', {}, undefined, factory)
    const socket = socketAt(0)
    socket.established()

    const subscribeFrame = socket.frames.find((frame) => frame.event === 'pusher:subscribe')
    expect(subscribeFrame).toBeDefined()
    expect(typeof subscribeFrame?.data).toBe('object')
    expect(subscribeFrame?.data).toEqual({ channel: 'game-room.abc' })
  })

  /**
   * THE SECOND TRAP. An inbound frame's `data` is a JSON STRING. Handing it to a handler
   * unparsed yields undefined for every field, silently.
   */
  it('parses the JSON string an event carries', () => {
    const handler = vi.fn()
    subscribe(options, 'game-room.abc', { GameBetRank: handler }, undefined, factory)
    const socket = socketAt(0)
    socket.established()

    socket.deliver({
      event: 'GameBetRank',
      channel: 'game-room.abc',
      data: JSON.stringify({ total_users: 3, top_10: [], bottom_10: [] }),
    })

    expect(handler).toHaveBeenCalledWith({ total_users: 3, top_10: [], bottom_10: [] })
  })

  it('reports connecting then connected', () => {
    const states: PusherState[] = []
    subscribe(options, 'game-room.abc', {}, (state) => states.push(state), factory)
    socketAt(0).established()

    expect(states).toEqual(['connecting', 'connected'])
  })

  it('ignores an event for another channel', () => {
    const handler = vi.fn()
    subscribe(options, 'game-room.abc', { GameBetRank: handler }, undefined, factory)
    const socket = socketAt(0)
    socket.established()

    socket.deliver({ event: 'GameBetRank', channel: 'game-room.other', data: '{}' })
    expect(handler).not.toHaveBeenCalled()
  })

  it('survives a frame that is not JSON', () => {
    const handler = vi.fn()
    subscribe(options, 'game-room.abc', { GameBetRank: handler }, undefined, factory)
    const socket = socketAt(0)
    socket.established()

    // Must not throw, and must not tear the connection down.
    expect(() => socket.deliverRaw('not json at all')).not.toThrow()
    expect(socket.closed).toBe(false)
  })

  /**
   * THE ACTIVITY TIMEOUT, which is the quiet killer. The protocol requires the CLIENT to
   * ping when the connection has been idle; without it the server drops the socket and the
   * page simply stops updating, with no error anywhere.
   */
  it('pings when the connection goes idle', () => {
    subscribe(options, 'game-room.abc', {}, undefined, factory)
    const socket = socketAt(0)
    socket.established(30)

    expect(socket.frames.some((frame) => frame.event === 'pusher:ping')).toBe(false)
    vi.advanceTimersByTime(30_000)
    expect(socket.frames.some((frame) => frame.event === 'pusher:ping')).toBe(true)
  })

  // A ping with no pong means the socket is open but the peer is gone — a state no close
  // event ever reports.
  it('reconnects when a ping goes unanswered', () => {
    subscribe(options, 'game-room.abc', {}, undefined, factory)
    socketAt(0).established(30)

    vi.advanceTimersByTime(30_000) // ping
    vi.advanceTimersByTime(30_000) // pong timeout
    vi.advanceTimersByTime(1_000) // first reconnect delay

    expect(FakeSocket.instances.length).toBe(2)
  })

  it('does not reconnect when a pong arrives', () => {
    subscribe(options, 'game-room.abc', {}, undefined, factory)
    const socket = socketAt(0)
    socket.established(30)

    vi.advanceTimersByTime(30_000)
    socket.deliver({ event: 'pusher:pong', data: '{}' })
    vi.advanceTimersByTime(60_000)

    // One extra socket would mean a needless reconnect; more would mean a loop.
    expect(FakeSocket.instances.length).toBe(1)
  })

  it('reconnects with a growing delay after a close', () => {
    subscribe(options, 'game-room.abc', {}, undefined, factory)
    socketAt(0).established()

    socketAt(0).onclose?.()
    // Nothing immediately: a tight retry loop from every open tab turns a restart into an
    // outage.
    expect(FakeSocket.instances.length).toBe(1)
    vi.advanceTimersByTime(1_000)
    expect(FakeSocket.instances.length).toBe(2)

    socketAt(1).onclose?.()
    vi.advanceTimersByTime(1_000)
    expect(FakeSocket.instances.length).toBe(2)
    vi.advanceTimersByTime(1_000)
    expect(FakeSocket.instances.length).toBe(3)
  })

  it('resets the backoff once a connection is established', () => {
    subscribe(options, 'game-room.abc', {}, undefined, factory)
    socketAt(0).established()
    socketAt(0).onclose?.()
    vi.advanceTimersByTime(1_000)

    // Second socket connects successfully, so the next failure starts from the first delay
    // again rather than continuing to grow.
    socketAt(1).established()
    socketAt(1).onclose?.()
    vi.advanceTimersByTime(1_000)

    expect(FakeSocket.instances.length).toBe(3)
  })

  it('stops everything on leave', () => {
    const states: PusherState[] = []
    const channel = subscribe(options, 'game-room.abc', {}, (state) => states.push(state), factory)
    const socket = socketAt(0)
    socket.established(30)

    channel.leave()

    expect(socket.closed).toBe(true)
    expect(states.at(-1)).toBe('disconnected')

    // No ping, no reconnect, no timer left firing against a room nobody is looking at.
    vi.advanceTimersByTime(120_000)
    expect(FakeSocket.instances.length).toBe(1)
    expect(socket.frames.some((frame) => frame.event === 'pusher:ping')).toBe(false)
  })

  it('does not reconnect after leave, even if the socket closes later', () => {
    const channel = subscribe(options, 'game-room.abc', {}, undefined, factory)
    const socket = socketAt(0)
    socket.established()
    channel.leave()

    // The handler was detached, so this is a no-op; calling it proves the detach happened.
    socket.onclose?.()
    vi.advanceTimersByTime(60_000)
    expect(FakeSocket.instances.length).toBe(1)
  })
})

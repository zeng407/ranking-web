/**
 * The slice of the Pusher protocol this app needs: subscribe to public channels and
 * receive events.
 *
 * WHY NOT pusher-js. The SPA has two runtime dependencies (vue, vue-router) and this
 * would be the third, for a protocol whose public-channel half is the forty lines below.
 * pusher-js also carries private/presence channel auth, a state machine and a plugin
 * surface, none of which is used here. The parts that are genuinely easy to get wrong —
 * the activity timeout and reconnection — are implemented explicitly below, and the room
 * view keeps a slow poll running regardless, so a silently dead socket degrades to a
 * stale-but-moving leaderboard rather than a frozen one.
 *
 * TWO PROTOCOL TRAPS, both found the hard way while building the Go test harness:
 *
 *  1. On an OUTBOUND frame, `data` must be an OBJECT. Sending it as a JSON string makes
 *     Soketi read `.channel` off a primitive, and the uncaught TypeError kills the node
 *     process — the symptom is "connection refused", which reads like a firewall problem.
 *  2. On an INBOUND frame, `data` is a JSON STRING that has to be parsed. Treating it as
 *     an object silently yields undefined for every field.
 */

export interface PusherOptions {
  key: string
  /** Host only, no scheme: "soketi" or "ws.example.com". */
  host: string
  port: number
  /** true selects wss. */
  secure: boolean
  /** Optional cluster, for the hosted service rather than Soketi. */
  cluster?: string
}

export type PusherState = 'connecting' | 'connected' | 'disconnected'

export interface PusherChannel {
  /** Stops listening and closes the socket when nothing else needs it. */
  leave(): void
}

type EventHandler = (payload: unknown) => void

interface InboundFrame {
  event: string
  channel?: string
  /** A JSON string for application events; an object for some pusher: events. */
  data?: string | Record<string, unknown>
}

/**
 * Reconnect delays in milliseconds, then the last value repeats.
 *
 * Starts at one second rather than immediately: a server that just dropped the connection
 * is often restarting, and a tight retry loop from every open tab is how a restart turns
 * into an outage.
 */
const RECONNECT_DELAYS_MS = [1_000, 2_000, 5_000, 10_000, 30_000]

/** Fallback when the server does not send activity_timeout. Pusher's documented default. */
const DEFAULT_ACTIVITY_TIMEOUT_MS = 120_000

/** How long to wait for a pong before treating the connection as dead. */
const PONG_TIMEOUT_MS = 30_000

export function pusherURL(options: PusherOptions): string {
  const scheme = options.secure ? 'wss' : 'ws'
  const host = options.cluster ? `ws-${options.cluster}.pusher.com` : options.host
  const query = new URLSearchParams({ protocol: '7', client: 'js', version: '8.0.0' })
  return `${scheme}://${host}:${options.port}/app/${encodeURIComponent(options.key)}?${query}`
}

/**
 * subscribe joins one public channel and calls handlers as events arrive.
 *
 * One socket per subscribe call. That is wasteful in the abstract and correct here: this
 * app subscribes to exactly one room at a time, and sharing a connection would mean
 * tracking reference counts to know when to close it.
 */
export function subscribe(
  options: PusherOptions,
  channel: string,
  handlers: Record<string, EventHandler>,
  onState?: (state: PusherState) => void,
  socketFactory: (url: string) => WebSocket = (url) => new WebSocket(url),
): PusherChannel {
  let socket: WebSocket | null = null
  let closed = false
  let attempt = 0
  let reconnectTimer: ReturnType<typeof setTimeout> | undefined
  let activityTimer: ReturnType<typeof setTimeout> | undefined
  let pongTimer: ReturnType<typeof setTimeout> | undefined
  let activityTimeoutMs = DEFAULT_ACTIVITY_TIMEOUT_MS

  function clearTimers(): void {
    if (activityTimer) clearTimeout(activityTimer)
    if (pongTimer) clearTimeout(pongTimer)
    activityTimer = undefined
    pongTimer = undefined
  }

  /**
   * Restarts the idle countdown. The protocol requires the CLIENT to ping when the
   * connection has been quiet for activity_timeout; without it the server drops the
   * socket and the page stops updating with no error anywhere.
   */
  function noteActivity(): void {
    clearTimers()
    if (closed) return
    activityTimer = setTimeout(() => {
      send({ event: 'pusher:ping', data: {} })
      // A ping with no pong means the socket is open but the peer is gone — which a
      // close event would never tell us about.
      pongTimer = setTimeout(() => reconnect(), PONG_TIMEOUT_MS)
    }, activityTimeoutMs)
  }

  function send(frame: { event: string; data: Record<string, unknown> }): void {
    // data as an OBJECT, never a string. See the trap in this file's header.
    if (socket?.readyState === WebSocket.OPEN) socket.send(JSON.stringify(frame))
  }

  function reconnect(): void {
    if (closed) return
    clearTimers()
    // Detached before closing so the handler does not schedule a second reconnect.
    const dying = socket
    socket = null
    if (dying) {
      dying.onclose = null
      dying.onerror = null
      dying.onmessage = null
      try {
        dying.close()
      } catch {
        // Already closing; nothing to do.
      }
    }
    onState?.('connecting')
    const delay = RECONNECT_DELAYS_MS[Math.min(attempt, RECONNECT_DELAYS_MS.length - 1)]
    attempt += 1
    reconnectTimer = setTimeout(open, delay)
  }

  function open(): void {
    if (closed) return
    onState?.('connecting')
    try {
      socket = socketFactory(pusherURL(options))
    } catch {
      reconnect()
      return
    }

    socket.onopen = () => noteActivity()

    socket.onmessage = (message: MessageEvent) => {
      noteActivity()
      let frame: InboundFrame
      try {
        frame = JSON.parse(String(message.data)) as InboundFrame
      } catch {
        // A frame that does not parse is not worth tearing the connection down for.
        return
      }

      switch (frame.event) {
        case 'pusher:connection_established': {
          attempt = 0
          const established = parseFrameData(frame.data) as { activity_timeout?: number } | null
          if (established?.activity_timeout) {
            // The server's value is in SECONDS.
            activityTimeoutMs = established.activity_timeout * 1_000
          }
          onState?.('connected')
          send({ event: 'pusher:subscribe', data: { channel } })
          noteActivity()
          return
        }
        case 'pusher:pong':
          if (pongTimer) clearTimeout(pongTimer)
          pongTimer = undefined
          return
        case 'pusher:error':
          // An error frame is terminal for this socket; the server closes it next.
          return
        default:
          break
      }

      // Only frames for the channel this call joined, in case a shared socket ever
      // arrives.
      if (frame.channel && frame.channel !== channel) return

      const handler = handlers[frame.event]
      if (handler) handler(parseFrameData(frame.data))
    }

    socket.onclose = () => {
      if (closed) return
      onState?.('disconnected')
      reconnect()
    }

    socket.onerror = () => {
      // onclose follows, which is where the reconnect happens; doing it here too would
      // schedule two.
    }
  }

  open()

  return {
    leave(): void {
      closed = true
      clearTimers()
      if (reconnectTimer) clearTimeout(reconnectTimer)
      const dying = socket
      socket = null
      if (dying) {
        dying.onclose = null
        dying.onmessage = null
        dying.onerror = null
        try {
          dying.close()
        } catch {
          // Already closing.
        }
      }
      onState?.('disconnected')
    },
  }
}

/**
 * Parses an inbound frame's data.
 *
 * The protocol sends a JSON STRING for application events. Some pusher: frames send an
 * object instead, so both are accepted rather than assuming one.
 */
function parseFrameData(data: string | Record<string, unknown> | undefined): unknown {
  if (data === undefined || data === null) return null
  if (typeof data !== 'string') return data
  try {
    return JSON.parse(data)
  } catch {
    // Not JSON: hand back the raw string rather than losing it.
    return data
  }
}

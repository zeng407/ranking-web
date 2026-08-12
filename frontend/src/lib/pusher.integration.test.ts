// @vitest-environment node

import { describe, expect, it } from 'vitest'

import { subscribe, type PusherOptions } from './pusher'

/**
 * Runs this client against a real Soketi.
 *
 * Gated on SOKETI_WS_HOST, the same convention the Go integration tests use for
 * MYSQL_TEST_HOST: a hermetic run skips, and a run against the local stack exercises the
 * protocol against the server that actually implements it. The unit tests use a fake socket
 * and therefore only prove that this client is self-consistent — which is exactly the trap
 * that let a 204 handling bug ship.
 *
 * Node has had a global WebSocket since 22, so the client's default factory works here with
 * no polyfill.
 */

const host = process.env.SOKETI_WS_HOST
const key = process.env.SOKETI_APP_KEY

const options: PusherOptions = {
  key: key || '',
  host: host || '',
  port: Number.parseInt(process.env.SOKETI_WS_PORT || '6001', 10),
  secure: false,
}

describe.skipIf(!host || !key)('pusher against a real Soketi', () => {
  it('connects, subscribes, and stays subscribed', async () => {
    const states: string[] = []
    const channel = subscribe(options, 'game-room.integration-probe', {}, (state) => {
      states.push(state)
    })

    try {
      await waitFor(() => states.includes('connected'), 10_000)
      expect(states).toContain('connected')

      // Still connected a moment later: a rejected subscribe would close the socket and
      // push 'disconnected' onto the list.
      await sleep(1_000)
      expect(states.at(-1)).toBe('connected')
    } finally {
      channel.leave()
    }
  }, 20_000)

  /**
   * Receives an event published through the server's own HTTP API, decoding it the way an
   * application event actually arrives: `data` as a JSON STRING.
   */
  it('parses an event the server really sent', async () => {
    const received: unknown[] = []
    const channel = subscribe(options, 'game-room.integration-probe', {
      GameBetRank: (payload) => received.push(payload),
    })

    try {
      await sleep(1_500) // let the subscribe land
      await publish('game-room.integration-probe', 'GameBetRank', {
        total_users: 2,
        top_10: [{ user_id: 'a', name: 'probe', score: 1010, rank: 1, accuracy: '50.00', total_played: 2, total_correct: 1, combo: 0 }],
        bottom_10: [],
      })

      await waitFor(() => received.length > 0, 10_000)
      const board = received[0] as { total_users: number; top_10: Array<{ score: number }> }
      expect(board.total_users).toBe(2)
      // Parsed, not a string: reading .score off a string would be undefined.
      expect(board.top_10[0]?.score).toBe(1010)
    } finally {
      channel.leave()
    }
  }, 25_000)
})

/** Publishes through Soketi's HTTP API, which needs the Pusher auth signature. */
async function publish(channel: string, event: string, payload: unknown): Promise<void> {
  const appId = process.env.SOKETI_APP_ID || ''
  const secret = process.env.SOKETI_APP_SECRET || ''
  // data is a STRING here, which is what makes the inbound frame carry a string too.
  const body = JSON.stringify({ name: event, channel, data: JSON.stringify(payload) })

  const { createHash, createHmac } = await import('node:crypto')
  const bodyMd5 = createHash('md5').update(body).digest('hex')
  const timestamp = Math.floor(Date.now() / 1000)
  const params = `auth_key=${options.key}&auth_timestamp=${timestamp}&auth_version=1.0&body_md5=${bodyMd5}`
  const signature = createHmac('sha256', secret)
    .update(`POST\n/apps/${appId}/events\n${params}`)
    .digest('hex')

  const response = await fetch(
    `http://${options.host}:${options.port}/apps/${appId}/events?${params}&auth_signature=${signature}`,
    { method: 'POST', headers: { 'Content-Type': 'application/json' }, body },
  )
  if (!response.ok) {
    throw new Error(`publish failed: ${response.status} ${await response.text()}`)
  }
}

function sleep(ms: number): Promise<void> {
  return new Promise((resolve) => setTimeout(resolve, ms))
}

async function waitFor(condition: () => boolean, timeoutMs: number): Promise<void> {
  const deadline = Date.now() + timeoutMs
  while (Date.now() < deadline) {
    if (condition()) return
    await sleep(100)
  }
  throw new Error('timed out waiting for a condition')
}

import { describe, expect, it } from 'vitest'

import {
  applyLocalVote,
  champion,
  createInitialSnapshot,
  finalDisplayedPair,
  type LocalGameSnapshot,
} from './localGame'
import type { GameSession } from '../services/gameplay'

function session(count: number): GameSession {
  return {
    game_serial: 'g',
    post: { serial: 'p', title: 'post' },
    server_vote_count: 0,
    elements: Array.from({ length: count }, (_, index) => ({
      id: index + 1,
      title: `e${index + 1}`,
    })),
  } as unknown as GameSession
}

function play(count: number, pick: 'left' | 'right'): LocalGameSnapshot {
  const game: LocalGameSnapshot = createInitialSnapshot(session(count), 'w', 'l')
  let guard = 0
  while (game.status === 'playing' && guard < 500) {
    guard += 1
    const match = game.current_match
    if (!match) break
    const winnerId = pick === 'left' ? match.left_id : match.right_id
    const loserId = pick === 'left' ? match.right_id : match.left_id
    applyLocalVote(game, winnerId, loserId, `v${guard}`)
  }
  return game
}

function lastMatch(game: LocalGameSnapshot) {
  const last = game.match_history[0]
  if (!last) throw new Error('the game recorded no matches')
  return last
}

describe('the champion is the element whose panel was clicked last', () => {
  for (const count of [2, 3, 5, 6, 8, 16, 17]) {
    for (const pick of ['left', 'right'] as const) {
      it(`${count} elements, always picking the ${pick} panel`, () => {
        const game = play(count, pick)
        const last = lastMatch(game)
        expect(champion(game)?.id).toBe(last.winner_id)
        expect(last.winner_side).toBe(pick)
      })
    }
  }
})

describe('finalDisplayedPair reports the last pair left-first', () => {
  for (const count of [2, 3, 5, 6, 8, 16, 17]) {
    it(`${count} elements, right panel picked: the winner is reported second`, () => {
      const game = play(count, 'right')
      const last = lastMatch(game)
      expect(finalDisplayedPair(game)).toEqual([last.loser_id, last.winner_id])
    })
    it(`${count} elements, left panel picked: the winner is reported first`, () => {
      const game = play(count, 'left')
      const last = lastMatch(game)
      expect(finalDisplayedPair(game)).toEqual([last.winner_id, last.loser_id])
    })
  }

  it('a game still in play has no final pair', () => {
    const game = createInitialSnapshot(session(8), 'w', 'l')
    expect(finalDisplayedPair(game)).toBeNull()
  })
})

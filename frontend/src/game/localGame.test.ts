import { describe, expect, it } from 'vitest'

import {
  applyLocalVote,
  chooseNextMatch,
  createInitialSnapshot,
  matchesForStage,
  restoreLegacySnapshot,
  restoreSnapshot,
} from './localGame'
import type { GameSession } from '../services/gameplay'

function session(count: number): GameSession {
  return {
    game_serial: 'game-1',
    server_vote_count: 0,
    post: {
      title: 'Test', serial: 'post-1', description: '', is_censored: false,
      elements_count: count, max_elements: count,
    },
    elements: Array.from({ length: count }, (_, index) => ({
      id: index + 1, title: `Element ${index + 1}`, type: 'image', source_url: null,
      thumb_url: null, mediumthumb_url: null, lowthumb_url: null,
      video_start_second: null, video_end_second: null, video_source: null,
      video_id: null, video_duration_second: null,
    })),
  }
}

describe('local game', () => {
  it('uses the legacy-compatible stage sizes', () => {
    expect(matchesForStage(1, 45)).toBe(23)
    expect(matchesForStage(2, 22)).toBe(6)
    expect(matchesForStage(2, 16)).toBe(8)
    expect(matchesForStage(3, 8)).toBe(4)
  })

  it('persists each local vote before cloud acknowledgement', () => {
    const game = createInitialSnapshot(session(2), 'writer', 'lease', () => 0)
    const match = game.current_match!
    applyLocalVote(game, match.left_id, match.right_id, 'vote-1', () => 0)

    expect(game.status).toBe('completed')
    expect(game.outbox).toHaveLength(1)
    expect(game.local_votes).toHaveLength(1)
    expect(game.match_history[0]).toEqual(expect.objectContaining({
      winner_side: 'left', current_round: 1, of_round: 1,
    }))
    expect(restoreSnapshot(JSON.stringify(game), 'post-1')?.outbox).toHaveLength(1)
  })

  it('may reshuffle an unvoted pair on reload without changing completed local progress', () => {
    const game = createInitialSnapshot(session(8), 'writer', 'lease', () => 0)
    const firstMatch = game.current_match!
    applyLocalVote(game, firstMatch.left_id, firstMatch.right_id, 'vote-1', () => 0)
    const savedVotes = structuredClone(game.local_votes)
    const savedOutbox = structuredClone(game.outbox)
    const savedHistory = structuredClone(game.match_history)
    const originalPair = [game.current_match!.left_id, game.current_match!.right_id]

    game.current_match = null
    chooseNextMatch(game, () => 0.999, true)

    expect([game.current_match!.left_id, game.current_match!.right_id]).not.toEqual(originalPair)
    expect(game.local_votes).toEqual(savedVotes)
    expect(game.outbox).toEqual(savedOutbox)
    expect(game.match_history).toEqual(savedHistory)
  })

  it('rejects snapshots belonging to another post', () => {
    const game = createInitialSnapshot(session(4), 'writer', 'lease', () => 0)
    expect(restoreSnapshot(JSON.stringify(game), 'other-post')).toBeNull()
  })

  it('migrates the previous Vue game snapshot without removing its outbox', () => {
    const legacy = {
      schemaVersion: 3,
      clientMode: true,
      gameSerial: 'legacy-game',
      elementsCount: 2,
      localElements: session(2).elements.map((element) => ({
        ...element, local_win_count: 0, local_eliminated: false, local_played: 0, local_is_ready: true,
      })),
      localVotes: [{ local_vote_id: 'vote-1', winner_id: 1, loser_id: 2 }],
      unsentVotes: [{ local_vote_id: 'vote-1', winner_id: 1, loser_id: 2 }],
      serverVoteCount: 0,
      clientState: { stage: 1, matchesInStage: 1, targetMatches: 1, stageStartCount: 2 },
      matchHistory: [{
        id: 'history-1', winSide: 'right',
        winner: { title: 'Element 1', thumb: 'https://example.test/winner.webp' },
        loser: { title: 'Element 2', thumb: 'https://example.test/loser.webp' },
      }],
      localStateRevision: 8,
    }
    const migrated = restoreLegacySnapshot(JSON.stringify(legacy), 'post-1')
    expect(migrated?.game_serial).toBe('legacy-game')
    expect(migrated?.outbox).toEqual([{ local_vote_id: 'vote-1', winner_id: 1, loser_id: 2 }])
    expect(migrated?.match_history[0]).toEqual(expect.objectContaining({
      winner_id: 1,
      loser_id: 2,
      winner_side: 'right',
      winner_thumb: 'https://example.test/winner.webp',
      loser_thumb: 'https://example.test/loser.webp',
    }))
    expect(migrated?.revision).toBe(8)
  })

  it('completes every tested bracket in exactly n minus one local votes', () => {
    for (let count = 2; count <= 80; count += 1) {
      const game = createInitialSnapshot(session(count), 'writer', 'lease', () => 0)
      while (game.status === 'playing') {
        const match = game.current_match
        expect(match).not.toBeNull()
        applyLocalVote(game, match!.left_id, match!.right_id, `vote-${game.local_votes.length + 1}`, () => 0)
      }
      expect(game.local_votes, `candidate count ${count}`).toHaveLength(count - 1)
      expect(game.elements.filter((element) => !element.local_eliminated)).toHaveLength(1)
    }
  })
})

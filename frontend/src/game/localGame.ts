import type { GameElement, GameSession, GameVote } from '../services/gameplay'

export const LOCAL_GAME_SCHEMA_VERSION = 1

export interface LocalGameElement extends GameElement {
  local_win_count: number
  local_eliminated: boolean
  local_played: number
  local_is_ready: boolean
}

export interface LocalVote extends GameVote {
  local_vote_id: string
}

export interface LocalMatch {
  left_id: number
  right_id: number
  current_round: number
  of_round: number
  remain_elements: number
  stage: number
}

export interface MatchHistoryItem {
  vote_id: string
  winner_id: number
  loser_id: number
  winner_title: string
  loser_title: string
  winner_thumb?: string
  loser_thumb?: string
  winner_side?: 'left' | 'right'
  current_round?: number
  of_round?: number
}

export interface ClientState {
  stage: number
  matches_in_stage: number
  target_matches: number
  stage_start_count: number
}

export type LocalGameStatus = 'playing' | 'completed'

export interface LocalGameSnapshot {
  schema_version: number
  post_serial: string
  post_title: string
  game_serial: string
  selected_count: number
  elements: LocalGameElement[]
  local_votes: LocalVote[]
  outbox: LocalVote[]
  in_flight: LocalVote[] | null
  server_vote_count: number
  client_state: ClientState
  current_match: LocalMatch | null
  match_history: MatchHistoryItem[]
  status: LocalGameStatus
  cloud_sync_disabled: boolean
  cloud_sync_reason: string | null
  writer_id: string
  lease_token: string
  revision: number
  updated_at: number
}

export function createInitialSnapshot(
  session: GameSession,
  writerId: string,
  leaseToken: string,
  random: () => number = Math.random,
): LocalGameSnapshot {
  const elements = session.elements.map((element) => ({
    ...element,
    local_win_count: 0,
    local_eliminated: false,
    local_played: 0,
    local_is_ready: true,
  }))
  const snapshot: LocalGameSnapshot = {
    schema_version: LOCAL_GAME_SCHEMA_VERSION,
    post_serial: session.post.serial,
    post_title: session.post.title,
    game_serial: session.game_serial,
    selected_count: elements.length,
    elements,
    local_votes: [],
    outbox: [],
    in_flight: null,
    server_vote_count: session.server_vote_count,
    client_state: {
      stage: 1,
      matches_in_stage: 0,
      target_matches: matchesForStage(1, elements.length),
      stage_start_count: elements.length,
    },
    current_match: null,
    match_history: [],
    status: 'playing',
    cloud_sync_disabled: false,
    cloud_sync_reason: null,
    writer_id: writerId,
    lease_token: leaseToken,
    revision: 1,
    updated_at: Date.now(),
  }
  return chooseNextMatch(snapshot, random)
}

export function chooseNextMatch(
  snapshot: LocalGameSnapshot,
  random: () => number = Math.random,
  randomizeReadyCandidates = false,
): LocalGameSnapshot {
  if (snapshot.status === 'completed') return snapshot
  const active = snapshot.elements.filter((element) => !element.local_eliminated)
  if (active.length < 2) {
    snapshot.status = 'completed'
    snapshot.current_match = null
    return snapshot
  }

  if (snapshot.client_state.matches_in_stage >= snapshot.client_state.target_matches) {
    snapshot.client_state.stage += 1
    snapshot.client_state.matches_in_stage = 0
    snapshot.client_state.stage_start_count = active.length
    snapshot.client_state.target_matches = matchesForStage(snapshot.client_state.stage, active.length)
    active.forEach((element) => { element.local_is_ready = true })
  }

  let ready = shuffled(active.filter((element) => element.local_is_ready), random)
  if (ready.length === 0) {
    // A partially written older snapshot should remain playable locally.
    active.forEach((element) => { element.local_is_ready = true })
    ready = shuffled(active, random)
  } else if (snapshot.client_state.stage === 2 && !randomizeReadyCandidates) {
    ready.sort((left, right) => left.local_played - right.local_played)
  }

  const left = ready[0]
  if (!left) return snapshot
  const right = ready[1] ?? shuffled(
    active.filter((element) => element.id !== left.id && !element.local_is_ready),
    random,
  )[0]
  if (!right) return snapshot

  snapshot.current_match = {
    left_id: left.id,
    right_id: right.id,
    current_round: snapshot.client_state.matches_in_stage + 1,
    of_round: snapshot.client_state.target_matches,
    remain_elements: active.length,
    stage: snapshot.client_state.stage,
  }
  return snapshot
}

export function applyLocalVote(
  snapshot: LocalGameSnapshot,
  winnerId: number,
  loserId: number,
  voteId: string,
  random: () => number = Math.random,
): LocalGameSnapshot {
  const current = snapshot.current_match
  if (!current || ![current.left_id, current.right_id].includes(winnerId)
    || ![current.left_id, current.right_id].includes(loserId) || winnerId === loserId) {
    throw new Error('vote does not match the current local round')
  }
  const winner = snapshot.elements.find((element) => element.id === winnerId)
  const loser = snapshot.elements.find((element) => element.id === loserId)
  if (!winner || !loser || winner.local_eliminated || loser.local_eliminated) {
    throw new Error('local game state is no longer playable')
  }

  winner.local_win_count += 1
  winner.local_played += 1
  winner.local_is_ready = false
  loser.local_played += 1
  loser.local_eliminated = true
  loser.local_is_ready = false

  const vote: LocalVote = { local_vote_id: voteId, winner_id: winnerId, loser_id: loserId }
  snapshot.local_votes.push(vote)
  snapshot.outbox.push(vote)
  snapshot.match_history.unshift({
    vote_id: voteId,
    winner_id: winnerId,
    loser_id: loserId,
    winner_title: winner.title,
    loser_title: loser.title,
    winner_side: current.left_id === winnerId ? 'left' : 'right',
    current_round: current.current_round,
    of_round: current.of_round,
  })
  snapshot.match_history = snapshot.match_history.slice(0, 80)
  snapshot.client_state.matches_in_stage += 1
  snapshot.current_match = null
  snapshot.revision += 1
  snapshot.updated_at = Date.now()
  return chooseNextMatch(snapshot, random)
}

export function restoreSnapshot(raw: string | null, postSerial: string): LocalGameSnapshot | null {
  if (!raw) return null
  try {
    const snapshot = JSON.parse(raw) as Partial<LocalGameSnapshot>
    if (snapshot.schema_version !== LOCAL_GAME_SCHEMA_VERSION
      || snapshot.post_serial !== postSerial
      || typeof snapshot.game_serial !== 'string'
      || !Array.isArray(snapshot.elements)
      || snapshot.elements.length < 2
      || !Array.isArray(snapshot.local_votes)
      || !Array.isArray(snapshot.outbox)
      || !snapshot.client_state
      || typeof snapshot.revision !== 'number') {
      return null
    }
    return snapshot as LocalGameSnapshot
  } catch {
    return null
  }
}

/**
 * Converts the schema-v3 snapshot produced by the Laravel/Vue 2 game. The
 * source record is intentionally left untouched so cutover can always roll
 * back without losing an in-progress local game.
 */
export function restoreLegacySnapshot(raw: string | null, postSerial: string): LocalGameSnapshot | null {
  if (!raw) return null
  try {
    const legacy = JSON.parse(raw) as Record<string, any>
    if (legacy.schemaVersion !== 3 || legacy.clientMode !== true
      || typeof legacy.gameSerial !== 'string'
      || !Array.isArray(legacy.localElements)
      || legacy.localElements.length < 2
      || !legacy.clientState) {
      return null
    }
    const normalizeVote = (vote: any, index: number): LocalVote | null => {
      const winnerId = Number(vote?.winner_id)
      const loserId = Number(vote?.loser_id)
      if (!Number.isInteger(winnerId) || !Number.isInteger(loserId) || winnerId <= 0 || loserId <= 0) return null
      return {
        local_vote_id: String(vote.local_vote_id || `${legacy.gameSerial}:legacy:${index + 1}`),
        winner_id: winnerId,
        loser_id: loserId,
      }
    }
    const localVotes = (Array.isArray(legacy.localVotes) ? legacy.localVotes : [])
      .map(normalizeVote).filter((vote: LocalVote | null): vote is LocalVote => vote !== null)
    const outbox = (Array.isArray(legacy.unsentVotes) ? legacy.unsentVotes : [])
      .map(normalizeVote).filter((vote: LocalVote | null): vote is LocalVote => vote !== null)
    const knownOutboxIds = new Set(outbox.map((vote) => vote.local_vote_id))
    if (Array.isArray(legacy.inFlightBatch?.votes)) {
      legacy.inFlightBatch.votes.forEach((vote: any, index: number) => {
        const normalized = normalizeVote(vote, localVotes.length + index)
        if (normalized && !knownOutboxIds.has(normalized.local_vote_id)) {
          outbox.push(normalized)
          knownOutboxIds.add(normalized.local_vote_id)
        }
      })
    }
    const elements = legacy.localElements.map((element: any) => ({
      ...element,
      id: Number(element.id),
      local_win_count: Number(element.local_win_count || 0),
      local_eliminated: element.local_eliminated === true,
      local_played: Number(element.local_played || 0),
      local_is_ready: element.local_is_ready === true,
    })) as LocalGameElement[]
    const current = legacy.currentLocalMatch
    const activeCount = elements.filter((element) => !element.local_eliminated).length
    return {
      schema_version: LOCAL_GAME_SCHEMA_VERSION,
      post_serial: postSerial,
      post_title: '',
      game_serial: legacy.gameSerial,
      selected_count: Number(legacy.elementsCount || elements.length),
      elements,
      local_votes: localVotes,
      outbox,
      in_flight: outbox.length ? [...outbox] : null,
      server_vote_count: Number.isInteger(Number(legacy.serverVoteCount))
        ? Number(legacy.serverVoteCount)
        : Math.max(0, localVotes.length - outbox.length),
      client_state: {
        stage: Number(legacy.clientState.stage || 1),
        matches_in_stage: Number(legacy.clientState.matchesInStage || 0),
        target_matches: Number(legacy.clientState.targetMatches || matchesForStage(1, elements.length)),
        stage_start_count: Number(legacy.clientState.stageStartCount || elements.length),
      },
      current_match: current ? {
        left_id: Number(current.left_id),
        right_id: Number(current.right_id),
        current_round: Number(current.current_round || 1),
        of_round: Number(current.of_round || 1),
        remain_elements: Number(current.remain_elements || activeCount),
        stage: Number(legacy.clientState.stage || 1),
      } : null,
      match_history: (Array.isArray(legacy.matchHistory) ? legacy.matchHistory : []).map((item: any, index: number) => ({
        vote_id: String(item.id || `${legacy.gameSerial}:history:${index}`),
        winner_id: Number(item.winner_id || elements.find((element) => element.title === item.winner?.title)?.id || 0),
        loser_id: Number(item.loser_id || elements.find((element) => element.title === item.loser?.title)?.id || 0),
        winner_title: String(item.winner?.title || ''),
        loser_title: String(item.loser?.title || ''),
        winner_thumb: typeof item.winner?.thumb === 'string' ? item.winner.thumb : undefined,
        loser_thumb: typeof item.loser?.thumb === 'string' ? item.loser.thumb : undefined,
        winner_side: item.winSide === 'left' || item.winSide === 'right' ? item.winSide : undefined,
      })),
      status: activeCount < 2 ? 'completed' : 'playing',
      cloud_sync_disabled: legacy.localOnlyAfterBatchConflict === true || Boolean(legacy.localBranchId),
      cloud_sync_reason: legacy.cloudSyncDisabledReason || legacy.localBranchReason || null,
      writer_id: String(legacy.writerId || ''),
      lease_token: String(legacy.writerLeaseToken || ''),
      revision: Math.max(1, Number(legacy.localStateRevision || 0)),
      updated_at: Number(legacy.updatedAt || Date.now()),
    }
  } catch {
    return null
  }
}

export function champion(snapshot: LocalGameSnapshot): LocalGameElement | null {
  return snapshot.elements.find((element) => !element.local_eliminated) ?? null
}

/**
 * The final pair, in the order the two candidates were shown, left first.
 *
 * The server records a winner and a loser per round, never a side, so this is the only
 * thing that knows which finalist stood on the left. The home page's champion rail
 * places the two finalists in this order; without it every entry there shows the winner
 * on the left, which reads as "the left one always wins".
 *
 * Null for a game still in play, and for a snapshot written before the side was
 * recorded — the server then falls back to winner-first.
 */
export function finalDisplayedPair(snapshot: LocalGameSnapshot): [number, number] | null {
  if (snapshot.status !== 'completed') return null
  const last = snapshot.match_history[0]
  if (!last || (last.winner_side !== 'left' && last.winner_side !== 'right')) return null
  return last.winner_side === 'right'
    ? [last.loser_id, last.winner_id]
    : [last.winner_id, last.loser_id]
}

export function rankedElements(snapshot: LocalGameSnapshot): LocalGameElement[] {
  return [...snapshot.elements].sort((left, right) => {
    if (left.local_eliminated !== right.local_eliminated) return left.local_eliminated ? 1 : -1
    if (right.local_win_count !== left.local_win_count) return right.local_win_count - left.local_win_count
    return left.title.localeCompare(right.title)
  })
}

export function matchesForStage(stage: number, remain: number): number {
  if (remain <= 1) return 0
  if (stage === 1) return Math.ceil(remain / 2)
  if (stage === 2) {
    let powerOfTwo = 1
    while (powerOfTwo * 2 <= remain) powerOfTwo *= 2
    const difference = remain - powerOfTwo
    if (difference > 0) return difference
  }
  return Math.floor(remain / 2)
}

function shuffled<T>(items: T[], random: () => number): T[] {
  const copy = [...items]
  for (let index = copy.length - 1; index > 0; index -= 1) {
    const randomIndex = Math.floor(random() * (index + 1))
    const current = copy[index]
    const swap = copy[randomIndex]
    if (current === undefined || swap === undefined) continue
    copy[index] = swap
    copy[randomIndex] = current
  }
  return copy
}

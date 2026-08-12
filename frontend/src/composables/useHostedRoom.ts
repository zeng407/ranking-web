import { computed, ref, type Ref } from 'vue'

import { APIError } from '../lib/api'
import { getGameRoomService, type GameRoomService } from '../services/gameRoom'

/**
 * The host's side of a game room: open one for the game being played, and remember it.
 *
 * The participant's side lives in useGameRoom. This one is deliberately small — a host is
 * still playing their own local game, and the room is an attachment to it rather than a
 * mode it enters.
 *
 * WHY THE SERIAL IS PERSISTED. Opening a room is idempotent server-side (the unique index
 * on game_rooms.game_id sees to that), so a reload could simply ask again. But the host's
 * page reloads often mid-game, and re-opening on every load would mean a request before the
 * invite link could be shown. Keyed by game serial rather than by post: a new game is a new
 * room.
 */

const STORAGE_PREFIX = 'gameroom_host_'

export type HostedRoomStatus = 'closed' | 'opening' | 'open' | 'failed'

export interface UseHostedRoom {
  serial: Ref<string>
  status: Ref<HostedRoomStatus>
  hosting: Ref<boolean>
  /** The link to hand to participants, or '' when no room is open. */
  inviteURL: Ref<string>
  open(currentCandidates?: number[]): Promise<void>
  /** Forgets the room locally. The room itself keeps existing server-side. */
  forget(): void
}

export function useHostedRoom(
  gameSerial: Ref<string>,
  locale: Ref<string>,
  service: GameRoomService = getGameRoomService(),
): UseHostedRoom {
  const serial = ref(readStored(gameSerial.value))
  const status = ref<HostedRoomStatus>(serial.value ? 'open' : 'closed')

  const hosting = computed(() => Boolean(serial.value))
  const inviteURL = computed(() => {
    if (!serial.value || typeof window === 'undefined') return ''
    // The localized route, because the recipient lands on a page and reads it. The room
    // view is at /{locale}/room/{serial}.
    return new URL(`/${locale.value}/room/${encodeURIComponent(serial.value)}`,
      window.location.origin).toString()
  })

  async function open(currentCandidates?: number[]): Promise<void> {
    if (!gameSerial.value || status.value === 'opening') return
    status.value = 'opening'
    try {
      const room = await service.open(gameSerial.value, currentCandidates)
      serial.value = room.serial
      store(gameSerial.value, room.serial)
      status.value = 'open'
    } catch (error) {
      // Nothing partial to undo: the room either exists server-side or it does not, and a
      // retry is idempotent.
      status.value = 'failed'
      if (!(error instanceof APIError)) throw error
    }
  }

  function forget(): void {
    serial.value = ''
    status.value = 'closed'
    clearStored(gameSerial.value)
  }

  return { serial, status, hosting, inviteURL, open, forget }
}

/**
 * The pair to report as on screen, or undefined.
 *
 * ONLY WHEN THIS BATCH EMPTIES THE OUTBOX. games.candidates means "the pair the host is
 * looking at", and the displayed pair is the one after every LOCAL vote. If the batch is
 * only part of the outbox — a host who was offline and is catching up — the server would be
 * told about a pair that is several rounds ahead of the votes it has just recorded, and the
 * room would show its participants a match whose wagers cannot be settled yet.
 *
 * Omitting it in that case leaves the column holding the last decided pair, which is stale
 * but coherent, and the next flush corrects it.
 */
export function onScreenPairForBatch(
  batchLength: number,
  outboxLength: number,
  displayed: readonly [{ id: number }, { id: number }] | null,
): number[] | undefined {
  if (!displayed || batchLength !== outboxLength) return undefined
  return [displayed[0].id, displayed[1].id]
}

function storageKey(gameSerial: string): string {
  return STORAGE_PREFIX + gameSerial
}

function readStored(gameSerial: string): string {
  if (!gameSerial || typeof localStorage === 'undefined') return ''
  try {
    return localStorage.getItem(storageKey(gameSerial)) || ''
  } catch {
    // Private browsing can deny storage entirely; the room still works, it just has to be
    // reopened after a reload.
    return ''
  }
}

function store(gameSerial: string, roomSerial: string): void {
  if (typeof localStorage === 'undefined') return
  try {
    localStorage.setItem(storageKey(gameSerial), roomSerial)
  } catch {
    // See readStored.
  }
}

function clearStored(gameSerial: string): void {
  if (typeof localStorage === 'undefined') return
  try {
    localStorage.removeItem(storageKey(gameSerial))
  } catch {
    // See readStored.
  }
}

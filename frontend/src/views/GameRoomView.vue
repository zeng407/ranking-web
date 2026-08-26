<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch, watchEffect } from 'vue'
import { useRoute } from 'vue-router'

import { normalizeLocale, translate } from '../i18n'
import { boardRows, popularRows, uniqueRows, useGameRoom } from '../composables/useGameRoom'
import { createGameplayService, gamePreviewImage, type GameElement } from '../services/gameplay'
import { MaxNicknameLength } from '../services/gameRoom'

/**
 * A game room, from a participant's side: watch the pairing the host has up, wager on it,
 * and see the leaderboard move.
 *
 * This is not the host's view. The host plays in GameView and their votes are what settle
 * everyone's wagers; a participant only ever bets. That split is how the old UI worked too —
 * the room serial was passed into the game component and non-hosts got the betting panel.
 */

const route = useRoute()
const locale = computed(() => normalizeLocale(route.params.locale))
const roomSerial = computed(() => String(route.params.serial || ''))

const room = useGameRoom(roomSerial.value)
const gameplay = createGameplayService()

/** Element rows for the pairing, keyed by id so a vote payload can be rendered. */
const elements = ref(new Map<number, GameElement>())
const nickname = ref('')
const renaming = ref(false)

const pairing = computed(() => {
  const votes = room.votes.value
  if (!votes) return null
  return {
    first: elements.value.get(votes.first_candidate) ?? null,
    second: elements.value.get(votes.second_candidate) ?? null,
    votes,
  }
})

/**
 * The two options on screen, in the host's order, each with its preview resolved once.
 */
const sides = computed(() => {
  const current = pairing.value
  if (!current) return []
  const { votes } = current
  return [
    {
      element: current.first,
      id: votes.first_candidate,
      other: votes.second_candidate,
      votes: votes.first_candidate_votes,
    },
    {
      element: current.second,
      id: votes.second_candidate,
      other: votes.first_candidate,
      votes: votes.second_candidate_votes,
    },
  ].map((side) => ({
    ...side,
    image: gamePreviewImage(side.element ?? ({} as GameElement)) ?? '',
  }))
})

/**
 * Which boards this room shows.
 *
 * A majority room scores taste: siding with the room adds points, going alone subtracts
 * them, so the same column read from both ends is two rankings. A room the host decides has
 * only one direction worth reading and keeps the single merged list.
 */
const boards = computed(() => {
  const board = room.leaderboard.value
  if (!room.majority.value) {
    return [{ key: 'all', title: 'roomLeaderboard' as const, hint: '' as const, rows: boardRows(board) }]
  }
  return [
    {
      key: 'popular',
      title: 'roomBoardPopular' as const,
      hint: 'roomBoardPopularHint' as const,
      rows: popularRows(board),
    },
    {
      key: 'unique',
      title: 'roomBoardUnique' as const,
      hint: 'roomBoardUniqueHint' as const,
      rows: uniqueRows(board),
    },
  ]
})
const ownPlayerId = computed(() => room.player.value?.player_id ?? '')

/**
 * Which side the caller wagered on, so the pick can be highlighted.
 *
 * Checked against the pairing on screen, because the server hands back the newest wager
 * whatever round it belongs to: once the host advances, last round's pick must stop
 * highlighting or the new pairing arrives looking as though it were already voted on.
 */
const ownPick = computed(() => {
  const bet = room.ownBet.value
  const votes = room.votes.value
  if (!bet || !votes) return 0
  const onScreen = [votes.first_candidate, votes.second_candidate]
  if (!onScreen.includes(bet.winner_id) || !onScreen.includes(bet.loser_id)) return 0
  return bet.winner_id
})

/**
 * Loads the game's elements whenever the room tells us which game it is on.
 *
 * The room payload names the pairing by element id only — the images live with the game. A
 * failure here leaves the ids on screen rather than blocking the room: you can still bet,
 * you just cannot see what you are betting on, which beats not being able to bet.
 *
 * KEYED ON THE SERIAL, NOT ON WHETHER THE MAP IS EMPTY. The host restarting moves the room
 * onto a new game with new element ids, and a map left over from the old one resolves none
 * of them — the room would show two bare ids until the page was reloaded by hand.
 */
const loadedGameSerial = ref('')
watch(
  () => room.gameSerial.value,
  async (gameSerial) => {
    if (!gameSerial || gameSerial === loadedGameSerial.value) return
    try {
      const session = await gameplay.resume(gameSerial)
      // Checked after the await: a second restart can land while this read is in flight,
      // and the later game is the one on screen.
      if (room.gameSerial.value !== gameSerial) return
      elements.value = new Map(session.elements.map((element) => [element.id, element]))
      loadedGameSerial.value = gameSerial
    } catch {
      // Left as it was; the template falls back to the element id.
    }
  },
  { immediate: true },
)

// Prefills the rename box with the name the server gave, so editing starts from what is
// on screen rather than from empty.
watch(
  () => room.player.value?.name,
  (name) => {
    if (name && !renaming.value) nickname.value = name
  },
  { immediate: true },
)

async function submitRename(): Promise<void> {
  const value = nickname.value.trim()
  if (!value) return
  renaming.value = true
  try {
    await room.rename(value)
  } finally {
    renaming.value = false
  }
}

function pick(winnerId: number, loserId: number): void {
  void room.bet(winnerId, loserId)
}

function elementLabel(element: GameElement | null, fallbackId: number): string {
  return element?.title || `#${fallbackId}`
}

onMounted(() => {
  void room.join()
})

// The websocket and the poll both have to stop, or navigating away leaves a socket open
// and a timer firing against a room nobody is looking at.
onBeforeUnmount(() => room.leave())

watchEffect(() => {
  document.title = `${translate(locale.value, 'gameRoom')} · 2Pick`
})
</script>

<template>
  <div class="room">
    <p v-if="room.status.value === 'loading'" class="room-status">
      {{ translate(locale, 'roomLoading') }}
    </p>

    <p v-else-if="room.status.value === 'not-found'" class="room-status">
      {{ translate(locale, 'roomNotFound') }}
    </p>

    <div v-else-if="room.status.value === 'failed'" class="room-status">
      <p>{{ translate(locale, 'roomLoadFailed') }}</p>
      <button type="button" class="room-retry" @click="room.join()">
        {{ translate(locale, 'retry') }}
      </button>
    </div>

    <template v-else>
      <header class="room-header">
        <div>
          <h1>{{ translate(locale, 'gameRoom') }}</h1>
          <p class="room-serial">{{ roomSerial }}</p>
        </div>
        <!-- The live indicator is honest about all three states: a room on its poll is
             working, just not instant, and saying so beats implying it is broken. -->
        <p class="room-live" :data-state="room.live.value">
          <span class="room-live-dot" aria-hidden="true"></span>
          {{ translate(locale, room.live.value === 'connected' ? 'roomLive' : 'roomPolling') }}
        </p>
      </header>

      <section v-if="pairing" class="room-pairing">
        <p class="room-round">
          {{ translate(locale, 'roomRound') }}
          {{ pairing.votes.current_round }} / {{ pairing.votes.of_round }}
        </p>

        <!-- The clock the host set. It is the server's remainder, so this counts to the
             same moment the host's panel does, whatever this device thinks the time is. -->
        <p v-if="room.majority.value" class="room-round-clock" role="status">
          <template v-if="room.secondsLeft.value !== null">
            <span>{{ translate(locale, 'roomRoundRemaining') }}</span>
            <b>{{ room.secondsLeft.value }}</b>
          </template>
          <span v-else>{{ translate(locale, 'roomRoundWaitHost') }}</span>
        </p>

        <div class="room-candidates">
          <button
            v-for="side in sides"
            :key="side.id"
            type="button"
            class="room-candidate"
            :class="{ 'is-picked': ownPick === side.id }"
            :disabled="room.betting.value"
            :aria-pressed="ownPick === side.id"
            @click="pick(side.id, side.other)"
          >
            <span class="room-candidate-media">
              <span
                v-if="side.image"
                class="room-candidate-backdrop"
                :style="{ backgroundImage: `url(${side.image})` }"
                aria-hidden="true"
              ></span>
              <img
                v-if="side.image"
                :src="side.image"
                :alt="elementLabel(side.element, side.id)"
                loading="lazy"
              >
            </span>
            <span class="room-candidate-title">{{ elementLabel(side.element, side.id) }}</span>
            <span class="room-candidate-votes">{{ side.votes }}</span>
          </button>
        </div>

        <p v-if="room.actionError.value" class="room-action-error">
          {{ translate(locale, room.actionError.value as never) }}
        </p>
      </section>

      <p v-else class="room-status room-waiting">{{ translate(locale, 'roomNoRound') }}</p>

      <section v-if="room.player.value" class="room-me">
        <h2>{{ translate(locale, 'roomYou') }}</h2>
        <dl class="room-stats">
          <div>
            <dt>{{ translate(locale, room.majority.value ? 'roomTasteScore' : 'roomScore') }}</dt>
            <dd>{{ room.player.value.score }}</dd>
          </div>
          <div><dt>{{ translate(locale, 'roomRank') }}</dt><dd>{{ room.player.value.rank || '—' }}</dd></div>
          <div><dt>{{ translate(locale, 'roomAccuracy') }}</dt><dd>{{ room.player.value.accuracy }}%</dd></div>
          <!-- A taste score pays by how the room split, never by a streak, so the column
               is zero here by construction and showing it would only invite the question. -->
          <div v-if="!room.majority.value">
            <dt>{{ translate(locale, 'roomCombo') }}</dt><dd>{{ room.player.value.combo }}</dd>
          </div>
        </dl>

        <form class="room-rename" @submit.prevent="submitRename">
          <label>
            <span>{{ translate(locale, 'roomNickname') }}</span>
            <input
              v-model="nickname"
              type="text"
              :maxlength="MaxNicknameLength"
              autocomplete="off"
            >
          </label>
          <button type="submit" :disabled="renaming || !nickname.trim()">
            {{ translate(locale, 'roomSave') }}
          </button>
        </form>
      </section>

      <section v-for="(board, index) in boards" :key="board.key" class="room-board">
        <h2>
          {{ translate(locale, board.title) }}
          <span v-if="index === 0 && room.leaderboard.value" class="room-board-count">
            {{ room.leaderboard.value.total_users }}
          </span>
        </h2>
        <p v-if="board.hint" class="room-board-hint">{{ translate(locale, board.hint) }}</p>
        <ol class="room-board-list">
          <li
            v-for="row in board.rows"
            :key="row.user_id"
            :class="{ 'is-me': row.user_id === ownPlayerId }"
          >
            <span class="room-board-rank">{{ row.rank || '—' }}</span>
            <span class="room-board-name">{{ row.name }}</span>
            <span class="room-board-score">{{ row.score }}</span>
            <span class="room-board-accuracy">{{ row.accuracy }}%</span>
          </li>
        </ol>
        <p v-if="board.rows.length === 0" class="room-status">
          {{ translate(locale, 'roomNoPlayers') }}
        </p>
      </section>
    </template>
  </div>
</template>

<style scoped>
.room {
  max-width: 60rem;
  margin: 0 auto;
  padding: 1.5rem 1rem 4rem;
  display: grid;
  gap: 1.5rem;
}

.room-header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 1rem;
}

.room-header h1 {
  margin: 0;
  font-size: 1.5rem;
}

.room-serial {
  margin: 0.25rem 0 0;
  font-family: ui-monospace, monospace;
  opacity: 0.7;
}

.room-live {
  display: inline-flex;
  align-items: center;
  gap: 0.4rem;
  margin: 0;
  font-size: 0.85rem;
  opacity: 0.8;
}

.room-live-dot {
  width: 0.5rem;
  height: 0.5rem;
  border-radius: 50%;
  background: currentColor;
  opacity: 0.4;
}

.room-live[data-state='connected'] .room-live-dot {
  opacity: 1;
  color: #22c55e;
}

.room-round {
  margin: 0 0 0.75rem;
  font-variant-numeric: tabular-nums;
}

/* The host's clock, counting the server's remainder rather than this device's idea of the
   time, so it hits zero at the same moment the host's does. */
.room-round-clock {
  display: flex;
  margin: 0 0 0.75rem;
  align-items: baseline;
  justify-content: center;
  gap: 0.5rem;
  padding: 0.4rem 0.75rem;
  border-radius: 999px;
  background: rgba(127, 127, 127, 0.12);
  font-size: 0.85rem;
  font-variant-numeric: tabular-nums;
}

.room-round-clock b {
  font-size: 1.4rem;
  line-height: 1;
}

.room-candidates {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 0.75rem;
}

.room-candidate {
  display: grid;
  gap: 0.5rem;
  padding: 0.75rem;
  border: 2px solid transparent;
  border-radius: 0.75rem;
  background: rgba(127, 127, 127, 0.1);
  cursor: pointer;
  text-align: center;
  font: inherit;
  color: inherit;
}

.room-candidate:disabled {
  cursor: progress;
  opacity: 0.7;
}

.room-candidate.is-picked {
  border-color: #22c55e;
}

/* Contained, not cropped. A participant is wagering on the whole picture, so the frame
   keeps a fixed shape and the blurred copy behind it fills whatever the aspect ratio
   leaves over — the same treatment the host's arena gives an option. */
.room-candidate-media {
  position: relative;
  overflow: hidden;
  aspect-ratio: 4 / 3;
  border-radius: 0.5rem;
  background: rgba(127, 127, 127, 0.14);
}

.room-candidate-backdrop {
  position: absolute;
  inset: 0;
  background-position: center;
  background-size: cover;
  filter: blur(22px) saturate(140%);
  opacity: 0.5;
  transform: scale(1.15);
}

.room-candidate-media img {
  position: absolute;
  display: block;
  inset: 0;
  width: 100%;
  height: 100%;
  object-fit: contain;
}

.room-candidate-votes {
  font-size: 1.25rem;
  font-variant-numeric: tabular-nums;
}

.room-action-error {
  margin: 0.75rem 0 0;
  color: #dc2626;
}

.room-stats {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(6rem, 1fr));
  gap: 0.75rem;
  margin: 0 0 1rem;
}

.room-stats dt {
  font-size: 0.8rem;
  opacity: 0.7;
}

.room-stats dd {
  margin: 0.15rem 0 0;
  font-size: 1.15rem;
  font-variant-numeric: tabular-nums;
}

.room-rename {
  display: flex;
  align-items: flex-end;
  gap: 0.5rem;
  flex-wrap: wrap;
}

.room-rename label {
  display: grid;
  gap: 0.25rem;
  font-size: 0.85rem;
}

.room-board-hint {
  margin: -0.5rem 0 0;
  font-size: 0.8rem;
  opacity: 0.7;
}

.room-board-list {
  list-style: none;
  margin: 0;
  padding: 0;
  display: grid;
  gap: 0.25rem;
}

.room-board-list li {
  display: grid;
  grid-template-columns: 2.5rem 1fr auto auto;
  gap: 0.75rem;
  padding: 0.4rem 0.6rem;
  border-radius: 0.5rem;
  font-variant-numeric: tabular-nums;
}

.room-board-list li.is-me {
  background: rgba(34, 197, 94, 0.15);
  font-weight: 600;
}

.room-board-rank,
.room-board-accuracy {
  opacity: 0.75;
}

.room-status {
  opacity: 0.75;
}

@media (max-width: 30rem) {
  .room-candidates {
    grid-template-columns: 1fr;
  }
}
</style>

<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'

import AdSlot from '../components/AdSlot.vue'
import CommentSection from '../components/CommentSection.vue'
import RankingExportDialog from '../components/RankingExportDialog.vue'
import RankingSkeleton from '../components/RankingSkeleton.vue'
import {
  applyLocalVote,
  champion,
  chooseNextMatch,
  createInitialSnapshot,
  finalDisplayedPair,
  rankedElements,
  restoreLegacySnapshot,
  restoreSnapshot,
  type LocalGameElement,
  type LocalGameSnapshot,
} from '../game/localGame'
import { APIError } from '../lib/api'
import { useAuth } from '../composables/useAuth'
import { closeImageViewer, openImageViewer } from '../services/imageViewer'
import { unlockPost } from '../services/postAccess'
import { boardRows, popularRows, uniqueRows } from '../composables/useGameRoom'
import {
  DEFAULT_ROUND_SECONDS,
  MAX_ROUND_SECONDS,
  MIN_ROUND_SECONDS,
  onScreenPairForBatch,
  useHostedRoom,
} from '../composables/useHostedRoom'
import type { RoundVotes } from '../services/gameRoom'
import { getAnonymousID } from '../lib/anonymousId'
import { downloadQRCode, drawQRCode } from '../lib/qrcode'
import { shareOrCopyLink } from '../lib/share'
import { localeDefinition, localizedPath, normalizeLocale, translate, type MessageKey } from '../i18n'
import {
  isRankTrendCharted,
  rankTrendCoordinates,
  rankTrendDomain,
  rankTrendGridlines,
  rankTrendPointRadius,
  rankTrendPolyline,
  rankTrendViewBox,
} from '../rank/rankTrend'
import type { RankingExportItem } from '../rank/exportRanking'
import {
  createGameplayService,
  fullSizeImage,
  gamePreviewImage,
  preferredGameImage,
  youtubeEmbedURL,
  type GameDefinition,
  type GameResult,
} from '../services/gameplay'
import {
  createPublicContentService,
  type RankDetails,
  type RankElement,
  type RankHistoryPoint,
  type RankReport,
  type RanksPage,
} from '../services/publicContent'

interface GameLease {
  owner_id: string
  token: string
  game_serial: string
  expires_at: number
}

const leaseDuration = 120_000
const leaseHeartbeat = 5_000
const syncDelay = 1_200
const resultMinimumLoadingDuration = 2_000
const resultMediaTimeout = 4_000

const route = useRoute()
const router = useRouter()
const locale = computed(() => normalizeLocale(route.params.locale))
const postSerial = computed(() => String(route.params.serial || ''))
const rankOnly = computed(() => String(route.name || '').startsWith('rank'))
/**
 * Which finished game's personal ranking this page is showing.
 *
 * `g` is what this client writes; `s` is the same serial under the name the Blade share
 * button used, and links carrying it are already in circulation. Both are read so a shared
 * result keeps resolving to a result rather than to the community ranking.
 */
const resultGameSerial = computed(() => {
  const named = typeof route.query.g === 'string' ? route.query.g : route.query.s
  return typeof named === 'string' ? named.trim() : ''
})
const service = createGameplayService()
const publicContentService = createPublicContentService()
const writerId = randomId()
const definition = ref<GameDefinition | null>(null)
/**
 * Ads are off for an 18+ post, on both its game and its ranking page: AdSense does
 * not allow its units on adult content, and the penalty is the whole account.
 */
const adsAllowed = computed(() => definition.value !== null && !definition.value.is_censored)
const { authenticated, loading: authLoading, refreshAuthState } = useAuth()
const censored = computed(() => definition.value?.is_censored === true)
/**
 * Whether this post is gated at all, whoever is looking.
 *
 * Not the same question as `censored`: an 18+ post is blurred and ad-free everywhere,
 * but whether it also needs an account is a deployment setting, so the server sends the
 * answer with the definition rather than the page inferring it. An API that predates the
 * setting sends no field, and the gate was on for every 18+ post there.
 */
const signInGated = computed(() => definition.value?.requires_sign_in ?? censored.value)
/**
 * Whether this visitor has to sign in before the page will do anything.
 *
 * A gated post stays previewable — it is listed on the home page, and its two blurred
 * thumbnails show here — but playing it, voting on it and reading its ranking need an
 * account. The server enforces all three with a 401; this only decides what to render, and
 * waits for `authLoading` so a signed-in visitor never sees the prompt flash past.
 */
const signInRequired = computed(() => signInGated.value && !authLoading.value && !authenticated.value)
const signInTarget = computed(() => ({
  path: localizedPath('/login', locale.value),
  query: { redirect: route.fullPath },
}))
// The ranking row an in-content ad follows, counted from zero.
const rankAdAfterRow = 9
const snapshot = ref<LocalGameSnapshot | null>(null)
const serverResult = ref<GameResult | null>(null)
const selectedCount = ref(32)
const loading = ref(true)
const creating = ref(false)
const loadError = ref(false)
/**
 * The door code on a password-protected post.
 *
 * The API answers 404 both for a post that does not exist and for one this visitor may
 * not see — it will not tell a stranger which serials are real. So a 404 here opens the
 * prompt rather than the error card, and the prompt says as much: a mistyped link lands
 * in the same place. The unlock call is what tells the two apart, because it answers 403
 * for a wrong code and 404 for a post that truly is not there.
 */
const doorCodeRequired = ref(false)
const doorCode = ref('')
const doorCodeError = ref<MessageKey | null>(null)
const unlocking = ref(false)
const readOnly = ref(false)
const syncing = ref(false)
const networkPending = ref(false)
const animating = ref(false)
const animationPair = ref<[LocalGameElement, LocalGameElement] | null>(null)
const animationWinnerId = ref<number | null>(null)
const resultTab = ref<'mine' | 'community'>(rankOnly.value ? 'community' : 'mine')
const communityRanks = ref<RanksPage>({ items: [], page: 1, per_page: 20, total: 0, total_pages: 0 })
const communityLoading = ref(false)
const communityError = ref(false)
const selectedCommunityRank = ref<RankReport | null>(null)
const rankDetails = ref<RankDetails | null>(null)
/**
 * The video a ranking row opened, and whether it is docked in the corner.
 *
 * Dismissing the overlay docks the player instead of unmounting it, so leaving the
 * big view does not interrupt what is playing.
 */
const openedVideo = ref<{ embedURL: string; rank: string; title: string } | null>(null)
const videoDocked = ref(false)
const trendLoading = ref(false)
/**
 * Whether the wait has lasted long enough to be worth showing. A cached details
 * read comes back in a few frames, and swapping the chart for a spinner and back
 * again in that time reads as the card tearing rather than as loading.
 */
const trendLoaderVisible = ref(false)
const trendLoaderDelay = 220
let trendLoaderTimer: number | undefined
const trendError = ref(false)
const restartDialog = ref<HTMLDialogElement | null>(null)
const multiplayerDialog = ref<HTMLDialogElement | null>(null)
/**
 * Which part of the multiplayer dialog is on screen.
 *
 * `mode` is the choice of game mode, `settings` is how a majority room ends its rounds, and
 * `invite` is the link to hand out. A host never leaves their own game for a room, so these
 * three steps are the whole of hosting.
 */
const multiplayerStep = ref<'mode' | 'settings' | 'invite'>('mode')
/** Where 確定 goes from the settings step; see openRoundSettings. */
const settingsNext = ref<'invite' | 'close'>('close')
/** Whether a majority room runs on a clock. False means the host ends every round by hand. */
const roundTimed = ref(true)
/** The countdown the host is about to set, in seconds. Clamped when it is sent. */
const roundSeconds = ref(DEFAULT_ROUND_SECONDS)
const votingPending = ref(false)
const votingError = ref(false)
/**
 * True while a round is being settled by the room.
 *
 * The winner is read from the server, so settling is not instant, and both triggers — the
 * countdown hitting zero and the button — can fire inside that window.
 */
const settlingRound = ref(false)
const rankingExportOpen = ref(false)
const restartError = ref(false)
const entryDecisionPending = ref(false)
const restartHoldActive = ref(false)
const resultPreparing = ref(false)
const resultReady = ref(false)
const shareCopied = ref(false)
const roomLinkCopied = ref(false)
let roomLinkCopiedTimer: number | undefined
const roomQRCanvas = ref<HTMLCanvasElement | null>(null)
/** Whether a code is drawn, so the download is offered only when there is one to save. */
const roomQRReady = ref(false)
const hoveredVideoID = ref<number | null>(null)
const leaseToken = ref('')
const legacyLeaseToken = ref(0)
const requestController = new AbortController()
let heartbeatTimer: number | undefined
let syncTimer: number | undefined
let animationTimer: number | undefined
let videoHoverTimer: number | undefined
let restartHoldTimer: number | undefined
let shareCopiedTimer: number | undefined
let communityRankRequestVersion = 0
let trendRequestVersion = 0
let resultPreparationVersion = 0

const storageKey = computed(() => `2pick:game:${postSerial.value}`)
const resultStorageKey = computed(() => `2pick:game-result:${postSerial.value}`)
const autoResumeKey = computed(() => `2pick:game:auto-resume:${postSerial.value}`)
const leaseKey = computed(() => `2pick:game:lease:${postSerial.value}`)
const legacyStateKey = computed(() => {
  const selectedBranch = sessionStorage.getItem(`gamebranch_selection_${postSerial.value}`)
  return selectedBranch ? `gamebranch_${postSerial.value}_${selectedBranch}` : `gamestate_${postSerial.value}`
})
const countOptions = computed(() => {
  const maximum = definition.value?.max_elements ?? 0
  return [...new Set([8, 16, 32, 64, 128, 256, maximum])]
    .filter((count) => count >= 2 && count <= maximum)
    .sort((left, right) => left - right)
})
const currentElements = computed<[LocalGameElement, LocalGameElement] | null>(() => {
  if (!snapshot.value?.current_match) return null
  const left = snapshot.value.elements.find((item) => item.id === snapshot.value?.current_match?.left_id)
  const right = snapshot.value.elements.find((item) => item.id === snapshot.value?.current_match?.right_id)
  return left && right ? [left, right] : null
})
/**
 * The pairing to draw, or null when there is none to draw yet.
 *
 * Nothing is drawn while the host is being asked continue-or-restart. Their answer replaces
 * the match either way — continuing re-picks it, restarting mints a new game — so the stored
 * one is a match nobody will ever vote on, and drawing it only flashes two entries and pulls
 * their media down for nothing.
 */
const visibleElements = computed(() => {
  if (entryDecisionPending.value) return null
  return animationPair.value ?? currentElements.value
})

/**
 * The game room this host has opened, if any.
 *
 * Tracks the game serial, and a restart mints a new one: the composable then MOVES the room
 * onto the new game rather than abandoning it, so the invite links and QR codes already
 * handed out keep working and the people holding them are told about the new pairing.
 *
 * The pair on screen travels with that move for the same reason it travels with open(): the
 * server never sees the bracket, and a game nobody has voted in yet has no pairing recorded
 * at all.
 */
const hostedRoom = useHostedRoom(
  computed(() => snapshot.value?.game_serial || ''),
  computed(() => localeDefinition(locale.value).prefix),
  undefined,
  () => {
    // visibleElements, not currentElements: a pair the host is not being shown is not a pair
    // to tell the room about. It keeps the reload down to one broadcast — the re-pick's —
    // instead of announcing the stored match first and replacing it a moment later.
    const displayed = visibleElements.value
    return displayed ? [displayed[0].id, displayed[1].id] : undefined
  },
  // Restarting navigates /r/… back to /g/…, which remounts this view. The post is the only
  // thing that survives that, so it is what the open room is carried across on.
  postSerial,
)
/**
 * The standings to show the host. Empty until somebody joins.
 *
 * A majority room scores taste — siding with the room adds, going alone subtracts — so the
 * same column read from both ends is two rankings rather than one list with a dull middle.
 * A room the host decides has only one direction worth reading and keeps the merged list.
 */
const roomBoards = computed(() => {
  const board = hostedRoom.board.value
  if (!hostedRoom.majority.value) {
    return [{ key: 'all', title: 'roomLeaderboard' as const, rows: boardRows(board) }]
  }
  return [
    { key: 'popular', title: 'roomBoardPopular' as const, rows: popularRows(board) },
    { key: 'unique', title: 'roomBoardUnique' as const, rows: uniqueRows(board) },
  ]
})

/**
 * Wagers on the pairing on screen, by element id, or null when the tally is about some
 * other match.
 *
 * The guard is the point. The tally is polled and the host votes locally, so between a vote
 * and the next read the counts still describe the match before this one — drawing them on
 * the new pair would credit other people's wagers to the wrong candidates. Either order of
 * the two ids is accepted: which one the room calls "first" is the server's business.
 */
const roundVotes = computed<Map<number, number> | null>(() => {
  const tally = hostedRoom.votes.value
  const pair = currentElements.value
  if (!tally || !pair) return null
  const counts = new Map<number, number>([
    [tally.first_candidate, tally.first_candidate_votes],
    [tally.second_candidate, tally.second_candidate_votes],
  ])
  if (!counts.has(pair[0].id) || !counts.has(pair[1].id)) return null
  return counts
})

const activeCount = computed(() => snapshot.value?.elements.filter((item) => !item.local_eliminated).length ?? 0)
const roundTitleCount = computed(() => Math.max(
  2,
  snapshot.value?.client_state.stage_start_count
    ?? snapshot.value?.current_match?.remain_elements
    ?? activeCount.value,
))
const roundTitle = computed(() => {
  if (roundTitleCount.value <= 2) return t('gameRoundFinal')
  if (roundTitleCount.value <= 4) return t('gameRoundSemifinal')
  if (roundTitleCount.value <= 8) return t('gameRoundQuarterfinal')
  return t('gameRoundOf', { count: roundTitleCount.value })
})
const winner = computed(() => snapshot.value ? champion(snapshot.value) : null)
const resultItems = computed(() => snapshot.value ? rankedElements(snapshot.value) : [])
const personalResultItems = computed(() => {
  if (serverResult.value) return serverResult.value.items.slice(0, 10)
  return resultItems.value.slice(0, 10).map((element, index) => ({
    rank: index + 1,
    win_count: element.local_win_count,
    global_rank: null,
    element,
  }))
})
const hasPersonalResult = computed(() => personalResultItems.value.length > 0)
/**
 * The player's own top three, shown as picture-first cards: the thing they ranked
 * is the picture, not the row of text beside it. Everything below stays a compact
 * two-column list so a ten-place ranking still reads at a glance.
 */
const personalPodiumItems = computed(() => personalResultItems.value.slice(0, 3))
const personalRestItems = computed(() => personalResultItems.value.slice(3))
const rankingExportItems = computed<RankingExportItem[]>(() => personalResultItems.value.map((item) => ({
  rank: item.rank,
  title: item.element.title,
  imageUrl: preferredGameImage(item.element),
})))
const canContinueCurrentGame = computed(() => !rankOnly.value && snapshot.value?.status === 'playing')
const restartDialogTitle = computed(() => canContinueCurrentGame.value ? t('gameRefreshPrompt') : t('gameNewRound'))
const localChampionLabels = computed(() => {
  if (serverResult.value?.items[0]) return [serverResult.value.items[0].element.title]
  if (winner.value) return [winner.value.title]
  const archived = restoreSnapshot(localStorage.getItem(resultStorageKey.value), postSerial.value)
  const archivedWinner = archived ? champion(archived) : null
  return archivedWinner ? [archivedWinner.title] : []
})
/**
 * How the room voted on each pairing it has decided, keyed by the pairing itself.
 *
 * Keyed on the pair rather than on the round numbers because that is what the local history
 * knows: it records matches, not the room's bracket position. The key is order-independent,
 * so the room calling a side "winner" and the local history agreeing is not assumed.
 *
 * Newest first from the server, so the first entry for a pairing wins — a bracket replayed
 * after a restart annotates its rounds with the replay rather than with the abandoned run.
 */
const roomRoundVotes = computed(() => {
  const rounds = new Map<string, RoundVotes>()
  for (const round of hostedRoom.history.value) {
    const key = pairKey(round.winner_id, round.loser_id)
    if (!rounds.has(key)) rounds.set(key, round)
  }
  return rounds
})

const historyItems = computed(() => {
  const game = snapshot.value
  if (!game) return []
  const rounds = roomRoundVotes.value
  return game.match_history.map((item) => ({
    ...item,
    winner: game.elements.find((element) => element.id === item.winner_id) ?? null,
    loser: game.elements.find((element) => element.id === item.loser_id) ?? null,
    // Null for a match played before the room opened, or in a room the host decides alone.
    roomRound: rounds.get(pairKey(item.winner_id, item.loser_id)) ?? null,
  }))
})

/** One key for a pairing, whichever way round it is given. */
function pairKey(one: number, other: number): string {
  return one < other ? `${one}:${other}` : `${other}:${one}`
}

/** The winning side's share of a decided round, as a whole percent. */
function roundShare(round: RoundVotes): number {
  const total = round.winner_votes + round.loser_votes
  if (total === 0) return 0
  return Math.round((round.winner_votes / total) * 100)
}

interface PreviewOption {
  key: string | number
  title: string
  image: string
  /** Pre-built CSS value so the URL is quoted rather than interpolated raw. */
  backdrop: string
  fallback: string | null
  isImage: boolean
}

/**
 * The two options shown before the game starts. They ride along on the
 * definition request made in onMounted, so the card paints on first load
 * instead of waiting for the player to press start.
 */
const previewOptions = computed<PreviewOption[]>(() => {
  const current = definition.value
  if (!current) return []

  return [current.element1, current.element2]
    .map((element, index): PreviewOption | null => {
      if (!element) return null
      const image = element.url || element.url2
      if (!image) return null
      return {
        key: element.id ?? index,
        title: element.title || '',
        image,
        backdrop: `url("${image.replace(/"/g, '%22')}")`,
        fallback: element.url2 && element.url2 !== image ? element.url2 : null,
        isImage: element.previewable,
      }
    })
    .filter((option): option is PreviewOption => option !== null)
})

// Falls back to the larger variant once, so a missing thumbnail does not leave
// an empty card.
function onPreviewImageError(event: Event, option: PreviewOption): void {
  const image = event.target as HTMLImageElement | null
  if (!image || !option.fallback || image.dataset.previewFallback === 'applied') return
  image.dataset.previewFallback = 'applied'
  image.src = option.fallback
}

/**
 * How far down the table a history chart is worth drawing.
 *
 * The original site drew one for the podium and the tail and gave every other
 * element a win rate alone, because a chart of ranks 40 to 44 is a flat line that
 * says nothing a percentage does not. Five keeps that judgement and states it once.
 */
const historyChartRankLimit = 5

const trendPoints = computed<RankHistoryPoint[]>(() => (
  rankDetails.value?.history.all ?? []
).filter((point) => positiveRank(point.rank) !== null))
/**
 * The standing shown above the chart. The selected list row carries it already,
 * so it is read from there first: taking it from the details response instead
 * blanked the number and dropped the win-rate bar for as long as the read took,
 * which moved everything under it twice per click.
 */
const cumulativeSelectedRank = computed(() => selectedCommunityRank.value
  ?? rankDetails.value?.groups?.cumulative
  ?? rankDetails.value?.current
  ?? null)
const trendCoordinates = computed(() => rankTrendCoordinates(trendPoints.value))
const trendPolyline = computed(() => rankTrendPolyline(trendCoordinates.value))
const trendPointRadius = computed(() => rankTrendPointRadius(trendCoordinates.value.length))
const trendFirstDate = computed(() => trendCoordinates.value[0]?.date ?? '')
const trendLastDate = computed(() => trendCoordinates.value.at(-1)?.date ?? '')
// A chart is only drawn for the ranks the shared scale covers, and only for the
// top few: everything below reads its standing off the two win rates instead.
const selectedChartRank = computed(() => positiveRank(selectedCommunityRank.value?.rank)
  ?? positiveRank(cumulativeSelectedRank.value?.rank))
const selectedRankIsCharted = computed(() => {
  const rank = selectedChartRank.value
  return rank !== null && rank <= historyChartRankLimit && isRankTrendCharted(rank)
})
const trendWorstLabel = `#${rankTrendDomain.worst}+`
// Geometry comes from the module so the gridlines and axis labels cannot drift
// away from the coordinates the points are plotted with.
const trendChart = { ...rankTrendViewBox, gridlines: rankTrendGridlines }

// Ranking rows show a still until the viewer asks for the video.
const playingRankVideoID = ref<number | null>(null)

function rankVideoEmbedURL(report: RankReport): string | null {
  return rankMediaEmbedURL(report.element)
}

function playRankVideo(report: RankReport): void {
  if (!rankVideoEmbedURL(report)) return
  playingRankVideoID.value = report.element.id
}

function stopRankVideo(): void {
  playingRankVideoID.value = null
}

onMounted(async () => {
  window.addEventListener('storage', onStorage)
  window.addEventListener('keydown', onKeydown)
  window.addEventListener('pagehide', releaseLease)
  await loadPost()
})

/**
 * Loads the post and decides what the page opens on.
 *
 * Separate from onMounted because unlocking a protected post has to run it again: the
 * first attempt is what discovers the post is protected, and everything after the door
 * code is entered is the same work.
 */
async function loadPost(): Promise<void> {
  let showSavedGameDecision = false
  try {
    definition.value = await service.definition(postSerial.value, requestController.signal)
    selectedCount.value = Math.min(32, definition.value.max_elements)
    // Nothing below this line is worth doing for a visitor who will be shown the sign-in
    // prompt instead: not the ranking requests, which would answer 401, and above all not
    // the saved game, which would put a playable board in front of the gate. Unforced, so
    // joining the header's boot request rather than rotating the refresh token again.
    if (signInGated.value) {
      if (authLoading.value) await refreshAuthState(locale.value)
      if (signInRequired.value) return
    }
    if (rankOnly.value) {
      if (resultGameSerial.value) {
        resultTab.value = 'mine'
        const archived = restoreSnapshot(localStorage.getItem(resultStorageKey.value), postSerial.value)
        if (archived?.game_serial === resultGameSerial.value && archived.status === 'completed') snapshot.value = archived
        void loadPersonalResultPage()
      } else {
        void syncArchivedResult()
        void loadCommunityRanks(1)
      }
      return
    }
    const saved = readSavedSnapshot()
    void syncArchivedResult()
    if (saved) {
      if (saved.elements.length < saved.selected_count) await recoverMissingElements(saved)
      if (!saved.post_title) saved.post_title = definition.value.title
      snapshot.value = saved
      selectedCount.value = saved.selected_count
      const autoResumeSerial = sessionStorage.getItem(autoResumeKey.value)
      if (autoResumeSerial === saved.game_serial) {
        sessionStorage.removeItem(autoResumeKey.value)
        resumeSnapshot(saved)
      } else {
        if (autoResumeSerial) sessionStorage.removeItem(autoResumeKey.value)
        entryDecisionPending.value = true
        showSavedGameDecision = true
      }
    } else {
      sessionStorage.removeItem(autoResumeKey.value)
    }
  } catch (error) {
    if (error instanceof DOMException && error.name === 'AbortError') return
    // 404 is the one answer that might not be final: it is also what a post this visitor
    // has not unlocked yet looks like.
    if (error instanceof APIError && error.status === 404) doorCodeRequired.value = true
    else loadError.value = true
  } finally {
    loading.value = false
    if (showSavedGameDecision) {
      await nextTick()
      showRestartDialog()
    }
  }
}

async function submitDoorCode(): Promise<void> {
  const code = doorCode.value.trim()
  if (!code || unlocking.value) return
  unlocking.value = true
  doorCodeError.value = null
  const outcome = await unlockPost(postSerial.value, code, undefined, requestController.signal)
  unlocking.value = false
  if (!outcome.ok) {
    if (outcome.kind === 'not-found') {
      // No such post, rather than one this visitor cannot see. Nothing to enter.
      doorCodeRequired.value = false
      loadError.value = true
      return
    }
    doorCodeError.value = outcome.kind === 'wrong-password'
      ? 'gameDoorCodeWrong'
      : outcome.kind === 'too-many' ? 'gameDoorCodeTooMany' : 'gameDoorCodeUnavailable'
    return
  }
  // The code itself is not kept anywhere — only the token the server gave back for it.
  doorCode.value = ''
  doorCodeRequired.value = false
  loading.value = true
  await loadPost()
}

onBeforeUnmount(() => {
  resultPreparationVersion += 1
  requestController.abort()
  window.removeEventListener('storage', onStorage)
  window.removeEventListener('keydown', onKeydown)
  window.removeEventListener('pagehide', releaseLease)
  if (heartbeatTimer) window.clearInterval(heartbeatTimer)
  if (syncTimer) window.clearTimeout(syncTimer)
  if (animationTimer) window.clearTimeout(animationTimer)
  if (videoHoverTimer) window.clearTimeout(videoHoverTimer)
  if (shareCopiedTimer) window.clearTimeout(shareCopiedTimer)
  if (trendLoaderTimer) window.clearTimeout(trendLoaderTimer)
  cancelRestartHold()
  hostedRoom.stopWatching()
  closeImageViewer()
  stopAllCandidateMedia()
  releaseLease()
})

function t(key: MessageKey, values: Record<string, string | number> = {}): string {
  let message: string = translate(locale.value, key)
  Object.entries(values).forEach(([name, value]) => {
    message = message.replaceAll(`{${name}}`, String(value))
  })
  return message
}

function positiveRank(rank: number | null | undefined): number | null {
  return typeof rank === 'number' && Number.isFinite(rank) && rank > 0 ? rank : null
}

function rankLabel(rank: number | null | undefined): string {
  const value = positiveRank(rank)
  return value === null ? t('gameNoRankData') : `#${value}`
}

/**
 * A win rate as a bar width. The scale is the absolute 0-100% one rather than
 * the page's own maximum, because 50% is the meaningful middle of a one-on-one
 * vote and a bar rescaled to the leader would put every list's top row at full
 * width no matter how it actually did.
 */
function winRateBarWidth(rate: string | null | undefined): string {
  const value = Number.parseFloat(rate ?? '')
  if (!Number.isFinite(value)) return '0%'
  return `${Math.min(100, Math.max(0, value))}%`
}

function resumeSnapshot(saved: LocalGameSnapshot): void {
  if (!saved.post_title) saved.post_title = definition.value?.title ?? ''
  snapshot.value = saved
  selectedCount.value = saved.selected_count
  if (!claimLease(false)) {
    readOnly.value = true
    return
  }
  readOnly.value = false
  saved.writer_id = writerId
  saved.lease_token = leaseToken.value
  // The current pair has not been voted on, so a reload may choose any still-ready
  // candidates from this stage without changing progress. Kept deliberately, rooms
  // included: a host reloads to get a different match. What the room needs is to be TOLD,
  // and it is told here rather than by whoever picked the room back up: on the usual reload
  // the host answers the saved-game dialog first, so the room is adopted — and reported —
  // while the pre-reload match is still on screen, and this re-pick happens afterwards.
  if (saved.status === 'playing') {
    saved.current_match = null
    chooseNextMatch(saved, Math.random, true)
    saved.revision += 1
    saved.updated_at = Date.now()
    void hostedRoom.reportPair()
  }
  saveSnapshot()
  if (saved.outbox.length) scheduleSync(0)
}

async function recoverMissingElements(saved: LocalGameSnapshot): Promise<void> {
  const session = await service.resume(saved.game_serial, requestController.signal)
  if (session.post.serial !== postSerial.value) throw new Error('saved game belongs to another post')
  const existingIds = new Set(saved.elements.map((element) => element.id))
  session.elements.forEach((element) => {
    if (existingIds.has(element.id)) return
    saved.elements.push({
      ...element,
      local_win_count: 0,
      local_eliminated: false,
      local_played: 0,
      local_is_ready: true,
    })
  })
  if (saved.elements.length !== saved.selected_count) {
    throw new Error('saved game candidates are incomplete')
  }
}

async function startGame(): Promise<void> {
  await createNewGame(false, false)
}

async function createNewGame(forceLease: boolean, discardLegacyOnSuccess: boolean): Promise<boolean> {
  if (!definition.value || creating.value) return false
  creating.value = true
  loadError.value = false
  const previousLegacyStateKey = legacyStateKey.value
  const previousLegacyLeaseKey = legacyLeaseKey()
  const previousSnapshot = snapshot.value
  try {
    if (!claimLease(forceLease)) {
      const saved = restoreSnapshot(localStorage.getItem(storageKey.value), postSerial.value)
      if (saved) snapshot.value = saved
      readOnly.value = true
      return false
    }
    const session = await service.create(postSerial.value, selectedCount.value, requestController.signal)
    const created = createInitialSnapshot(session, writerId, leaseToken.value)
    snapshot.value = created
    readOnly.value = false
    networkPending.value = false
    if (!saveSnapshot()) throw new Error('local game state could not be saved')
    resultPreparationVersion += 1
    resultPreparing.value = false
    resultReady.value = false
    if (discardLegacyOnSuccess) {
      localStorage.removeItem(previousLegacyStateKey)
      if (previousLegacyLeaseKey) localStorage.removeItem(previousLegacyLeaseKey)
      legacyLeaseToken.value = 0
    }
    return true
  } catch (error) {
    snapshot.value = previousSnapshot
    if (previousSnapshot?.outbox.length && !previousSnapshot.cloud_sync_disabled && !readOnly.value) {
      scheduleSync(networkPending.value ? 5_000 : 0)
    }
    if (!(error instanceof DOMException && error.name === 'AbortError')) loadError.value = true
    return false
  } finally {
    creating.value = false
  }
}

/**
 * A click on a candidate.
 *
 * IN A MAJORITY ROOM IT IS A WAGER, NOT A VERDICT. The room decides the round, so the host's
 * click joins the tally and then waits like everybody else's; settling stays with the clock
 * and with 結束回合, which keeps one source of truth for who won. Everywhere else — a solo
 * game, or a room the host decides — the click IS the verdict and votes locally.
 */
function pickCandidate(elementId: number): void {
  if (!hostedRoom.majority.value) {
    voteFor(elementId)
    return
  }
  const pair = currentElements.value
  if (!pair || readOnly.value || animating.value) return
  void hostedRoom.placeBet(elementId, pair[0].id === elementId ? pair[1].id : pair[0].id)
}

function voteFor(winnerId: number): void {
  const game = snapshot.value
  const match = game?.current_match
  const pair = currentElements.value
  if (!game || !match || !pair || game.status !== 'playing' || readOnly.value || animating.value) return
  if (!renewOwnedLease()) {
    readOnly.value = true
    loadCanonicalSnapshot()
    return
  }
  const loserId = match.left_id === winnerId ? match.right_id : match.left_id
  const backup = JSON.stringify(game)
  animationPair.value = [{ ...pair[0] }, { ...pair[1] }]
  animationWinnerId.value = winnerId
  animating.value = true
  stopAllCandidateMedia()
  try {
    applyLocalVote(game, winnerId, loserId, `${game.game_serial}:${game.local_votes.length + 1}:${randomId()}`)
    if (!saveSnapshot()) {
      snapshot.value = restoreSnapshot(localStorage.getItem(storageKey.value), postSerial.value)
        ?? JSON.parse(backup) as LocalGameSnapshot
      readOnly.value = true
      resetVoteAnimation()
      return
    }
    if (game.outbox.length >= 10 || game.elements.filter((item) => !item.local_eliminated).length < 2) scheduleSync(0)
    else scheduleSync(syncDelay)
    const delay = window.matchMedia('(prefers-reduced-motion: reduce)').matches ? 80 : 1_100
    animationTimer = window.setTimeout(() => {
      resetVoteAnimation()
      if (snapshot.value?.status === 'completed') void prepareCompletedResult()
    }, delay)
  } catch {
    snapshot.value = JSON.parse(backup) as LocalGameSnapshot
    resetVoteAnimation()
  }
}

function resetVoteAnimation(): void {
  if (animationTimer) window.clearTimeout(animationTimer)
  animationTimer = undefined
  animationPair.value = null
  animationWinnerId.value = null
  animating.value = false
  hoveredVideoID.value = null
}

async function loadCommunityRanks(page = communityRanks.value.page): Promise<void> {
  if (communityLoading.value) return
  const requestVersion = ++communityRankRequestVersion
  const scrollPosition = page !== communityRanks.value.page && communityRanks.value.items.length
    ? captureRankingScrollPosition()
    : null
  communityLoading.value = true
  communityError.value = false
  try {
    const ranks = await publicContentService.ranks(postSerial.value, page, 20)
    if (requestVersion !== communityRankRequestVersion) return
    communityRanks.value = ranks
    const firstReport = communityRanks.value.items[0]
    if (resultTab.value === 'community' && firstReport) {
      void selectCommunityRank(firstReport)
    } else if (!firstReport) {
      selectedCommunityRank.value = null
      rankDetails.value = null
    }
  } catch {
    if (requestVersion === communityRankRequestVersion) communityError.value = true
  } finally {
    if (requestVersion === communityRankRequestVersion) {
      communityLoading.value = false
      if (scrollPosition) {
        await nextTick()
        restoreRankingScrollPosition(scrollPosition)
      }
    }
  }
}

interface RankingScrollPosition {
  shell: HTMLElement | null
  shellTop: number
  windowTop: number
}

function captureRankingScrollPosition(): RankingScrollPosition {
  const shell = document.querySelector<HTMLElement>('#main-content.game-page-shell')
  return {
    shell,
    shellTop: shell?.scrollTop ?? 0,
    windowTop: window.scrollY,
  }
}

function restoreRankingScrollPosition(position: RankingScrollPosition): void {
  if (position.shell?.isConnected) {
    position.shell.scrollTop = position.shellTop
    return
  }
  window.scrollTo({ top: position.windowTop, left: 0, behavior: 'instant' })
}

async function prepareCompletedResult(): Promise<void> {
  const gameSerial = snapshot.value?.game_serial
  if (!gameSerial || snapshot.value?.status !== 'completed' || rankOnly.value) return
  ++resultPreparationVersion
  resultPreparing.value = true
  resultReady.value = false
  // Keep the durable archive sync alive while this component is replaced. The
  // result route can still render the local archive if the network is offline.
  void syncArchivedResult()
  await router.replace({
    path: localizedPath(`/r/${encodeURIComponent(postSerial.value)}`, locale.value),
    query: { g: gameSerial },
  })
}

async function loadPersonalResultPage(): Promise<void> {
  const gameSerial = resultGameSerial.value
  if (!gameSerial) return
  const preparationVersion = ++resultPreparationVersion
  resultPreparing.value = true
  resultReady.value = false
  await Promise.all([
    new Promise<void>((resolve) => window.setTimeout(resolve, resultMinimumLoadingDuration)),
    (async () => {
      await syncArchivedResult()
      try {
        const result = await service.result(gameSerial)
        if (result.post_serial !== postSerial.value || result.game_serial !== gameSerial) throw new Error('game result mismatch')
        serverResult.value = result
      } catch {
        // The local archive remains authoritative when a final batch cannot be
        // synchronized. Shared devices simply fall back to the public ranking.
      }
      await loadCommunityRanks(1)
      const urls = Array.from(new Set([
        ...personalResultItems.value.map((item) => preferredGameImage(item.element)),
        ...communityRanks.value.items.slice(0, 8).map(preferredRankImage),
      ].filter((url): url is string => Boolean(url))))
      await Promise.all(urls.map(preloadResultImage))
    })(),
  ])
  if (preparationVersion !== resultPreparationVersion || resultGameSerial.value !== gameSerial) return
  resultPreparing.value = false
  resultReady.value = true
}

function resultShareURL(): string {
  const url = new URL(localizedPath(`/r/${encodeURIComponent(postSerial.value)}`, locale.value), window.location.origin)
  url.searchParams.set('g', resultGameSerial.value || serverResult.value?.game_serial || snapshot.value?.game_serial || '')
  return url.toString()
}

/**
 * Opens the room for the game in play, or reopens the invite for one already open.
 *
 * The pair on screen goes with the request: a host opens the room mid-game, and without it
 * the first participants are shown whatever match was last decided.
 */
/**
 * Opens the multiplayer dialog.
 *
 * A host who already has a room lands on the invite directly: there is one room per game,
 * so the mode was settled when it was opened and re-asking would suggest otherwise. The
 * step argument is how the gear beside the round clock gets straight to the settings,
 * rather than through the invite the host is not looking for.
 */
function openMultiplayerDialog(step?: 'settings'): void {
  if (step === 'settings' && hostedRoom.hosting.value) openRoundSettings()
  else multiplayerStep.value = hostedRoom.hosting.value ? 'invite' : 'mode'
  if (!multiplayerDialog.value?.open) {
    multiplayerDialog.value?.showModal()
    setModalOpen(true)
  }
  if (multiplayerStep.value === 'invite') void renderRoomQRCode()
}

function closeMultiplayerDialog(): void {
  multiplayerDialog.value?.close()
}

/**
 * Starts the guess-the-host's-preference mode, which is the only one built.
 *
 * Staying on the mode step when opening fails is deliberate: the error belongs next to the
 * card that was pressed, and an invite step with no link to show would be a dead end.
 */
async function chooseGuessPreferenceMode(): Promise<void> {
  await openGameRoom()
  if (!hostedRoom.hosting.value) return
  multiplayerStep.value = 'invite'
  await renderRoomQRCode()
}

/**
 * Starts the majority mode, which asks how rounds end before handing out the link.
 *
 * The room is opened first so the settings have something to be written to, and it opens in
 * the ordinary host-decides mode: a room whose link is not out yet has nobody to decide
 * anything, and the mode is written the moment the host confirms the settings.
 */
async function chooseMajorityMode(): Promise<void> {
  await openGameRoom()
  if (!hostedRoom.hosting.value) return
  openRoundSettings('invite')
}

/**
 * Shows the round settings, prefilled from what the room is running on now.
 *
 * `next` is where 確定 goes. The link belongs to setting the room up — first time through,
 * mode then settings then invite — and nowhere else: a host who opened the gear beside the
 * clock to change 20 秒 to 手動 asked to change one setting, not to be handed a link they
 * already sent out.
 */
function openRoundSettings(next: 'invite' | 'close' = 'close'): void {
  settingsNext.value = next
  const current = hostedRoom.voting.value
  roundTimed.value = !(current?.mode === 'majority' && current.round_seconds === 0)
  if (current && current.round_seconds > 0) roundSeconds.value = current.round_seconds
  votingError.value = false
  multiplayerStep.value = 'settings'
}

/**
 * Writes the settings, then either hands out the link or gets out of the way.
 *
 * The seconds are clamped rather than validated: the server refuses anything outside the
 * range, and a host who typed 3 meant "as short as it goes", not "fail".
 */
async function confirmRoundSettings(): Promise<void> {
  if (votingPending.value) return
  votingPending.value = true
  votingError.value = false
  const seconds = roundTimed.value
    ? Math.min(MAX_ROUND_SECONDS, Math.max(MIN_ROUND_SECONDS, Math.round(roundSeconds.value) || DEFAULT_ROUND_SECONDS))
    : 0
  try {
    await hostedRoom.setVoting('majority', seconds)
    roundSeconds.value = seconds || roundSeconds.value
    if (settingsNext.value === 'close') {
      closeMultiplayerDialog()
      return
    }
    multiplayerStep.value = 'invite'
    await renderRoomQRCode()
  } catch {
    votingError.value = true
  } finally {
    votingPending.value = false
  }
}

/**
 * Whether the room may decide the pairing on screen right now.
 *
 * Everything `voteFor` refuses is refused here too, and for the same reasons: the settled
 * round becomes a local vote like any other.
 */
const canSettleRound = computed(() => Boolean(
  hostedRoom.majority.value
  && snapshot.value?.status === 'playing'
  && currentElements.value
  && !readOnly.value
  && !animating.value
  && !settlingRound.value,
))

/**
 * Lets the room decide the round on screen.
 *
 * The winner comes from a fresh read of the tally, so the pairing is checked again once it
 * arrives: a round that settled while the request was in flight has already moved on, and
 * voting again would eliminate a candidate nobody was shown.
 */
async function settleRound(): Promise<void> {
  const pair = currentElements.value
  if (!canSettleRound.value || !pair) return
  settlingRound.value = true
  try {
    const winnerId = await hostedRoom.majorityWinner([pair[0].id, pair[1].id])
    const shown = currentElements.value
    if (!shown || shown[0].id !== pair[0].id || shown[1].id !== pair[1].id) return
    voteFor(winnerId)
  } finally {
    settlingRound.value = false
  }
}

// The countdown is the server's, but the bracket is this browser's, so the only place that
// can act on a round running out is here.
watch(() => hostedRoom.roundExpired.value, (expired) => {
  if (expired) void settleRound()
})

/**
 * Draws the invite code, after the step it lives on has been rendered.
 *
 * A failure is left silent on purpose: the link is on screen as text next to the code and a
 * copy button, so a missing QR image costs a phone user one paste and nothing else.
 */
async function renderRoomQRCode(): Promise<void> {
  roomQRReady.value = false
  await nextTick()
  const canvas = roomQRCanvas.value
  const url = hostedRoom.inviteURL.value
  if (!canvas || !url) return
  try {
    await drawQRCode(canvas, url)
    roomQRReady.value = true
  } catch {
    // Left unready: the canvas stays hidden and the link is still there to copy.
  }
}

/** This candidate's share of the wagers, as a whole percent. */
function voteShare(elementId: number): number {
  const counts = roundVotes.value
  if (!counts) return 0
  const total = [...counts.values()].reduce((sum, value) => sum + value, 0)
  if (total === 0) return 0
  return Math.round(((counts.get(elementId) ?? 0) / total) * 100)
}

function saveRoomQRCode(): void {
  const canvas = roomQRCanvas.value
  if (!canvas || !roomQRReady.value) return
  downloadQRCode(canvas, `2pick-room-${hostedRoom.serial.value || 'invite'}.png`)
}

async function openGameRoom(): Promise<void> {
  const displayed = currentElements.value
  await hostedRoom.open(displayed ? [displayed[0].id, displayed[1].id] : undefined)
}

async function copyRoomLink(): Promise<void> {
  const url = hostedRoom.inviteURL.value
  if (!url) return

  if (await shareOrCopyLink(url, definition.value?.title || '2Pick') !== 'copied') return
  roomLinkCopied.value = true
  if (roomLinkCopiedTimer) window.clearTimeout(roomLinkCopiedTimer)
  roomLinkCopiedTimer = window.setTimeout(() => { roomLinkCopied.value = false }, 2_000)
}

async function sharePersonalResult(): Promise<void> {
  await shareOrCopyLink(resultShareURL(), definition.value?.title || '2Pick')
}

// Shares the /g/<serial> short URL rather than the localized route, so the link
// stays short and resolves for a recipient in any language.
function postShareURL(): string {
  return new URL(`/g/${encodeURIComponent(postSerial.value)}`, window.location.origin).toString()
}

async function sharePost(): Promise<void> {
  if (await shareOrCopyLink(postShareURL(), definition.value?.title || '2Pick') !== 'copied') return
  shareCopied.value = true
  if (shareCopiedTimer) window.clearTimeout(shareCopiedTimer)
  shareCopiedTimer = window.setTimeout(() => { shareCopied.value = false }, 2_000)
}

function openPersonalResultExport(): void {
  rankingExportOpen.value = true
}

function preloadResultImage(url: string): Promise<void> {
  return new Promise((resolve) => {
    const image = new Image()
    let settled = false
    const timeout = window.setTimeout(finish, resultMediaTimeout)
    function finish(): void {
      if (settled) return
      settled = true
      window.clearTimeout(timeout)
      image.onload = null
      image.onerror = null
      resolve()
    }
    image.onload = finish
    image.onerror = finish
    image.src = url
    if (image.complete) finish()
  })
}

function selectResultTab(tab: 'mine' | 'community'): void {
  resultTab.value = tab
  if (tab === 'community' && !communityRanks.value.items.length && !communityError.value) {
    void loadCommunityRanks(1)
  } else if (tab === 'community' && !selectedCommunityRank.value) {
    const firstReport = communityRanks.value.items[0]
    if (firstReport) void selectCommunityRank(firstReport)
  }
}

function rankMediaEmbedURL(element: RankElement | LocalGameElement): string | null {
  return element.type === 'video' ? youtubeEmbedURL(element) : null
}

/**
 * Opens a ranked element at a size worth looking at.
 *
 * The row and the card both show a thumbnail sized for a list, and the ranking is
 * the one screen where the entry is the thing being ranked — so it has to open at
 * its own size rather than only ever be seen cropped into a small frame. A video
 * entry opens its player: a still frame of a video is not the entry.
 */
function openRankMedia(rank: number | null, element: RankElement | LocalGameElement): void {
  const embedURL = rankMediaEmbedURL(element)
  if (embedURL) {
    openedVideo.value = { embedURL, rank: rankLabel(rank), title: element.title || '' }
    videoDocked.value = false
    return
  }
  const image = fullSizeImage(element)
  if (!image) return
  openImageViewer({ image, title: [rankLabel(rank), element.title].filter(Boolean).join(' ') })
}

function dockVideo(): void {
  videoDocked.value = true
}

function expandVideo(): void {
  videoDocked.value = false
}

function closeVideo(): void {
  openedVideo.value = null
  videoDocked.value = false
}

async function selectCommunityRank(report: RankReport): Promise<void> {
  selectedCommunityRank.value = report
  stopRankVideo()
  trendLoading.value = true
  trendError.value = false
  const requestVersion = ++trendRequestVersion
  // The old chart stays up until the wait is long enough to admit to, and is
  // only then replaced by the loader; a fast read swaps one chart straight for
  // the next with nothing collapsing in between.
  if (trendLoaderTimer) window.clearTimeout(trendLoaderTimer)
  trendLoaderTimer = window.setTimeout(() => {
    if (requestVersion !== trendRequestVersion) return
    rankDetails.value = null
    trendLoaderVisible.value = true
  }, trendLoaderDelay)
  try {
    const details = await publicContentService.rank(
      postSerial.value,
      report.element.id,
      ['all'],
    )
    if (requestVersion === trendRequestVersion && selectedCommunityRank.value?.element.id === report.element.id) {
      rankDetails.value = details
    }
  } catch {
    if (requestVersion === trendRequestVersion) {
      rankDetails.value = null
      trendError.value = true
    }
  } finally {
    if (requestVersion === trendRequestVersion) {
      if (trendLoaderTimer) window.clearTimeout(trendLoaderTimer)
      trendLoaderTimer = undefined
      trendLoaderVisible.value = false
      trendLoading.value = false
    }
  }
}

async function syncVotes(): Promise<void> {
  const game = snapshot.value
  if (!game || syncing.value || readOnly.value || game.cloud_sync_disabled || game.outbox.length === 0) return
  if (!renewOwnedLease()) {
    readOnly.value = true
    loadCanonicalSnapshot()
    return
  }
  const batch = game.outbox.slice(0, 128)
  game.in_flight = batch
  game.updated_at = Date.now()
  if (!saveSnapshot()) {
    readOnly.value = true
    loadCanonicalSnapshot()
    return
  }
  syncing.value = true
  try {
    // While hosting a room, tell the server which pair is now on screen. games.candidates
    // means exactly that, and this client picks its own pairs locally, so the server has no
    // other way to know — without it the room shows its participants the match just decided.
    // Only sent when this batch empties the outbox; see onScreenPairForBatch.
    //
    // The batch that finishes the game has no pair on screen any more, and sends the final
    // two in the order they were shown instead: that order is what the home page's champion
    // rail places the finalists by, and only this client knows it. See finalDisplayedPair.
    const onScreen = hostedRoom.hosting.value
      ? onScreenPairForBatch(batch.length, game.outbox.length, currentElements.value)
      : undefined
    const finalists = batch.length === game.outbox.length
      ? finalDisplayedPair(game) ?? undefined
      : undefined

    const result = await service.submitVotes(
      game.game_serial,
      game.server_vote_count,
      batch.map(({ winner_id, loser_id }) => ({ winner_id, loser_id })),
      getAnonymousID(),
      requestController.signal,
      onScreen ?? finalists,
    )
    if (!renewOwnedLease() || snapshot.value?.game_serial !== game.game_serial) {
      readOnly.value = true
      loadCanonicalSnapshot()
      return
    }
    const acceptedIds = new Set(batch.map((vote) => vote.local_vote_id))
    snapshot.value.outbox = snapshot.value.outbox.filter((vote) => !acceptedIds.has(vote.local_vote_id))
    snapshot.value.in_flight = null
    snapshot.value.server_vote_count = result.server_vote_count
    snapshot.value.updated_at = Date.now()
    networkPending.value = false
    saveSnapshot()
  } catch (error) {
    if (error instanceof DOMException && error.name === 'AbortError') return
    if (snapshot.value !== game || !renewOwnedLease()) {
      readOnly.value = true
      loadCanonicalSnapshot()
      return
    }
    if (isPermanentSyncFailure(error)) {
      game.cloud_sync_disabled = true
      game.cloud_sync_reason = error instanceof APIError ? error.code : 'server_rejected_branch'
      game.in_flight = null
      networkPending.value = false
      saveSnapshot()
    } else {
      // The outbox and in-flight marker deliberately remain durable. A
      // refresh or later retry sends the exact same revision-safe prefix.
      networkPending.value = true
      saveSnapshot()
    }
  } finally {
    syncing.value = false
    if (snapshot.value?.outbox.length && !snapshot.value.cloud_sync_disabled && !readOnly.value) {
      scheduleSync(networkPending.value ? 5_000 : 0)
    }
  }
}

function showRestartDialog(): void {
  if (!snapshot.value || creating.value) return
  selectedCount.value = snapshot.value.selected_count
  restartError.value = false
  if (!restartDialog.value?.open) {
    restartDialog.value?.showModal()
    setModalOpen(true)
  }
}

function openRestartDialog(): void {
  entryDecisionPending.value = false
  showRestartDialog()
}

function closeRestartDialog(): void {
  cancelRestartHold()
  restartDialog.value?.close()
}

function continueGame(): void {
  const shouldResume = entryDecisionPending.value
  entryDecisionPending.value = false
  if (shouldResume && snapshot.value) {
    resumeSnapshot(snapshot.value)
    if (snapshot.value.status === 'completed') void prepareCompletedResult()
  }
  closeRestartDialog()
}

function dismissRestartDialog(): void {
  if (entryDecisionPending.value) continueGame()
  else closeRestartDialog()
}

function beginRestartHold(event?: Event): void {
  event?.preventDefault()
  if (creating.value || syncing.value || restartHoldActive.value) return
  restartError.value = false
  restartHoldActive.value = true
  restartHoldTimer = window.setTimeout(() => {
    restartHoldTimer = undefined
    void restartGame().finally(() => { restartHoldActive.value = false })
  }, 1_000)
}

function cancelRestartHold(event?: Event): void {
  event?.preventDefault()
  if (!restartHoldTimer) return
  window.clearTimeout(restartHoldTimer)
  restartHoldTimer = undefined
  restartHoldActive.value = false
}

function onRestartKeydown(event: KeyboardEvent): void {
  if (!['Enter', ' '].includes(event.key) || event.repeat) return
  beginRestartHold(event)
}

function onRestartKeyup(event: KeyboardEvent): void {
  if (['Enter', ' '].includes(event.key)) cancelRestartHold(event)
}

async function restartGame(): Promise<void> {
  if (creating.value || syncing.value) return
  restartError.value = false
  if (syncTimer) window.clearTimeout(syncTimer)
  syncTimer = undefined
  const created = await createNewGame(true, true)
  if (!created) {
    restartError.value = true
    return
  }
  resetVoteAnimation()
  resultTab.value = 'mine'
  communityRanks.value = { items: [], page: 1, per_page: 20, total: 0, total_pages: 0 }
  selectedCommunityRank.value = null
  rankDetails.value = null
  closeImageViewer()
  closeVideo()
  entryDecisionPending.value = false
  resultPreparationVersion += 1
  resultPreparing.value = false
  resultReady.value = false
  closeRestartDialog()
  if (rankOnly.value && snapshot.value?.game_serial) {
    sessionStorage.setItem(autoResumeKey.value, snapshot.value.game_serial)
    await router.replace({ path: localizedPath(`/g/${encodeURIComponent(postSerial.value)}`, locale.value) })
  }
}

function takeOver(): void {
  if (!claimLease(true)) return
  const saved = readSavedSnapshot()
  if (!saved) return
  if (!saved.post_title) saved.post_title = definition.value?.title ?? ''
  saved.writer_id = writerId
  saved.lease_token = leaseToken.value
  if (saved.status === 'playing') {
    saved.current_match = null
    chooseNextMatch(saved, Math.random, true)
  }
  saved.revision += 1
  saved.updated_at = Date.now()
  snapshot.value = saved
  readOnly.value = false
  saveSnapshot()
  if (saved.outbox.length) scheduleSync(0)
}

function claimLease(force: boolean): boolean {
  const existing = readLease()
  if (!force && existing && existing.owner_id !== writerId && existing.expires_at > Date.now()) return false
  const legacyLease = readLegacyLease()
  if (!force && legacyLease && legacyLease.ownerId !== writerId && legacyLease.expiresAt > Date.now()) return false
  const token = randomId()
  const lease: GameLease = {
    owner_id: writerId,
    token,
    game_serial: snapshot.value?.game_serial ?? '',
    expires_at: Date.now() + leaseDuration,
  }
  localStorage.setItem(leaseKey.value, JSON.stringify(lease))
  const verified = readLease()
  if (verified?.owner_id !== writerId || verified.token !== token) return false
  leaseToken.value = token
  if (hasMatchingLegacySnapshot()) {
    const fencingToken = Math.max(Number(legacyLease?.fencingToken || 0), legacyLeaseToken.value) + 1
    legacyLeaseToken.value = fencingToken
    localStorage.setItem(legacyLeaseKey(), JSON.stringify({
      schemaVersion: 1,
      gameSerial: snapshot.value?.game_serial,
      ownerId: writerId,
      fencingToken,
      heartbeatAt: Date.now(),
      expiresAt: Date.now() + leaseDuration,
    }))
  }
  startHeartbeat()
  return true
}

function renewOwnedLease(): boolean {
  const lease = readLease()
  if (!lease || lease.owner_id !== writerId || lease.token !== leaseToken.value) return false
  lease.expires_at = Date.now() + leaseDuration
  lease.game_serial = snapshot.value?.game_serial ?? lease.game_serial
  localStorage.setItem(leaseKey.value, JSON.stringify(lease))
  if (hasMatchingLegacySnapshot()) {
    const legacyLease = readLegacyLease()
    if (!legacyLease || legacyLease.ownerId !== writerId
      || Number(legacyLease.fencingToken) !== legacyLeaseToken.value) {
      localStorage.removeItem(leaseKey.value)
      return false
    }
    legacyLease.heartbeatAt = Date.now()
    legacyLease.expiresAt = Date.now() + leaseDuration
    localStorage.setItem(legacyLeaseKey(), JSON.stringify(legacyLease))
  }
  return true
}

function startHeartbeat(): void {
  if (heartbeatTimer) window.clearInterval(heartbeatTimer)
  heartbeatTimer = window.setInterval(() => {
    if (!renewOwnedLease()) {
      readOnly.value = true
      if (heartbeatTimer) window.clearInterval(heartbeatTimer)
    }
  }, leaseHeartbeat)
}

function releaseLease(): void {
  const lease = readLease()
  if (lease?.owner_id === writerId && lease.token === leaseToken.value) {
    localStorage.removeItem(leaseKey.value)
  }
  const legacyLease = readLegacyLease()
  if (legacyLease?.ownerId === writerId && Number(legacyLease.fencingToken) === legacyLeaseToken.value) {
    localStorage.removeItem(legacyLeaseKey())
  }
}

function readLease(): GameLease | null {
  try {
    const lease = JSON.parse(localStorage.getItem(leaseKey.value) || 'null') as GameLease | null
    return lease && typeof lease.owner_id === 'string' && typeof lease.expires_at === 'number' ? lease : null
  } catch {
    return null
  }
}

function legacyLeaseKey(): string {
  return `gamelease_${postSerial.value}_${snapshot.value?.game_serial || ''}`
}

function readLegacyLease(): Record<string, any> | null {
  if (!snapshot.value?.game_serial) return null
  try {
    const lease = JSON.parse(localStorage.getItem(legacyLeaseKey()) || 'null') as Record<string, any> | null
    return lease && lease.gameSerial === snapshot.value.game_serial ? lease : null
  } catch {
    return null
  }
}

function hasMatchingLegacySnapshot(): boolean {
  const legacy = restoreLegacySnapshot(localStorage.getItem(legacyStateKey.value), postSerial.value)
  return Boolean(legacy && legacy.game_serial === snapshot.value?.game_serial)
}

function readSavedSnapshot(): LocalGameSnapshot | null {
  let current = restoreSnapshot(localStorage.getItem(storageKey.value), postSerial.value)
  let legacy = restoreLegacySnapshot(localStorage.getItem(legacyStateKey.value), postSerial.value)
  if (current?.status === 'completed') {
    archiveCompletedResult(current)
    localStorage.removeItem(storageKey.value)
    current = null
  }
  if (legacy?.status === 'completed') {
    archiveCompletedResult(legacy)
    localStorage.removeItem(legacyStateKey.value)
    legacy = null
  }
  if (!current) return legacy
  if (!legacy) return current
  if (current.game_serial === legacy.game_serial && legacy.updated_at > current.updated_at) return legacy
  return current
}

function saveSnapshot(): boolean {
  const game = snapshot.value
  if (!game || !renewOwnedLease()) return false
  const canonical = restoreSnapshot(localStorage.getItem(storageKey.value), postSerial.value)
  // Revisions are only comparable within the same game. A deliberate restart
  // begins at revision 1 with a new serial and must be allowed to replace the
  // higher-revision snapshot from the previous game.
  if (canonical && canonical.game_serial === game.game_serial
    && canonical.revision > game.revision && canonical.lease_token !== leaseToken.value) return false
  game.writer_id = writerId
  game.lease_token = leaseToken.value
  game.updated_at = Date.now()
  try {
    if (game.status === 'completed') {
      archiveCompletedResult(game)
      localStorage.removeItem(storageKey.value)
      localStorage.removeItem(legacyStateKey.value)
    } else {
      localStorage.setItem(storageKey.value, JSON.stringify(game))
    }
    return true
  } catch {
    return false
  }
}

function archiveCompletedResult(game: LocalGameSnapshot): void {
  const current = restoreSnapshot(localStorage.getItem(resultStorageKey.value), postSerial.value)
  if (current && current.updated_at > game.updated_at) return
  localStorage.setItem(resultStorageKey.value, JSON.stringify(game))
}

async function syncArchivedResult(): Promise<void> {
  const archived = restoreSnapshot(localStorage.getItem(resultStorageKey.value), postSerial.value)
  if (!archived || archived.status !== 'completed' || archived.cloud_sync_disabled || !archived.outbox.length) return
  const batch = archived.outbox.slice(0, 128)
  try {
    const result = await service.submitVotes(
      archived.game_serial,
      archived.server_vote_count,
      batch.map(({ winner_id, loser_id }) => ({ winner_id, loser_id })),
      getAnonymousID(),
    )
    const latest = restoreSnapshot(localStorage.getItem(resultStorageKey.value), postSerial.value)
    if (!latest || latest.game_serial !== archived.game_serial) return
    const acceptedIDs = new Set(batch.map((vote) => vote.local_vote_id))
    latest.outbox = latest.outbox.filter((vote) => !acceptedIDs.has(vote.local_vote_id))
    latest.in_flight = null
    latest.server_vote_count = result.server_vote_count
    latest.updated_at = Date.now()
    localStorage.setItem(resultStorageKey.value, JSON.stringify(latest))
    if (latest.outbox.length) void syncArchivedResult()
  } catch (error) {
    if (error instanceof DOMException && error.name === 'AbortError') return
    if (!isPermanentSyncFailure(error)) return
    const latest = restoreSnapshot(localStorage.getItem(resultStorageKey.value), postSerial.value)
    if (!latest || latest.game_serial !== archived.game_serial) return
    latest.cloud_sync_disabled = true
    latest.cloud_sync_reason = error instanceof APIError ? error.code : 'server_rejected_branch'
    latest.in_flight = null
    latest.updated_at = Date.now()
    localStorage.setItem(resultStorageKey.value, JSON.stringify(latest))
  }
}

function loadCanonicalSnapshot(): void {
  const saved = readSavedSnapshot()
  if (saved) snapshot.value = saved
}

function onStorage(event: StorageEvent): void {
  if (event.key === leaseKey.value) {
    const lease = readLease()
    if (lease && (lease.owner_id !== writerId || lease.token !== leaseToken.value)) readOnly.value = true
    return
  }
  if (event.key === legacyLeaseKey()) {
    const lease = readLegacyLease()
    if (lease && (lease.ownerId !== writerId || Number(lease.fencingToken) !== legacyLeaseToken.value)) {
      readOnly.value = true
    }
    return
  }
  if (event.key === legacyStateKey.value && event.newValue) {
    const incoming = restoreLegacySnapshot(event.newValue, postSerial.value)
    const lease = readLegacyLease()
    if (incoming && lease?.ownerId !== writerId) {
      incoming.post_title = definition.value?.title ?? ''
      snapshot.value = incoming
      readOnly.value = true
    }
    return
  }
  if (event.key !== storageKey.value || !event.newValue) return
  const incoming = restoreSnapshot(event.newValue, postSerial.value)
  if (!incoming) return
  const lease = readLease()
  if (lease?.owner_id !== writerId || lease.token !== leaseToken.value) {
    snapshot.value = incoming
    readOnly.value = true
  }
}

/**
 * Escape, for the docked video overlay.
 *
 * The game itself has no keyboard controls. The arrow-key and 1/2 shortcuts were removed
 * along with the help panel that documented them: a vote is a tap on a picture, and the
 * shortcuts made the page listen to every key press to serve a path almost nobody took.
 */
function onKeydown(event: KeyboardEvent): void {
  if (event.key !== 'Escape' || !openedVideo.value || videoDocked.value) return
  // Docked, not stopped: Escape leaves the big view, it does not end playback.
  dockVideo()
}

function scheduleSync(delay: number): void {
  if (syncTimer) window.clearTimeout(syncTimer)
  syncTimer = window.setTimeout(() => { void syncVotes() }, delay)
}

function isPermanentSyncFailure(error: unknown): boolean {
  if (!(error instanceof APIError)) return false
  return error.status === 409 || (error.status >= 400 && error.status < 500
    && ![408, 425, 429].includes(error.status))
}

function randomId(): string {
  return globalThis.crypto?.randomUUID?.() ?? `${Date.now()}-${Math.random().toString(36).slice(2)}`
}

/**
 * The blurred fill behind a contained picture, so the frame is never empty at the
 * sides of a portrait shot. Only a still image gets one: an embedded player and a
 * video both fill their own frame.
 */
function candidateBackdrop(element: LocalGameElement): string | null {
  if (youtubeEmbedURL(element) || (element.type === 'video' && element.source_url)) return null
  return imageBackdrop(preferredGameImage(element))
}

/** A `background-image` value for the blurred copy behind a contained picture. */
function imageBackdrop(image: string | null): string | null {
  return image ? `url("${image.replace(/"/g, '%22')}")` : null
}

function isPlayableVideo(element: LocalGameElement): boolean {
  return element.type === 'video' && Boolean(youtubeEmbedURL(element) || element.source_url)
}

function onCandidateVideoEnter(element: LocalGameElement): void {
  if (!isPlayableVideo(element) || animating.value
    || !window.matchMedia('(hover: hover) and (pointer: fine)').matches) return
  hoveredVideoID.value = element.id
  if (videoHoverTimer) window.clearTimeout(videoHoverTimer)
  videoHoverTimer = window.setTimeout(() => {
    if (hoveredVideoID.value !== element.id || animating.value) return
    visibleElements.value?.forEach((candidate) => {
      if (candidate.id === element.id) playCandidateMedia(candidate, true)
      else pauseCandidateMedia(candidate)
    })
  }, 250)
}

function onCandidateVideoLeave(element: LocalGameElement): void {
  if (hoveredVideoID.value !== element.id) return
  hoveredVideoID.value = null
  if (videoHoverTimer) window.clearTimeout(videoHoverTimer)
  videoHoverTimer = undefined
}

function candidateMediaNode(elementID: number): HTMLVideoElement | HTMLIFrameElement | null {
  return document.querySelector<HTMLVideoElement | HTMLIFrameElement>(`[data-game-media-id="${elementID}"]`)
}

function playCandidateMedia(element: LocalGameElement, audible: boolean): void {
  const media = candidateMediaNode(element.id)
  if (media instanceof HTMLVideoElement) {
    media.muted = !audible
    void media.play().catch(() => undefined)
    return
  }
  if (media instanceof HTMLIFrameElement) {
    postYouTubeCommand(media, audible ? 'unMute' : 'mute')
    postYouTubeCommand(media, 'playVideo')
  }
}

function pauseCandidateMedia(element: LocalGameElement): void {
  const media = candidateMediaNode(element.id)
  if (media instanceof HTMLVideoElement) {
    media.pause()
    media.muted = true
    return
  }
  if (media instanceof HTMLIFrameElement) {
    postYouTubeCommand(media, 'pauseVideo')
    postYouTubeCommand(media, 'mute')
  }
}

function stopAllCandidateMedia(): void {
  visibleElements.value?.forEach(pauseCandidateMedia)
}

/**
 * Puts the pairing's media back the way the markup starts it: playing, muted, looping.
 *
 * Silent on purpose. Sound is only ever the host's hover, and a modal closing is not a
 * hover.
 */
function resumeCandidateMedia(): void {
  hoveredVideoID.value = null
  visibleElements.value?.forEach((element) => playCandidateMedia(element, false))
}

/**
 * A dialog covering the board pauses the video behind it.
 *
 * The pairing's video autoplays and loops, so a host who reloads into the continue-or-restart
 * question is left with motion behind a modal they have to read, and no way to stop it — the
 * controls are under the backdrop. Every modal here covers the pairing, so they all do this.
 */
function setModalOpen(open: boolean): void {
  if (open) {
    stopAllCandidateMedia()
    return
  }
  resumeCandidateMedia()
}

function postYouTubeCommand(frame: HTMLIFrameElement, command: 'playVideo' | 'pauseVideo' | 'mute' | 'unMute'): void {
  frame.contentWindow?.postMessage(JSON.stringify({ event: 'command', func: command, args: [] }), '*')
}

function historyImage(element: LocalGameElement | null, fallback?: string): string | null {
  return (element ? gamePreviewImage(element) : null) || fallback || null
}

function preferredRankImage(report: RankReport): string | null {
  const element = report.element
  return element.lowthumb_url || element.mediumthumb_url || element.thumb_url || element.source_url
}
</script>

<template>
  <template v-if="loading">
    <!-- A ranking page is a ranking: it opens on the shape of one rather than on a
         waiting box a sixth of its height, so nothing the reader is looking at moves
         when the rows arrive. A game has no such shape to promise yet. -->
    <section v-if="rankOnly" class="game-ranking" role="status" :aria-label="t('gameLoading')">
      <header class="game-public-ranking-heading" aria-hidden="true">
        <div>
          <p class="eyebrow">2PICK · RANKING</p>
          <!-- A width of its own: this heading is sized by its title, so a share
               of it is a share of nothing. -->
          <h1><span class="skeleton-line" style="width: 16rem" /></h1>
        </div>
        <div class="game-public-ranking-actions">
          <span class="button skeleton" style="width: 7rem" />
        </div>
      </header>
      <RankingSkeleton />
    </section>
    <section v-else class="game-state-card">{{ t('gameLoading') }}</section>
  </template>
  <!-- The door code. Shown instead of the error card, because a 404 on a protected post
       and a 404 on a wrong link are the same answer from the server. -->
  <section v-else-if="doorCodeRequired" class="game-state-card game-door-code">
    <h1>{{ t('gameDoorCodeTitle') }}</h1>
    <p class="game-door-code-hint">{{ t('gameDoorCodeHint') }}</p>
    <form class="game-door-code-form" @submit.prevent="submitDoorCode">
      <label class="game-door-code-label" for="game-door-code">{{ t('gameDoorCodeLabel') }}</label>
      <input
        id="game-door-code"
        v-model="doorCode"
        type="password"
        autocomplete="off"
        :disabled="unlocking"
        class="game-door-code-input"
      />
      <p v-if="doorCodeError" class="game-door-code-error" role="alert">{{ t(doorCodeError) }}</p>
      <button class="button button-primary" type="submit" :disabled="unlocking || !doorCode.trim()">
        {{ unlocking ? t('gameDoorCodeChecking') : t('gameDoorCodeSubmit') }}
      </button>
    </form>
  </section>

  <section v-else-if="!definition || (loadError && !snapshot)" class="game-state-card game-error-state">
    <p>{{ t('gameLoadError') }}</p>
    <button class="button button-primary" type="button" @click="$router.go(0)">{{ t('retry') }}</button>
  </section>

  <!-- The ranking of an 18+ post is locked outright: unlike the game page there is no
       preview of it to show, so the whole card is the prompt. -->
  <section v-else-if="rankOnly && signInRequired" class="game-state-card game-sign-in-required">
    <h1>{{ definition.title }}</h1>
    <p class="game-sign-in-hint">{{ t('gameRankSignInRequiredHint') }}</p>
    <div class="game-sign-in-actions">
      <RouterLink class="button button-primary" :to="signInTarget">{{ t('login') }}</RouterLink>
      <RouterLink class="button button-quiet" :to="localizedPath('/', locale)">{{ t('gameBackHome') }}</RouterLink>
    </div>
  </section>

  <section v-else-if="!snapshot && !rankOnly" class="game-setup">
    <div class="game-setup-intro">
      <p class="eyebrow">2PICK · NEW GAME</p>
      <h1>{{ definition.title }}</h1>
      <p v-if="definition.description" class="game-description">{{ definition.description }}</p>

      <!-- Preview of the two options, rendered from the definition response so
           it is on screen before the player presses start. -->
      <div v-if="previewOptions.length" class="game-preview">
        <figure v-for="option in previewOptions" :key="option.key" class="game-preview-card">
          <div class="game-preview-media" :class="{ 'is-censored': censored }">
            <div class="game-preview-backdrop" :style="{ backgroundImage: option.backdrop }"></div>
            <img
              v-if="option.isImage"
              :src="option.image"
              :alt="option.title"
              decoding="async"
              @error="onPreviewImageError($event, option)"
            >
            <video v-else :src="`${option.image}#t=1`" muted playsinline preload="metadata"></video>
          </div>
          <figcaption class="game-preview-title">{{ option.title }}</figcaption>
        </figure>
      </div>
    </div>

    <div class="game-setup-panel">
      <p>{{ t('gameAvailable', { count: definition.elements_count }) }}</p>
      <h2>{{ signInRequired ? t('gameSignInRequiredTitle') : t('gameChooseCount') }}</h2>
      <p v-if="signInRequired" class="game-sign-in-hint">{{ t('gameSignInRequiredHint') }}</p>
      <div v-else class="game-count-options">
        <button
          v-for="count in countOptions"
          :key="count"
          type="button"
          :class="{ active: selectedCount === count }"
          @click="selectedCount = count"
        >{{ count }}</button>
      </div>
      <div class="game-setup-actions">
        <RouterLink
          v-if="signInRequired"
          class="button button-primary game-start-button"
          :to="signInTarget"
        >{{ t('login') }}</RouterLink>
        <button v-else class="button button-primary game-start-button" type="button" :disabled="creating" @click="startGame">
          {{ creating ? t('gameCreating') : t('gameStart') }}
        </button>
        <RouterLink
          v-if="!signInRequired"
          class="button button-quiet"
          :to="localizedPath(`/r/${encodeURIComponent(postSerial)}`, locale)"
        >
          {{ t('viewRanking') }}
        </RouterLink>
        <button class="button button-quiet game-share-button" type="button" @click="sharePost">
          <svg viewBox="0 0 24 24" aria-hidden="true">
            <circle cx="18" cy="5" r="3" /><circle cx="6" cy="12" r="3" /><circle cx="18" cy="19" r="3" />
            <path d="m8.6 10.6 6.8-4M8.6 13.4l6.8 4" />
          </svg>
          {{ shareCopied ? t('gameShareCopied') : t('gameShare') }}
        </button>
      </div>
    </div>
  </section>

  <section
    v-else
    :key="snapshot?.game_serial || 'game'"
    class="game-page"
    :class="{ 'is-playing': snapshot?.status === 'playing' || animating }"
  >
    <template v-if="snapshot && visibleElements && (snapshot.status === 'playing' || animating)">
      <header class="game-heading">
        <h1 :title="snapshot.post_title">{{ snapshot.post_title }}</h1>
        <div class="game-stats">
          <span class="game-stage-stat" :title="t('gameProgress', {
            current: snapshot.current_match?.current_round ?? 1,
            total: snapshot.current_match?.of_round ?? 1,
          })">
            <svg viewBox="0 0 24 24" aria-hidden="true"><path d="M7 4l10 16M17 4L7 20M5 5l2-2 2 2M15 19l2 2 2-2" /></svg>
            <b class="game-round-title">{{ roundTitle }}</b>
            <small>{{ snapshot.current_match?.current_round ?? 1 }}/{{ snapshot.current_match?.of_round ?? 1 }}</small>
          </span>
          <span :title="t('gameRemain', { count: activeCount })">
            <svg viewBox="0 0 24 24" aria-hidden="true"><path d="M12 3v4M8 5h8M7 9h10l-1 12H8L7 9Z" /><path d="M10 13v4M14 13v4" /></svg>
            <b>{{ activeCount }}</b>
          </span>
          <!-- The actions are grouped so a phone can move them to their own row under the
               title and its pills: side by side they are wider than the screen. -->
          <div class="game-controls">
            <!-- The take-over control stays because it is actionable. The former
                 sync-status icon was removed: it only described background
                 syncing, which is not the player's concern. -->
            <button
              v-if="readOnly"
              class="game-sync-indicator is-readonly"
              type="button"
              :title="t('gameTakeOver')"
              :aria-label="t('gameTakeOver')"
              @click="takeOver"
            >
              <svg viewBox="0 0 24 24" aria-hidden="true"><rect x="4" y="10" width="12" height="10" rx="2" /><path d="M7 10V7a3 3 0 0 1 6 0v3M16 7h5M19 4l3 3-3 3" /></svg>
            </button>
            <!-- Hosting a room. Hidden once the game is over: a finished game has no round for
                 anyone to wager on, so opening a room then would produce an empty one. -->
            <button
              v-if="snapshot?.status !== 'completed'"
              class="game-controls-toggle"
              type="button"
              :class="{ 'is-hosting': hostedRoom.hosting.value }"
              :disabled="hostedRoom.status.value === 'opening'"
              :title="hostedRoom.hosting.value ? t('roomInviteTitle') : t('roomHostStart')"
              :aria-label="hostedRoom.hosting.value ? t('roomInviteTitle') : t('roomHostStart')"
              @click="openMultiplayerDialog()"
            >
              <svg viewBox="0 0 24 24" aria-hidden="true">
                <circle cx="9" cy="8" r="3" /><circle cx="17" cy="9" r="2.5" />
                <path d="M3.5 19a5.5 5.5 0 0 1 11 0M15 19a4 4 0 0 1 5.5-3.7" />
              </svg>
            </button>
            <button
              class="game-controls-toggle game-share-control"
              type="button"
              :title="shareCopied ? t('gameShareCopied') : t('gameShare')"
              :aria-label="shareCopied ? t('gameShareCopied') : t('gameShare')"
              @click="sharePost"
            >
              <svg viewBox="0 0 24 24" aria-hidden="true">
                <circle cx="18" cy="5" r="3" /><circle cx="6" cy="12" r="3" /><circle cx="18" cy="19" r="3" />
                <path d="m8.6 10.6 6.8-4M8.6 13.4l6.8 4" />
              </svg>
            </button>
            <button
              class="game-controls-toggle"
              type="button"
              :disabled="creating"
              :title="t('gameRefresh')"
              :aria-label="t('gameRefresh')"
              @click="openRestartDialog"
            >
              <svg viewBox="0 0 24 24" aria-hidden="true"><path d="M4 4v6h6M5.6 15a8 8 0 1 0 .5-7.5L4 10" /></svg>
            </button>
          </div>
        </div>
      </header>

      <div class="game-layout" :class="{ 'has-room': hostedRoom.hosting.value }">
        <div class="game-arena" :class="{ disabled: readOnly }">
          <div
            v-for="(element, index) in visibleElements"
            :key="element.id"
            class="game-candidate-panel"
            :class="{
              'game-candidate-left': index === 0,
              'game-candidate-right': index === 1,
              'game-candidate-winner': animating && animationWinnerId === element.id,
              'game-candidate-loser': animating && animationWinnerId !== element.id,
            }"
          >
            <article class="game-candidate">
              <div
                class="game-candidate-media"
                :class="{ 'is-video': isPlayableVideo(element) }"
                @mouseenter="onCandidateVideoEnter(element)"
                @mouseleave="onCandidateVideoLeave(element)"
              >
                <div
                  v-if="candidateBackdrop(element)"
                  class="game-candidate-backdrop"
                  :style="{ backgroundImage: candidateBackdrop(element) || '' }"
                  aria-hidden="true"
                ></div>
                <iframe
                  v-if="youtubeEmbedURL(element)"
                  :src="youtubeEmbedURL(element) || ''"
                  :title="element.title"
                  allow="autoplay; encrypted-media; picture-in-picture"
                  loading="lazy"
                  :data-game-media-id="element.id"
                />
                <video
                  v-else-if="element.type === 'video' && element.source_url"
                  :src="element.source_url"
                  :data-game-media-id="element.id"
                  autoplay muted loop playsinline controls preload="metadata"
                />
                <img v-else-if="preferredGameImage(element)" :src="preferredGameImage(element) || ''" :alt="element.title" draggable="false">
                <div v-else class="game-media-fallback">{{ index + 1 }}</div>
              </div>
              <h2>{{ element.title }}</h2>
              <!-- What the room has wagered on this pairing: behind the black box in a
                   room the host decides, always on where the room decides for itself. -->
              <p v-if="hostedRoom.showVotes.value" class="game-candidate-bets">
                <b>{{ roundVotes ? roundVotes.get(element.id) ?? 0 : '—' }}</b>
                <span v-if="roundVotes">{{ voteShare(element.id) }}%</span>
              </p>
              <!-- The host votes too. In a majority room the click is a wager and the
                   round still ends on the clock; see pickCandidate. -->
              <button
                class="game-vote-button"
                type="button"
                :class="{ 'is-picked': hostedRoom.ownPick.value === element.id }"
                :disabled="readOnly || animating"
                :aria-pressed="hostedRoom.majority.value ? hostedRoom.ownPick.value === element.id : undefined"
                :aria-label="t('gameVoteFor', { title: element.title })"
                @click="pickCandidate(element.id)"
              >
                <svg viewBox="0 0 24 24" aria-hidden="true"><path d="M7.5 10.5v10H4a2 2 0 0 1-2-2v-6a2 2 0 0 1 2-2h3.5Zm0 0 4.2-7.1a1.8 1.8 0 0 1 3.3 1.2v4.1h4.2a2.8 2.8 0 0 1 2.7 3.5l-1.7 6.2a2.8 2.8 0 0 1-2.7 2.1h-10" /></svg>
              </button>
            </article>
          </div>
          <span class="game-versus" aria-hidden="true">VS</span>
        </div>

        <aside class="game-history">
          <div class="game-history-heading">
            <h2>{{ t('gameHistory') }}</h2>
          </div>
          <p v-if="!snapshot.match_history.length">{{ t('gameNoHistory') }}</p>
          <ol v-else>
            <li
              v-for="item in historyItems"
              :key="item.vote_id"
              class="game-history-card"
              :class="{ 'left-win': item.winner_side === 'left', 'right-win': item.winner_side === 'right' }"
            >
              <figure class="game-history-pick game-history-winner">
                <img v-if="historyImage(item.winner, item.winner_thumb)" :src="historyImage(item.winner, item.winner_thumb) || ''" :alt="item.winner_title" loading="lazy">
                <div v-else class="game-history-placeholder">{{ item.winner_title.slice(0, 1) }}</div>
                <figcaption>
                  <svg viewBox="0 0 24 24" aria-hidden="true"><path d="M8 4h8v4a4 4 0 0 1-8 0V4ZM6 6H3v1a4 4 0 0 0 5 4M18 6h3v1a4 4 0 0 1-5 4M12 12v4M8 20h8M9 16h6" /></svg>
                  <strong>{{ item.winner_title }}</strong>
                  <em v-if="item.roomRound?.your_pick === item.winner_id" class="game-history-yours">{{ t('roomHistoryYourPick') }}</em>
                </figcaption>
              </figure>
              <figure class="game-history-pick game-history-loser">
                <img v-if="historyImage(item.loser, item.loser_thumb)" :src="historyImage(item.loser, item.loser_thumb) || ''" :alt="item.loser_title" loading="lazy">
                <div v-else class="game-history-placeholder">{{ item.loser_title.slice(0, 1) }}</div>
                <figcaption>
                  <svg viewBox="0 0 24 24" aria-hidden="true"><path d="M6 6l12 12M18 6 6 18" /></svg>
                  <span>{{ item.loser_title }}</span>
                  <em v-if="item.roomRound?.your_pick === item.loser_id" class="game-history-yours">{{ t('roomHistoryYourPick') }}</em>
                </figcaption>
              </figure>
              <!-- How the room voted on this match. Absent for a match played before the
                   room opened, and for a room that has no votes to report. -->
              <p v-if="item.roomRound" class="game-history-votes">
                <b>{{ item.roomRound.winner_votes }}</b>
                <span>{{ t('roomHistoryShare', { share: roundShare(item.roomRound) }) }}</span>
                <b class="game-history-votes-loser">{{ item.roomRound.loser_votes }}</b>
              </p>
            </li>
          </ol>
        </aside>

        <!-- The host's window into their own room. They never enter it — this is how they
             see who is in there and how the standings are moving while they set matchups. -->
        <aside v-if="hostedRoom.hosting.value" class="game-room-panel">
          <div class="game-room-panel-head">
            <h2>{{ t('gameRoom') }}</h2>
            <!-- Honest about all three states: a room on its poll is working, just not
                 instant, and saying so beats implying it is broken. -->
            <p class="game-room-live" :data-state="hostedRoom.live.value">
              <span class="game-room-live-dot" aria-hidden="true"></span>
              {{ t(hostedRoom.live.value === 'connected' ? 'roomLive' : 'roomPolling') }}
            </p>
          </div>

          <p class="game-room-players">
            <span>{{ t('roomPlayers') }}</span>
            <b>{{ hostedRoom.players.value }}</b>
          </p>

          <!-- The round clock, and the way out of it. The button is offered on a timed
               round too: cutting one short is useful whether or not one was going to end
               on its own. -->
          <div v-if="hostedRoom.majority.value" class="game-room-round">
            <p class="game-room-round-clock">
              <span>{{ hostedRoom.secondsLeft.value === null ? t('roomRoundManual') : t('roomRoundRemaining') }}</span>
              <b v-if="hostedRoom.secondsLeft.value !== null">{{ hostedRoom.secondsLeft.value }}</b>
            </p>
            <button
              type="button"
              class="game-room-round-settle"
              :disabled="!canSettleRound"
              @click="settleRound"
            >{{ t('roomRoundSettle') }}</button>
            <!-- Beside the clock it changes, rather than buried in the invite dialog the
                 host only opens once. -->
            <button
              type="button"
              class="game-room-round-settings-button"
              :title="t('roomRoundSettings')"
              :aria-label="t('roomRoundSettings')"
              @click="openMultiplayerDialog('settings')"
            >
              <svg viewBox="0 0 24 24" aria-hidden="true">
                <circle cx="12" cy="12" r="3" />
                <path d="M19.4 15a1.7 1.7 0 0 0 .3 1.9l.1.1a2 2 0 1 1-2.8 2.8l-.1-.1a1.7 1.7 0 0 0-1.9-.3 1.7 1.7 0 0 0-1 1.5V21a2 2 0 1 1-4 0v-.1a1.7 1.7 0 0 0-1.1-1.5 1.7 1.7 0 0 0-1.9.3l-.1.1a2 2 0 1 1-2.8-2.8l.1-.1a1.7 1.7 0 0 0 .3-1.9 1.7 1.7 0 0 0-1.5-1H3a2 2 0 1 1 0-4h.1a1.7 1.7 0 0 0 1.5-1.1 1.7 1.7 0 0 0-.3-1.9l-.1-.1a2 2 0 1 1 2.8-2.8l.1.1a1.7 1.7 0 0 0 1.9.3H9a1.7 1.7 0 0 0 1-1.5V3a2 2 0 1 1 4 0v.1a1.7 1.7 0 0 0 1 1.5 1.7 1.7 0 0 0 1.9-.3l.1-.1a2 2 0 1 1 2.8 2.8l-.1.1a1.7 1.7 0 0 0-.3 1.9V9a1.7 1.7 0 0 0 1.5 1H21a2 2 0 1 1 0 4h-.1a1.7 1.7 0 0 0-1.5 1Z" />
              </svg>
            </button>
          </div>

          <div class="game-room-panel-actions">
            <!-- A host-mode device: it hides the counts from the person running a
                 猜喜好 game. A majority room shows them to everybody by design. -->
            <button
              v-if="!hostedRoom.majority.value"
              type="button"
              class="game-room-panel-button"
              :class="{ 'is-on': hostedRoom.blackBox.value }"
              :aria-pressed="hostedRoom.blackBox.value"
              @click="hostedRoom.toggleBlackBox()"
            >
              <svg viewBox="0 0 24 24" aria-hidden="true">
                <path d="M4 8h16v11H4zM4 8l2-3h12l2 3M12 5v14" />
              </svg>
              {{ t(hostedRoom.blackBox.value ? 'roomBlackBoxClose' : 'roomBlackBoxOpen') }}
            </button>
            <button type="button" class="game-room-panel-button" @click="openMultiplayerDialog()">
              <svg viewBox="0 0 24 24" aria-hidden="true">
                <path d="M10 13a5 5 0 0 0 7 0l3-3a5 5 0 0 0-7-7l-1 1M14 11a5 5 0 0 0-7 0l-3 3a5 5 0 0 0 7 7l1-1" />
              </svg>
              {{ t('roomInviteShow') }}
            </button>
          </div>

          <template v-for="board in roomBoards" :key="board.key">
            <h3>{{ t(board.title) }}</h3>
            <ol v-if="board.rows.length" class="game-room-board">
              <li v-for="row in board.rows" :key="row.user_id">
                <span class="game-room-board-rank">{{ row.rank || '—' }}</span>
                <span class="game-room-board-name">{{ row.name }}</span>
                <span class="game-room-board-score">{{ row.score }}</span>
              </li>
            </ol>
            <p v-else class="game-room-board-empty">{{ t('roomNoPlayers') }}</p>
          </template>
        </aside>

        <!-- Always rendered, unlike the room panel it used to take turns with: on a narrow
             screen the history is capped and the rail fits beside it even while hosting. Which
             widths show it is a layout question, so CSS answers it. -->
        <aside class="game-ad-slot" :aria-label="t('advertisement')">
          <div><span>AD</span></div>
        </aside>
      </div>
    </template>

    <template v-else-if="(rankOnly && resultGameSerial && !resultReady) || (snapshot?.status === 'completed' && winner && !resultReady)">
      <section
        class="game-result-loading"
        role="status"
        aria-live="polite"
        :aria-busy="resultPreparing"
      >
        <div class="game-result-loading-hero">
          <div class="game-result-loading-media" aria-hidden="true">
            <div class="game-result-loading-duel">
              <span></span>
              <b>VS</b>
              <span></span>
            </div>
          </div>
          <div class="game-result-loading-copy">
            <p class="eyebrow">2PICK · RESULT</p>
            <h1>{{ t('gamePreparingRanking') }}</h1>
            <p>{{ t('gamePreparingRankingHint') }}</p>
            <div class="game-result-loading-progress" aria-hidden="true"><span></span></div>
          </div>
        </div>
        <div class="game-result-loading-ranking" aria-hidden="true">
          <span v-for="index in 4" :key="index"></span>
        </div>
      </section>
    </template>

    <template v-else-if="snapshot?.status === 'completed' && winner">
      <div class="game-result-hero game-result-revealed">
        <div class="game-result-media">
          <img v-if="preferredGameImage(winner)" :src="preferredGameImage(winner) || ''" :alt="winner.title">
          <div v-else class="game-media-fallback">1</div>
        </div>
        <div>
          <p class="eyebrow">{{ t('gameWinner') }}</p>
          <h1>{{ winner.title }}</h1>
          <div class="hero-actions">
            <button class="button button-primary" type="button" @click="openRestartDialog">{{ t('gameRestart') }}</button>
            <RouterLink class="button button-quiet" :to="localizedPath('/', locale)">{{ t('gameBackHome') }}</RouterLink>
          </div>
        </div>
      </div>
      <AdSlot v-if="adsAllowed" name="gameResult" shape="horizontal" :locale="locale" />
    </template>

    <section
      v-if="(rankOnly && (!resultGameSerial || resultReady)) || (snapshot?.status === 'completed' && resultReady)"
      class="game-ranking"
      :class="{ 'game-result-revealed': !rankOnly && resultReady }"
    >
      <header v-if="rankOnly" class="game-public-ranking-heading">
        <div>
          <p class="eyebrow">2PICK · RANKING</p>
          <h1>{{ definition.title }}</h1>
        </div>
      <div class="game-public-ranking-actions">
        <button v-if="hasPersonalResult" class="button button-quiet game-share-result" type="button" @click="sharePersonalResult">
          {{ t('gameShareResult') }}
        </button>
        <button v-if="hasPersonalResult" class="button button-quiet game-export-result" type="button" @click="openPersonalResultExport">
          {{ t('gameDownloadResult') }}
        </button>
        <RouterLink class="button button-primary" :to="localizedPath(`/g/${encodeURIComponent(postSerial)}`, locale)">
          {{ t('gameStart') }}
        </RouterLink>
      </div>
      </header>
    <div v-if="hasPersonalResult" class="game-result-tabs" role="tablist" aria-label="Ranking">
          <button type="button" role="tab" :aria-selected="resultTab === 'mine'" :class="{ active: resultTab === 'mine' }" @click="selectResultTab('mine')">
            <svg viewBox="0 0 24 24" aria-hidden="true"><circle cx="12" cy="8" r="4" /><path d="M4 21a8 8 0 0 1 16 0" /></svg>
            {{ t('gameMyRanking') }}
          </button>
          <button type="button" role="tab" :aria-selected="resultTab === 'community'" :class="{ active: resultTab === 'community' }" @click="selectResultTab('community')">
            <svg viewBox="0 0 24 24" aria-hidden="true"><path d="M4 20v-7M10 20V8M16 20V4M22 20H2" /></svg>
            {{ t('gameCommunityRanking') }}
          </button>
      </div>
    <div v-if="hasPersonalResult && resultTab === 'mine'" class="game-personal-ranking">
      <div v-if="personalPodiumItems.length" class="game-personal-podium">
        <article v-for="item in personalPodiumItems" :key="item.element.id" class="game-personal-hero">
          <span class="game-personal-rank">{{ item.rank }}</span>
          <button
            class="game-personal-media"
            type="button"
            :disabled="!fullSizeImage(item.element)"
            :aria-label="t('gameZoomRankImage', { title: item.element.title })"
            @click="openRankMedia(item.rank, item.element)"
          >
            <div
              v-if="imageBackdrop(preferredGameImage(item.element))"
              class="game-personal-backdrop"
              :style="{ backgroundImage: imageBackdrop(preferredGameImage(item.element)) || '' }"
              aria-hidden="true"
            ></div>
            <img v-if="preferredGameImage(item.element)" :src="preferredGameImage(item.element) || ''" :alt="item.element.title" loading="lazy">
          </button>
          <div class="game-personal-meta">
            <strong>{{ item.element.title }}</strong>
            <small>{{ t('gameGlobalRank', { rank: rankLabel(item.global_rank) }) }}</small>
          </div>
        </article>
      </div>
      <ol v-if="personalRestItems.length" class="game-personal-rest">
        <li v-for="item in personalRestItems" :key="item.element.id">
          <span>{{ item.rank }}</span>
          <button
            class="game-personal-media"
            type="button"
            :disabled="!fullSizeImage(item.element)"
            :aria-label="t('gameZoomRankImage', { title: item.element.title })"
            @click="openRankMedia(item.rank, item.element)"
          >
            <div
              v-if="imageBackdrop(preferredGameImage(item.element))"
              class="game-personal-backdrop"
              :style="{ backgroundImage: imageBackdrop(preferredGameImage(item.element)) || '' }"
              aria-hidden="true"
            ></div>
            <img v-if="preferredGameImage(item.element)" :src="preferredGameImage(item.element) || ''" :alt="item.element.title" loading="lazy">
          </button>
          <div>
            <strong>{{ item.element.title }}</strong>
            <small>{{ t('gameGlobalRank', { rank: rankLabel(item.global_rank) }) }}</small>
          </div>
        </li>
      </ol>
    </div>
      <div v-else class="game-community-ranking" role="tabpanel">
          <template v-if="communityLoading && !communityRanks.items.length">
            <RankingSkeleton />
            <span class="sr-only" role="status">{{ t('gameCommunityLoading') }}</span>
          </template>
          <div v-else-if="communityError && !communityRanks.items.length" class="game-ranking-state">
            <p>{{ t('gameCommunityError') }}</p>
            <button class="button button-quiet" type="button" @click="loadCommunityRanks(communityRanks.page)">{{ t('retry') }}</button>
          </div>
          <p v-else-if="!communityRanks.items.length" class="game-ranking-state">{{ t('gameCommunityEmpty') }}</p>
          <div
            v-else
            class="game-community-layout"
            :class="{ 'is-updating': communityLoading }"
            :aria-busy="communityLoading"
          >
            <section class="game-rank-trend">
              <header v-if="selectedCommunityRank">
                <div>
                  <!-- Dropped along with the chart rather than announced: a label
                       for something that is not there reads as a fault. -->
                  <span v-if="selectedRankIsCharted">{{ t('gameRankTrend') }}</span>
                  <h2>{{ selectedCommunityRank.element.title }}</h2>
                </div>
                <!-- The cumulative standing is the ranking; the thousand-vote
                     one lives on its own row in the list below, so repeating it
                     here as a second box read as a tab nobody could switch. -->
                <dl class="game-selected-group-ranks">
                  <div>
                    <dt>{{ t('gameCumulativeRanking') }}</dt>
                    <dd>
                      <strong>{{ rankLabel(cumulativeSelectedRank?.rank) }}</strong>
                      <small v-if="positiveRank(cumulativeSelectedRank?.rank)">{{ t('gameWinRate', { rate: cumulativeSelectedRank?.win_rate ?? '' }) }}</small>
                    </dd>
                    <dd v-if="positiveRank(cumulativeSelectedRank?.rank)" class="game-winrate-bar" aria-hidden="true">
                      <span :style="{ width: winRateBarWidth(cumulativeSelectedRank?.win_rate) }"></span>
                    </dd>
                  </div>
                </dl>
              </header>
              <!-- The selected element at a size worth looking at. Contained
                   rather than cropped, because a ranking picture is the thing
                   being ranked, and clicking it opens the full image. -->
              <button
                v-if="selectedCommunityRank
                  && !rankVideoEmbedURL(selectedCommunityRank)
                  && preferredRankImage(selectedCommunityRank)"
                class="game-rank-figure"
                type="button"
                :aria-label="t('gameZoomRankImage', { title: selectedCommunityRank.element.title || '' })"
                @click="openRankMedia(selectedCommunityRank.rank, selectedCommunityRank.element)"
              >
                <img
                  :src="preferredRankImage(selectedCommunityRank) || ''"
                  :alt="selectedCommunityRank.element.title || ''"
                >
                <span class="game-rank-zoom-hint" aria-hidden="true">
                  <svg viewBox="0 0 24 24">
                    <circle cx="11" cy="11" r="7" /><path d="M20 20l-4.4-4.4M11 8v6M8 11h6" />
                  </svg>
                </span>
              </button>
              <!-- Video entries preview as a still and only load the player once
                   the viewer asks for it. Depends on the selected row alone, so it
                   survives a details reload without the card resizing. -->
              <figure
                v-if="selectedCommunityRank && rankVideoEmbedURL(selectedCommunityRank)"
                class="game-rank-video"
              >
                <iframe
                  v-if="playingRankVideoID === selectedCommunityRank.element.id"
                  :src="rankVideoEmbedURL(selectedCommunityRank) || ''"
                  :title="selectedCommunityRank.element.title || ''"
                  allow="autoplay; encrypted-media; picture-in-picture"
                  allowfullscreen
                />
                <button
                  v-else
                  type="button"
                  :aria-label="t('gamePlayRankVideo', { title: selectedCommunityRank.element.title || '' })"
                  @click="playRankVideo(selectedCommunityRank)"
                >
                  <img
                    v-if="preferredRankImage(selectedCommunityRank)"
                    :src="preferredRankImage(selectedCommunityRank) || ''"
                    :alt="selectedCommunityRank.element.title || ''"
                  >
                  <span class="game-rank-video-play" aria-hidden="true">
                    <svg viewBox="0 0 24 24"><path d="m8 5 11 7-11 7V5Z" /></svg>
                  </span>
                </button>
              </figure>

              <!-- One chain for the chart slot. Splitting the loading states from
                   the chart states rendered two 240px placeholders at once and
                   swung the card 243px on every pagination click. -->
              <template v-if="selectedRankIsCharted">
                <div v-if="trendLoaderVisible" class="game-trend-slot game-trend-state" aria-busy="true">
                <span class="game-trend-loader" aria-hidden="true"></span>
                <span class="sr-only">{{ t('gameTrendLoading') }}</span>
                </div>
                <div v-else-if="trendError" class="game-trend-slot game-trend-state">
                <span>{{ t('gameTrendError') }}</span>
                <button v-if="selectedCommunityRank" type="button" @click="selectCommunityRank(selectedCommunityRank)">{{ t('retry') }}</button>
                </div>
                <p v-else-if="!trendCoordinates.length && !trendLoading" class="game-trend-slot game-trend-state">{{ t('gameTrendEmpty') }}</p>
                <div v-else-if="trendCoordinates.length" class="game-trend-slot game-trend-chart-wrap">
                <svg class="game-trend-chart" :viewBox="`0 0 ${trendChart.width} ${trendChart.height}`" role="img" :aria-label="t('gameRankTrend')">
                  <line v-for="y in trendChart.gridlines" :key="y" :x1="trendChart.paddingX" :y1="y" :x2="trendChart.width - trendChart.paddingX" :y2="y" />
                  <!-- Fixed scale, so the labels are constants and every chart on
                       the page can be compared against the next. -->
                  <text x="6" :y="trendChart.paddingY + 6">#{{ rankTrendDomain.best }}</text>
                  <text x="6" :y="trendChart.height - trendChart.paddingY + 6">{{ trendWorstLabel }}</text>
                  <polyline :points="trendPolyline" />
                  <circle
                    v-for="point in trendCoordinates"
                    :key="`${point.date}-${point.rank}`"
                    :cx="point.x"
                    :cy="point.y"
                    :r="trendPointRadius"
                    :class="{ 'is-clamped': point.clamped }"
                  >
                    <title>{{ point.date }} · #{{ point.rank }} · {{ t('gameWinRate', { rate: point.win_rate }) }}</title>
                  </circle>
                </svg>
                <div class="game-trend-dates"><span>{{ trendFirstDate }}</span><span>{{ trendLastDate }}</span></div>
                </div>
                <!-- A read too quick for the loader and with nothing to draw yet:
                     the slot holds its height and says nothing, rather than
                     flashing "no data" at a chart that is one frame away. -->
                <div v-else class="game-trend-slot" aria-hidden="true"></div>
              </template>
            </section>

            <ol class="game-community-list">
              <template v-for="(report, index) in communityRanks.items" :key="report.element.id">
              <li :class="{ active: selectedCommunityRank?.element.id === report.element.id }">
                <div class="game-community-row">
                  <span class="game-community-position">{{ positiveRank(report.rank) ?? ((communityRanks.page - 1) * communityRanks.per_page + index + 1) }}</span>
                  <!-- The picture is its own control: the row selects the element,
                       the thumbnail opens the picture at full size. -->
                  <button
                    class="game-community-thumb"
                    type="button"
                    :disabled="!rankVideoEmbedURL(report) && !fullSizeImage(report.element)"
                    :aria-label="t('gameZoomRankImage', { title: report.element.title || '' })"
                    @click="openRankMedia(report.rank, report.element)"
                  >
                    <img v-if="preferredRankImage(report)" :src="preferredRankImage(report) || ''" :alt="report.element.title || ''" loading="lazy">
                    <!-- Marks the row as holding a video: clicking it opens the
                         player rather than a still frame. -->
                    <span v-if="rankVideoEmbedURL(report)" class="game-community-thumb-video" aria-hidden="true">
                      <svg viewBox="0 0 24 24"><path d="m8 5 11 7-11 7V5Z" /></svg>
                    </span>
                  </button>
                  <button class="game-community-open" type="button" @click="selectCommunityRank(report)">
                    <span class="game-community-title">
                      <strong>{{ report.element.title }}</strong>
                      <small>{{ t('gameWinRate', { rate: report.win_rate }) }}</small>
                      <!-- The same number as a length: a column of bars is what
                           makes 85% and 61% comparable at a glance. -->
                      <span class="game-winrate-bar" aria-hidden="true">
                        <span :style="{ width: winRateBarWidth(report.win_rate) }"></span>
                      </span>
                    </span>
                  </button>
                </div>
              </li>
              <li v-if="adsAllowed && index === rankAdAfterRow" class="game-community-ad">
                <AdSlot name="rankList" shape="horizontal" :locale="locale" />
              </li>
              </template>
            </ol>
          </div>
          <span v-if="communityLoading && communityRanks.items.length" class="game-ranking-update-indicator" role="status">
            <span class="sr-only">{{ t('gameCommunityLoading') }}</span>
          </span>
          <nav v-if="communityRanks.total_pages > 1" class="game-ranking-pagination" :aria-label="t('gameCommunityRanking')">
            <button type="button" :disabled="communityLoading || communityRanks.page <= 1" @click="loadCommunityRanks(communityRanks.page - 1)">{{ t('previousPage') }}</button>
            <span>{{ communityRanks.page }} / {{ communityRanks.total_pages }}</span>
            <button type="button" :disabled="communityLoading || communityRanks.page >= communityRanks.total_pages" @click="loadCommunityRanks(communityRanks.page + 1)">{{ t('nextPage') }}</button>
          </nav>
      </div>

      <!-- One player in two positions. Leaving the big view docks it bottom left
           and it keeps playing; only its close button ends playback. -->
      <div
        v-if="openedVideo"
        class="game-rank-player"
        :class="{ 'is-docked': videoDocked }"
        role="dialog"
        :aria-label="openedVideo.title"
        @click.self="dockVideo"
      >
        <div class="game-rank-player-actions">
          <button
            v-if="videoDocked"
            class="game-rank-player-expand"
            type="button"
            :aria-label="t('gameExpandRankVideo')"
            @click="expandVideo"
          >
            <svg viewBox="0 0 24 24" aria-hidden="true"><path d="M14 4h6v6M20 4l-7 7M10 20H4v-6M4 20l7-7" /></svg>
          </button>
          <button
            v-else
            class="game-rank-player-dock"
            type="button"
            :aria-label="t('gameDockRankVideo')"
            @click="dockVideo"
          >
            <svg viewBox="0 0 24 24" aria-hidden="true"><path d="M4 10h6V4M10 10 3 3M20 14h-6v6M14 14l7 7" /></svg>
          </button>
          <button class="game-rank-player-close" type="button" :aria-label="t('close')" @click="closeVideo">
            <svg viewBox="0 0 24 24" aria-hidden="true"><path d="M6 6l12 12M18 6L6 18" /></svg>
          </button>
        </div>
        <div class="game-rank-player-frame">
          <iframe
            :src="openedVideo.embedURL"
            :title="openedVideo.title"
            allow="autoplay; encrypted-media; picture-in-picture"
            allowfullscreen
          />
        </div>
        <p v-if="!videoDocked" class="game-rank-player-caption">
          <strong>{{ openedVideo.rank }}</strong>
          <span>{{ openedVideo.title }}</span>
        </p>
      </div>
    </section>

    <CommentSection
      v-if="rankOnly || (snapshot?.status === 'completed' && resultReady)"
      :post-serial="postSerial"
      :locale="locale"
      :local-champions="localChampionLabels"
    />
  </section>

  <RankingExportDialog
    :open="rankingExportOpen"
    :title="definition?.title || '2Pick'"
    :items="rankingExportItems"
    :locale="locale"
    @close="rankingExportOpen = false"
  />

  <!-- Multiplayer. Two steps: pick the mode, then hand out the link. The host stays on
       their own game throughout — there is nothing for them inside the room, and walking
       in would leave the game they are hosting unattended. -->
  <dialog
    ref="multiplayerDialog"
    class="game-multiplayer-dialog"
    aria-labelledby="game-multiplayer-title"
    @cancel.prevent="closeMultiplayerDialog"
    @close="setModalOpen(false)"
  >
    <form method="dialog" @submit.prevent>
      <header>
        <div>
          <p class="eyebrow">2PICK · {{ t('gameRoom') }}</p>
          <h2 id="game-multiplayer-title">
            {{ multiplayerStep === 'invite'
              ? t('roomInviteFriends')
              : multiplayerStep === 'settings' ? t('roomRoundSettings') : t('roomChooseMode') }}
          </h2>
        </div>
        <button type="button" :aria-label="t('close')" @click="closeMultiplayerDialog">×</button>
      </header>

      <template v-if="multiplayerStep === 'mode'">
        <div class="game-mode-options">
          <button
            class="game-mode-card"
            type="button"
            :disabled="hostedRoom.status.value === 'opening'"
            @click="chooseGuessPreferenceMode"
          >
            <svg viewBox="0 0 24 24" aria-hidden="true"><path d="M12 20s-7-4.6-7-9.4A3.9 3.9 0 0 1 12 8a3.9 3.9 0 0 1 7 2.6c0 4.8-7 9.4-7 9.4Z" /></svg>
            <strong>{{ t('roomModePreference') }}</strong>
            <p>{{ t('roomModePreferenceDescription') }}</p>
            <ul>
              <li>{{ t('roomModePreferenceLeaderboard') }}</li>
              <li>{{ t('roomModeBlackBox') }}</li>
              <li>{{ t('roomModePoints') }}</li>
              <li>{{ t('roomModeCombo') }}</li>
            </ul>
          </button>
          <button
            class="game-mode-card"
            type="button"
            :disabled="hostedRoom.status.value === 'opening'"
            @click="chooseMajorityMode"
          >
            <svg viewBox="0 0 24 24" aria-hidden="true">
              <circle cx="9" cy="8" r="3" /><circle cx="17" cy="9" r="2.5" />
              <path d="M3.5 19a5.5 5.5 0 0 1 11 0M15 19a4 4 0 0 1 5.5-3.7" />
            </svg>
            <strong>{{ t('roomModeMajority') }}</strong>
            <p>{{ t('roomModeMajorityDescription') }}</p>
            <ul>
              <li>{{ t('roomModeMajorityTimer') }}</li>
              <li>{{ t('roomBoardPopular') }}</li>
              <li>{{ t('roomBoardUnique') }}</li>
              <li>{{ t('roomModeMajorityShare') }}</li>
            </ul>
          </button>
        </div>
        <p
          v-if="hostedRoom.status.value === 'failed'"
          class="game-room-invite-error"
          role="alert"
        >{{ t('roomHostFailed') }}</p>
      </template>

      <section v-else-if="multiplayerStep === 'settings'" class="game-room-round-settings">
        <p class="game-room-invite-hint">{{ t('roomRoundSettingsHint') }}</p>
        <div class="game-room-round-modes">
          <button
            type="button"
            class="game-room-round-mode"
            :class="{ 'is-on': roundTimed }"
            :aria-pressed="roundTimed"
            @click="roundTimed = true"
          >
            <strong>{{ t('roomRoundTimed') }}</strong>
            <span>{{ t('roomRoundTimedDescription') }}</span>
          </button>
          <button
            type="button"
            class="game-room-round-mode"
            :class="{ 'is-on': !roundTimed }"
            :aria-pressed="!roundTimed"
            @click="roundTimed = false"
          >
            <strong>{{ t('roomRoundManualMode') }}</strong>
            <span>{{ t('roomRoundManualDescription') }}</span>
          </button>
        </div>
        <label v-if="roundTimed" class="game-room-round-seconds">
          <span>{{ t('roomRoundSeconds') }}</span>
          <input
            v-model.number="roundSeconds"
            type="number"
            inputmode="numeric"
            :min="MIN_ROUND_SECONDS"
            :max="MAX_ROUND_SECONDS"
            step="1"
          >
        </label>
        <p v-if="votingError" class="game-room-invite-error" role="alert">{{ t('roomRoundSettingsFailed') }}</p>
        <div class="game-room-invite-actions">
          <button
            class="button button-primary"
            type="button"
            :disabled="votingPending"
            @click="confirmRoundSettings"
          >{{ t('roomRoundConfirm') }}</button>
        </div>
      </section>

      <section v-else class="game-room-invite">
        <p class="game-room-invite-host">{{ t('roomHostYou') }}</p>
        <p class="game-room-invite-hint">{{ t('roomInviteHint') }}</p>
        <!-- Hidden until a code is drawn, so a failed encode leaves no empty white square
             behind. The link below is the fallback either way. -->
        <canvas
          v-show="roomQRReady"
          ref="roomQRCanvas"
          class="game-room-invite-qr"
          :aria-label="t('roomInviteTitle')"
          role="img"
        ></canvas>
        <!-- The link is shown as text as well as copied: clipboard access is refused often
             enough that a copy button alone leaves people stuck. -->
        <p class="game-room-invite-url">{{ hostedRoom.inviteURL.value }}</p>
        <div class="game-room-invite-actions">
          <button class="button button-primary" type="button" @click="copyRoomLink">
            {{ roomLinkCopied ? t('roomInviteCopied') : t('roomInviteCopy') }}
          </button>
          <button
            v-if="roomQRReady"
            class="button button-ghost"
            type="button"
            @click="saveRoomQRCode"
          >
            {{ t('roomQrDownload') }}
          </button>
        </div>
      </section>
    </form>
  </dialog>

  <dialog
    ref="restartDialog"
    class="game-restart-dialog"
    aria-labelledby="game-restart-title"
    @cancel.prevent="dismissRestartDialog"
    @close="setModalOpen(false)"
  >
    <form method="dialog" @submit.prevent>
      <header>
        <div>
          <p class="eyebrow">2PICK · GAME</p>
        <h2 id="game-restart-title">{{ restartDialogTitle }}</h2>
        </div>
        <button type="button" :aria-label="t('close')" @click="dismissRestartDialog">×</button>
      </header>

    <button v-if="canContinueCurrentGame" class="game-continue-option" type="button" @click="continueGame">
        <svg viewBox="0 0 24 24" aria-hidden="true"><path d="m8 5 11 7-11 7V5Z" /></svg>
        <span><strong>{{ t('gameContinue') }}</strong><small>{{ snapshot?.post_title }}</small></span>
      </button>

    <RouterLink
      v-if="canContinueCurrentGame"
        class="game-dialog-ranking-link"
        :to="localizedPath(`/r/${encodeURIComponent(postSerial)}`, locale)"
        @click="closeRestartDialog"
      >
        <svg viewBox="0 0 24 24" aria-hidden="true"><path d="M4 20v-7M10 20V8M16 20V4M22 20H2" /></svg>
        <span>{{ t('viewRanking') }}</span>
      </RouterLink>

      <section>
        <h3>{{ t('gameNewRound') }}</h3>
        <p>{{ t('gameChooseCount') }}</p>
        <div class="game-count-options">
          <button
            v-for="count in countOptions"
            :key="count"
            type="button"
            :class="{ active: selectedCount === count }"
            @click="selectedCount = count"
          >{{ count }}</button>
        </div>
        <small v-if="canContinueCurrentGame" id="game-restart-hold-hint">{{ t('gameHoldRestartHint') }}</small>
        <p v-if="restartError" class="game-restart-error" role="alert">{{ t('gameRestartError') }}</p>
        <button
      v-if="canContinueCurrentGame"
          class="button button-primary game-restart-confirm"
          :class="{ 'is-holding': restartHoldActive }"
          type="button"
          :disabled="creating || syncing"
          aria-describedby="game-restart-hold-hint"
          @click.prevent
          @pointerdown="beginRestartHold"
          @pointerup="cancelRestartHold"
          @pointerleave="cancelRestartHold"
          @pointercancel="cancelRestartHold"
          @keydown="onRestartKeydown"
          @keyup="onRestartKeyup"
          @blur="cancelRestartHold"
          @contextmenu.prevent
        ><span>{{ creating ? t('gameCreating') : t('gameHoldRestart') }}</span></button>
    <button
      v-else
      class="button button-primary game-restart-confirm"
      type="button"
      :disabled="creating || syncing"
      @click="restartGame"
    ><span>{{ creating ? t('gameCreating') : t('gameNewRound') }}</span></button>
      </section>
    </form>
  </dialog>
</template>

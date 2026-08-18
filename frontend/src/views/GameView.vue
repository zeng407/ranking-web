<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'

import AdSlot from '../components/AdSlot.vue'
import CommentSection from '../components/CommentSection.vue'
import RankingExportDialog from '../components/RankingExportDialog.vue'
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
import { unlockPost } from '../services/postAccess'
import { onScreenPairForBatch, useHostedRoom } from '../composables/useHostedRoom'
import { getAnonymousID } from '../lib/anonymousId'
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
 * Whether this visitor has to sign in before the page will do anything.
 *
 * An 18+ post stays previewable — it is listed on the home page, and its two blurred
 * thumbnails show here — but playing it, voting on it and reading its ranking need an
 * account. The server enforces all three with a 401; this only decides what to render, and
 * waits for `authLoading` so a signed-in visitor never sees the prompt flash past.
 */
const signInRequired = computed(() => censored.value && !authLoading.value && !authenticated.value)
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
const zoomedPicture = ref<{ image: string; rank: string; title: string } | null>(null)
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
const controlsOpen = ref(false)
const restartDialog = ref<HTMLDialogElement | null>(null)
const rankingExportOpen = ref(false)
const restartError = ref(false)
const entryDecisionPending = ref(false)
const restartHoldActive = ref(false)
const resultPreparing = ref(false)
const resultReady = ref(false)
const shareCopied = ref(false)
const roomLinkCopied = ref(false)
let roomLinkCopiedTimer: number | undefined
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
const visibleElements = computed(() => animationPair.value ?? currentElements.value)

/**
 * The game room this host has opened, if any.
 *
 * Keyed on the game serial rather than the post: a restart is a new game and therefore a new
 * room, and an invite link to the old one must not silently follow the host into it.
 */
const hostedRoom = useHostedRoom(
  computed(() => snapshot.value?.game_serial || ''),
  computed(() => localeDefinition(locale.value).prefix),
)
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
const historyItems = computed(() => {
  const game = snapshot.value
  if (!game) return []
  return game.match_history.map((item) => ({
    ...item,
    winner: game.elements.find((element) => element.id === item.winner_id) ?? null,
    loser: game.elements.find((element) => element.id === item.loser_id) ?? null,
  }))
})

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
    if (definition.value.is_censored) {
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
  if (saved.status === 'playing') {
    // The current pair has not been voted on, so a reload may choose any
    // still-ready candidates from this stage without changing progress.
    saved.current_match = null
    chooseNextMatch(saved, Math.random, true)
    saved.revision += 1
    saved.updated_at = Date.now()
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
async function openGameRoom(): Promise<void> {
	const displayed = currentElements.value
	await hostedRoom.open(displayed ? [displayed[0].id, displayed[1].id] : undefined)
}

async function copyRoomLink(): Promise<void> {
	const url = hostedRoom.inviteURL.value
	if (!url) return

	if (navigator.share) {
		try {
			await navigator.share({ title: definition.value?.title || '2Pick', url })
			return
		} catch (error) {
			if (error instanceof DOMException && error.name === 'AbortError') return
		}
	}

	try {
		await navigator.clipboard.writeText(url)
		roomLinkCopied.value = true
		if (roomLinkCopiedTimer) window.clearTimeout(roomLinkCopiedTimer)
		roomLinkCopiedTimer = window.setTimeout(() => { roomLinkCopied.value = false }, 2_000)
	} catch {
		// Clipboard access can be denied; the link is on screen to copy by hand.
	}
}

async function sharePersonalResult(): Promise<void> {
	const url = resultShareURL()
	if (navigator.share) {
		try {
			await navigator.share({ title: definition.value?.title || '2Pick', url })
			return
		} catch (error) {
			if (error instanceof DOMException && error.name === 'AbortError') return
		}
	}
	await navigator.clipboard?.writeText(url)
}

// Shares the /g/<serial> short URL rather than the localized route, so the link
// stays short and resolves for a recipient in any language.
function postShareURL(): string {
	return new URL(`/g/${encodeURIComponent(postSerial.value)}`, window.location.origin).toString()
}

async function sharePost(): Promise<void> {
	const url = postShareURL()

	if (navigator.share) {
		try {
			await navigator.share({ title: definition.value?.title || '2Pick', url })
			return
		} catch (error) {
			if (error instanceof DOMException && error.name === 'AbortError') return
		}
	}

	// Desktop browsers without the share sheet fall back to copying the link.
	try {
		await navigator.clipboard.writeText(url)
		shareCopied.value = true
		if (shareCopiedTimer) window.clearTimeout(shareCopiedTimer)
		shareCopiedTimer = window.setTimeout(() => { shareCopied.value = false }, 2_000)
	} catch {
		// Clipboard access can be denied; the URL is still in the address bar.
	}
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
  zoomedPicture.value = { image, rank: rankLabel(rank), title: element.title || '' }
}

function closeZoom(): void {
  zoomedPicture.value = null
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
  if (!restartDialog.value?.open) restartDialog.value?.showModal()
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
  closeZoom()
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

function onKeydown(event: KeyboardEvent): void {
  // Before the input guard: the zoom's own close button is a BUTTON and usually
  // holds focus while it is open, so Escape has to reach here from there too.
  if (event.key === 'Escape' && openedVideo.value && !videoDocked.value) {
    // Docked, not stopped: Escape leaves the big view, it does not end playback.
    dockVideo()
    return
  }
  if (event.key === 'Escape' && zoomedPicture.value) {
    closeZoom()
    return
  }
  const target = event.target
  if (target instanceof HTMLElement
    && (target.isContentEditable || ['INPUT', 'TEXTAREA', 'SELECT', 'BUTTON'].includes(target.tagName))) return
  if (event.key === '?') {
    controlsOpen.value = !controlsOpen.value
    return
  }
  if (!currentElements.value || readOnly.value || animating.value) return
  if (event.key === 'ArrowLeft' || event.key === '1') voteFor(currentElements.value[0].id)
  if (event.key === 'ArrowRight' || event.key === '2') voteFor(currentElements.value[1].id)
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
  <section v-if="loading" class="game-state-card">{{ t('gameLoading') }}</section>
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
          <RouterLink
            class="game-controls-toggle"
            :to="localizedPath(`/r/${encodeURIComponent(postSerial)}`, locale)"
            :title="t('viewRanking')"
            :aria-label="t('viewRanking')"
          >
            <svg viewBox="0 0 24 24" aria-hidden="true"><path d="M4 20v-7M10 20V8M16 20V4M22 20H2" /></svg>
          </RouterLink>
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
            @click="hostedRoom.hosting.value ? copyRoomLink() : openGameRoom()"
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
          <button
            class="game-controls-toggle"
            type="button"
            :aria-expanded="controlsOpen"
            aria-controls="game-control-help"
            :title="controlsOpen ? t('gameControlsClose') : t('gameControls')"
            :aria-label="controlsOpen ? t('gameControlsClose') : t('gameControls')"
            @click="controlsOpen = !controlsOpen"
          >
            <svg viewBox="0 0 24 24" aria-hidden="true"><rect x="2" y="5" width="20" height="14" rx="3" /><path d="M6 9h.01M10 9h.01M14 9h.01M18 9h.01M6 13h.01M10 13h.01M14 13h4M7 16h10" /></svg>
          </button>
        </div>
      </header>

      <section v-if="hostedRoom.hosting.value" class="game-room-invite">
        <div>
          <p class="game-room-invite-title">{{ t('roomInviteTitle') }}</p>
          <!-- The link is shown as text as well as copied: clipboard access is refused often
               enough that a copy button alone leaves people stuck. -->
          <p class="game-room-invite-url">{{ hostedRoom.inviteURL.value }}</p>
        </div>
        <div class="game-room-invite-actions">
          <button type="button" @click="copyRoomLink">
            {{ roomLinkCopied ? t('roomInviteCopied') : t('roomInviteCopy') }}
          </button>
          <RouterLink :to="`/${localeDefinition(locale).prefix}/room/${hostedRoom.serial.value}`">
            {{ t('roomInviteOpen') }}
          </RouterLink>
        </div>
      </section>

      <p v-else-if="hostedRoom.status.value === 'failed'" class="game-room-invite-error">
        {{ t('roomHostFailed') }}
      </p>

      <Transition name="game-help">
        <section v-if="controlsOpen" id="game-control-help" class="game-control-help">
          <header>
            <div>
              <svg viewBox="0 0 24 24" aria-hidden="true"><rect x="2" y="5" width="20" height="14" rx="3" /><path d="M6 9h.01M10 9h.01M14 9h.01M18 9h.01M6 13h.01M10 13h.01M14 13h4M7 16h10" /></svg>
              <h2>{{ t('gameControls') }}</h2>
            </div>
            <button type="button" :aria-label="t('gameControlsClose')" @click="controlsOpen = false">×</button>
          </header>
          <div class="game-control-list">
            <div><span><kbd>←</kbd><kbd>1</kbd></span><strong>{{ t('gameChooseLeft') }}</strong></div>
            <div><span><kbd>→</kbd><kbd>2</kbd></span><strong>{{ t('gameChooseRight') }}</strong></div>
          </div>
        </section>
      </Transition>

      <div class="game-layout">
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
              <button
                class="game-vote-button"
                type="button"
                :disabled="readOnly || animating"
                :aria-label="t('gameVoteFor', { title: element.title })"
                @click="voteFor(element.id)"
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
                </figcaption>
              </figure>
              <figure class="game-history-pick game-history-loser">
                <img v-if="historyImage(item.loser, item.loser_thumb)" :src="historyImage(item.loser, item.loser_thumb) || ''" :alt="item.loser_title" loading="lazy">
                <div v-else class="game-history-placeholder">{{ item.loser_title.slice(0, 1) }}</div>
                <figcaption>
                  <svg viewBox="0 0 24 24" aria-hidden="true"><path d="M6 6l12 12M18 6 6 18" /></svg>
                  <span>{{ item.loser_title }}</span>
                </figcaption>
              </figure>
            </li>
          </ol>
        </aside>

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
          <p v-if="communityLoading && !communityRanks.items.length" class="game-ranking-state">{{ t('gameCommunityLoading') }}</p>
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

      <!-- The picture on its own, which is what a ranking list is about. Outside
           both tab panels, because either tab can open it. Escape closes it; see
           onKeydown. -->
      <div
        v-if="zoomedPicture"
        class="game-rank-zoom"
        role="dialog"
        aria-modal="true"
        :aria-label="zoomedPicture.title"
        @click.self="closeZoom"
      >
        <button class="game-rank-zoom-close" type="button" :aria-label="t('close')" @click="closeZoom">
          <svg viewBox="0 0 24 24" aria-hidden="true"><path d="M6 6l12 12M18 6L6 18" /></svg>
        </button>
        <img :src="zoomedPicture.image" :alt="zoomedPicture.title">
        <p>
          <strong>{{ zoomedPicture.rank }}</strong>
          <span>{{ zoomedPicture.title }}</span>
        </p>
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

  <dialog
    ref="restartDialog"
    class="game-restart-dialog"
    aria-labelledby="game-restart-title"
    @cancel.prevent="dismissRestartDialog"
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

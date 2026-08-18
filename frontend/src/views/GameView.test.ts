// @vitest-environment happy-dom

import { flushPromises, mount, type VueWrapper } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { applyLocalVote, createInitialSnapshot } from '../game/localGame'
import { APIError } from '../lib/api'
import type { GameDefinition, GameSession } from '../services/gameplay'
import { translate } from '../i18n'
import { resetPostAccessForTests } from '../services/postAccess'
import GameView from './GameView.vue'

const serviceMocks = vi.hoisted(() => ({
  definition: vi.fn(),
  create: vi.fn(),
  resume: vi.fn(),
  submitVotes: vi.fn(),
	result: vi.fn(),
  ranks: vi.fn(),
  rank: vi.fn(),
}))

const routerMocks = vi.hoisted(() => ({
	replace: vi.fn(),
}))

const exportMocks = vi.hoisted(() => ({
	createPersonalRankingExport: vi.fn(),
	disposePersonalRankingExport: vi.fn(),
	downloadPersonalRankingExport: vi.fn(),
}))

const routeMock = vi.hoisted(() => ({
  name: 'game-localized',
  params: { locale: 'zh-tw', serial: 'post-1' },
	query: {} as Record<string, string>,
	fullPath: '/zh-tw/g/post-1',
}))

/*
Auth is mocked rather than resolved: signing in for real would mean a refresh request, and
what these tests are about is which of the two 18+ states the page renders. Every test
starts anonymous and settled, so the ordinary post tests never see the sign-in branch.
*/
const authMocks = vi.hoisted(() => ({
	authenticated: false,
	loading: false,
	refreshAuthState: vi.fn(),
}))

vi.mock('../composables/useAuth', async () => {
	const { computed } = await import('vue')
	return {
		useAuth: () => ({
			authenticated: computed(() => authMocks.authenticated),
			loading: computed(() => authMocks.loading),
			refreshAuthState: authMocks.refreshAuthState,
		}),
	}
})

beforeEach(() => {
	authMocks.authenticated = false
	authMocks.loading = false
})

vi.mock('vue-router', async (importOriginal) => ({
  ...await importOriginal<typeof import('vue-router')>(),
  useRoute: () => routeMock,
	useRouter: () => routerMocks,
}))

vi.mock('../rank/exportRanking', () => exportMocks)

const unlockMocks = vi.hoisted(() => ({ unlockPost: vi.fn() }))

vi.mock('../services/postAccess', async (importOriginal) => ({
  ...await importOriginal<typeof import('../services/postAccess')>(),
  unlockPost: unlockMocks.unlockPost,
}))

vi.mock('../services/gameplay', async (importOriginal) => ({
  ...await importOriginal<typeof import('../services/gameplay')>(),
  createGameplayService: () => ({
    definition: serviceMocks.definition,
    create: serviceMocks.create,
    resume: serviceMocks.resume,
    submitVotes: serviceMocks.submitVotes,
		result: serviceMocks.result,
  }),
}))

vi.mock('../services/publicContent', async (importOriginal) => ({
  ...await importOriginal<typeof import('../services/publicContent')>(),
  createPublicContentService: () => ({
    ranks: serviceMocks.ranks,
    rank: serviceMocks.rank,
  }),
}))

vi.mock('../components/CommentSection.vue', () => ({
  default: {
    props: ['postSerial', 'locale', 'localChampions'],
    template: '<section class="comments-section-stub"></section>',
  },
}))

const definition: GameDefinition = {
  title: '測試投票',
  serial: 'post-1',
  description: '',
  is_censored: false,
  elements_count: 2,
  max_elements: 2,
}

function session(gameSerial: string, titlePrefix: string): GameSession {
  return {
    game_serial: gameSerial,
    server_vote_count: 0,
    post: definition,
    elements: [1, 2].map((id) => ({
      id,
      title: `${titlePrefix} ${id}`,
      type: 'image',
      source_url: `https://example.test/${titlePrefix}-${id}.webp`,
      thumb_url: null,
      mediumthumb_url: null,
      lowthumb_url: null,
      video_start_second: null,
      video_end_second: null,
      video_source: null,
      video_id: null,
      video_duration_second: null,
    })),
  }
}

async function mountStartedGame(): Promise<VueWrapper> {
  serviceMocks.definition.mockResolvedValue(definition)
  serviceMocks.create.mockResolvedValueOnce(session('game-old', '舊選項'))

  const wrapper = mount(GameView, {
    global: {
      mocks: { $router: { go: vi.fn() } },
      stubs: {
        RouterLink: {
          props: ['to'],
          template: '<a :data-to="to"><slot /></a>',
        },
      },
    },
  })
  await flushPromises()
  await wrapper.get('.game-start-button').trigger('click')
  await flushPromises()
  return wrapper
}

async function mountSavedGame(): Promise<VueWrapper> {
  const firstVisit = await mountStartedGame()
  firstVisit.unmount()
  serviceMocks.definition.mockResolvedValue(definition)
  const wrapper = mount(GameView, {
    global: {
      mocks: { $router: { go: vi.fn() } },
      stubs: {
        RouterLink: {
          props: ['to'],
          template: '<a :data-to="to"><slot /></a>',
        },
      },
    },
  })
  await flushPromises()
  return wrapper
}

async function completeRestartHold(wrapper: VueWrapper): Promise<void> {
  await wrapper.get('.game-restart-confirm').trigger('pointerdown')
  await vi.advanceTimersByTimeAsync(1_000)
  await flushPromises()
}

describe('GameView restart regression', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    localStorage.clear()
    sessionStorage.clear()
    routeMock.name = 'game-localized'
    routeMock.params = { locale: 'zh-tw', serial: 'post-1' }
		routeMock.query = {}
    serviceMocks.ranks.mockResolvedValue({
      items: [], page: 1, per_page: 20, total: 0, total_pages: 0,
    })
    serviceMocks.submitVotes.mockResolvedValue({
      status: 'end_game', server_vote_count: 1, complete: true,
    })
		serviceMocks.result.mockResolvedValue({
			game_serial: 'game-old',
			post_serial: 'post-1',
			items: [],
		})
		exportMocks.createPersonalRankingExport.mockResolvedValue({
			imageUrl: 'blob:https://2pick.test/ranking-preview',
			blob: new Blob(['ranking'], { type: 'image/png' }),
			filename: '2pick-test-top10.png',
			text: '#1 分享結果 1',
		})
  })

  afterEach(() => {
    vi.useRealTimers()
    vi.unstubAllGlobals()
    document.body.innerHTML = ''
  })

  it('opens the restart decision without discarding the current local game', async () => {
    const wrapper = await mountStartedGame()
    const savedBefore = localStorage.getItem('2pick:game:post-1')

    await wrapper.get('button[title="重整遊戲"]').trigger('click')

    const dialog = wrapper.get('.game-restart-dialog').element as HTMLDialogElement
    expect(dialog.open).toBe(true)
    expect(wrapper.text()).toContain('要繼續目前進度，還是重開一局？')
    expect(localStorage.getItem('2pick:game:post-1')).toBe(savedBefore)
    expect(serviceMocks.create).toHaveBeenCalledTimes(1)

    wrapper.unmount()
  })

  it('asks what to do before resuming a saved game and offers its ranking', async () => {
    const wrapper = await mountSavedGame()

    expect((wrapper.get('.game-restart-dialog').element as HTMLDialogElement).open).toBe(true)
    expect(wrapper.text()).toContain('要繼續目前進度，還是重開一局？')
    expect(wrapper.get('.game-dialog-ranking-link').attributes('data-to')).toBe('/zh-tw/r/post-1')
    expect(serviceMocks.create).toHaveBeenCalledTimes(1)

    wrapper.unmount()
  })

  it('requires an uninterrupted one-second hold before replacing saved progress', async () => {
    vi.useFakeTimers()
    const wrapper = await mountSavedGame()
    serviceMocks.create.mockResolvedValueOnce(session('game-held-restart', '按住重開'))
    const restart = wrapper.get('.game-restart-confirm')
    expect(restart.text()).toContain('按住 1 秒重開')

    await restart.trigger('pointerdown')
    await vi.advanceTimersByTimeAsync(999)
    expect(restart.classes()).toContain('is-holding')
    expect(serviceMocks.create).toHaveBeenCalledTimes(1)
    await restart.trigger('pointerup')
    await vi.advanceTimersByTimeAsync(1_100)
    expect(serviceMocks.create).toHaveBeenCalledTimes(1)

    await restart.trigger('pointerdown')
    await vi.advanceTimersByTimeAsync(1_000)
    await flushPromises()

    expect(serviceMocks.create).toHaveBeenCalledTimes(2)
    expect(JSON.parse(localStorage.getItem('2pick:game:post-1') || '{}').game_serial).toBe('game-held-restart')

    wrapper.unmount()
    vi.useRealTimers()
  })

  it('resumes the saved game only after the user chooses continue', async () => {
    const wrapper = await mountSavedGame()
    const savedBefore = JSON.parse(localStorage.getItem('2pick:game:post-1') || '{}')

    await wrapper.get('.game-continue-option').trigger('click')

    expect((wrapper.get('.game-restart-dialog').element as HTMLDialogElement).open).toBe(false)
    expect(serviceMocks.create).toHaveBeenCalledTimes(1)
    expect(JSON.parse(localStorage.getItem('2pick:game:post-1') || '{}').game_serial).toBe(savedBefore.game_serial)
    expect(wrapper.find('.game-candidate-media').exists()).toBe(true)

    wrapper.unmount()
  })

  it('continues the existing game without calling the create API', async () => {
    const wrapper = await mountStartedGame()
    await wrapper.get('button[title="重整遊戲"]').trigger('click')

    await wrapper.get('.game-continue-option').trigger('click')

    expect((wrapper.get('.game-restart-dialog').element as HTMLDialogElement).open).toBe(false)
    expect(serviceMocks.create).toHaveBeenCalledTimes(1)
    expect(JSON.parse(localStorage.getItem('2pick:game:post-1') || '{}').game_serial).toBe('game-old')

    wrapper.unmount()
  })

  it('replaces and remounts the game view after a successful restart', async () => {
    vi.useFakeTimers()
    const wrapper = await mountStartedGame()
    const oldPage = wrapper.get('.game-page').element
    serviceMocks.create.mockResolvedValueOnce(session('game-new', '新選項'))

    await wrapper.get('button[title="重整遊戲"]').trigger('click')
    await completeRestartHold(wrapper)

    expect(serviceMocks.create).toHaveBeenLastCalledWith('post-1', 2, expect.any(AbortSignal))
    expect((wrapper.get('.game-restart-dialog').element as HTMLDialogElement).open).toBe(false)
    expect(wrapper.get('.game-page').element).not.toBe(oldPage)
    expect(wrapper.text()).toContain('新選項 1')
    expect(wrapper.text()).not.toContain('舊選項 1')
    expect(JSON.parse(localStorage.getItem('2pick:game:post-1') || '{}').game_serial).toBe('game-new')

    wrapper.unmount()
  })

	it('restarts a completed game with one click even though its revision is higher than a new game', async () => {
		vi.useFakeTimers()
		const wrapper = await mountStartedGame()
		await wrapper.get('.game-vote-button').trigger('click')
		const progressed = JSON.parse(localStorage.getItem('2pick:game-result:post-1') || '{}')
		expect(progressed.revision).toBeGreaterThan(1)
		serviceMocks.create.mockResolvedValueOnce(session('game-after-progress', '重開選項'))

		await wrapper.get('button[title="重整遊戲"]').trigger('click')
		expect(wrapper.get('.game-restart-confirm').text()).not.toContain('按住')
		await wrapper.get('.game-restart-confirm').trigger('click')
		await flushPromises()

    expect((wrapper.get('.game-restart-dialog').element as HTMLDialogElement).open).toBe(false)
    expect(wrapper.find('[role="alert"]').exists()).toBe(false)
    expect(JSON.parse(localStorage.getItem('2pick:game:post-1') || '{}').game_serial).toBe('game-after-progress')

    wrapper.unmount()
  })

  it('keeps the dialog and previous local progress when restart creation fails', async () => {
    vi.useFakeTimers()
    const wrapper = await mountStartedGame()
    const savedBefore = localStorage.getItem('2pick:game:post-1')
    serviceMocks.create.mockRejectedValueOnce(new Error('network unavailable'))

    await wrapper.get('button[title="重整遊戲"]').trigger('click')
    await completeRestartHold(wrapper)

    expect((wrapper.get('.game-restart-dialog').element as HTMLDialogElement).open).toBe(true)
    expect(wrapper.get('[role="alert"]').text()).toContain('目前無法開始新的一局')
    expect(wrapper.text()).toContain('舊選項 1')
    expect(localStorage.getItem('2pick:game:post-1')).toBe(savedBefore)

    wrapper.unmount()
  })

  it('backs each pictured option with a blurred copy of its own picture', async () => {
    const wrapper = await mountStartedGame()

    const backdrops = wrapper.findAll('.game-candidate-backdrop')
    const pictures = wrapper.findAll('.game-candidate-media img')
    expect(backdrops).toHaveLength(pictures.length)
    backdrops.forEach((backdrop, index) => {
      expect(backdrop.attributes('style')).toContain(pictures[index]!.attributes('src'))
    })

    wrapper.unmount()
  })

  it('only votes through the vote button and saves the vote locally before animation finishes', async () => {
    const wrapper = await mountStartedGame()

    await wrapper.get('.game-candidate-media').trigger('click')
    expect(JSON.parse(localStorage.getItem('2pick:game:post-1') || '{}').local_votes).toHaveLength(0)

    const selectedButton = wrapper.get('.game-vote-button')
    const selectedTitle = (selectedButton.attributes('aria-label') ?? '').replace('投給 ', '')
    await selectedButton.trigger('click')

    const saved = JSON.parse(localStorage.getItem('2pick:game-result:post-1') || '{}')
    expect(saved.local_votes).toHaveLength(1)
    expect(saved.outbox).toHaveLength(1)
    expect(wrapper.find('.game-candidate-winner').exists()).toBe(true)
    expect(wrapper.find('.game-candidate-loser').exists()).toBe(true)
    expect(wrapper.get('.game-history').text()).toContain(selectedTitle)
    expect(wrapper.findAll('.game-history img')).toHaveLength(2)

    wrapper.unmount()
  })

  it('holds a stable ranking loader until both its minimum duration and ranking data are ready', async () => {
    vi.useFakeTimers()
		routeMock.name = 'rank-localized'
		routeMock.query = { g: 'shared-game' }
		serviceMocks.definition.mockResolvedValue(definition)
		serviceMocks.result.mockResolvedValue({
			game_serial: 'shared-game',
			post_serial: 'post-1',
			items: [{
				rank: 1,
				win_count: 1,
				global_rank: 7,
				element: session('shared-game', '分享結果').elements[0],
			}],
		})
    const imageLoads: Array<() => void> = []
    class DeferredImage {
      onload: (() => void) | null = null
      onerror: (() => void) | null = null
      complete = false

      set src(_value: string) {
        imageLoads.push(() => {
          this.complete = true
          this.onload?.()
        })
      }
    }
    vi.stubGlobal('Image', DeferredImage)
    let resolveRanks!: (value: {
      items: never[]; page: number; per_page: number; total: number; total_pages: number
    }) => void
    serviceMocks.ranks.mockReturnValueOnce(new Promise((resolve) => { resolveRanks = resolve }))
		const wrapper = mount(GameView, {
			global: { stubs: { RouterLink: { props: ['to'], template: '<a :data-to="to"><slot /></a>' } } },
		})
		await flushPromises()

    expect(wrapper.find('.game-result-loading').exists()).toBe(true)
    expect(wrapper.find('.game-ranking').exists()).toBe(false)

    await vi.advanceTimersByTimeAsync(2_000)
    expect(wrapper.find('.game-result-loading').exists()).toBe(true)

    resolveRanks({ items: [], page: 1, per_page: 20, total: 0, total_pages: 0 })
    await flushPromises()

    expect(imageLoads.length).toBeGreaterThan(0)
    expect(wrapper.find('.game-result-loading').exists()).toBe(true)
    imageLoads.forEach((finishLoading) => finishLoading())
    await flushPromises()

    expect(wrapper.find('.game-result-loading').exists()).toBe(false)
    expect(wrapper.find('.game-ranking').exists()).toBe(true)

    wrapper.unmount()
  })

  it('archives a completed result without leaving resumable progress or reopening the continue dialog', async () => {
    vi.useFakeTimers()
    const wrapper = await mountStartedGame()

    await wrapper.get('.game-vote-button').trigger('click')

    expect(localStorage.getItem('2pick:game:post-1')).toBeNull()
    expect(localStorage.getItem('2pick:game-result:post-1')).not.toBeNull()

    const legacyCompletedProgress = localStorage.getItem('2pick:game-result:post-1')!
    wrapper.unmount()
    localStorage.setItem('2pick:game:post-1', legacyCompletedProgress)
    localStorage.removeItem('2pick:game-result:post-1')
    vi.useRealTimers()
    serviceMocks.definition.mockResolvedValue(definition)
    const revisit = mount(GameView, {
      global: {
        mocks: { $router: { go: vi.fn() } },
        stubs: {
          RouterLink: {
            props: ['to'],
            template: '<a :data-to="to"><slot /></a>',
          },
        },
      },
    })
    await flushPromises()

    expect(revisit.find('.game-start-button').exists()).toBe(true)
    expect((revisit.get('.game-restart-dialog').element as HTMLDialogElement).open).toBe(false)
    expect(localStorage.getItem('2pick:game:post-1')).toBeNull()
    expect(localStorage.getItem('2pick:game-result:post-1')).not.toBeNull()
    expect(serviceMocks.submitVotes).toHaveBeenCalledTimes(1)

    revisit.unmount()
  })

	it('navigates a completed vote to the shareable ranking URL with its game query', async () => {
		vi.useFakeTimers()
		serviceMocks.submitVotes
			.mockImplementationOnce(() => new Promise(() => undefined))
			.mockResolvedValueOnce({ status: 'end_game', server_vote_count: 1, complete: true })
		const wrapper = await mountStartedGame()

		await wrapper.get('.game-vote-button').trigger('click')
		await vi.advanceTimersByTimeAsync(3_500)
		await flushPromises()

		expect(routerMocks.replace).toHaveBeenCalledWith({
			path: '/zh-tw/r/post-1',
			query: { g: 'game-old' },
		})
		expect(serviceMocks.submitVotes.mock.calls.some((call) => call[4] === undefined)).toBe(true)
		wrapper.unmount()
	})

	it('restores a shared personal result from g and shows each top-ten global rank', async () => {
		vi.useFakeTimers()
		routeMock.name = 'rank-localized'
		routeMock.query = { g: 'shared-game' }
		serviceMocks.definition.mockResolvedValue(definition)
		serviceMocks.result.mockResolvedValue({
			game_serial: 'shared-game',
			post_serial: 'post-1',
			items: [1, 2, 3, 4, 5].map((rank) => ({
				rank,
				win_count: 6 - rank,
				global_rank: rank === 1 ? 7 : rank === 2 ? 0 : null,
				element: {
					...session('shared-game', '分享結果').elements[(rank - 1) % 2],
					id: rank,
				},
			})),
		})
		const share = vi.fn().mockResolvedValue(undefined)
		Object.defineProperty(navigator, 'share', { configurable: true, value: share })
		class LoadedImage {
			onload: (() => void) | null = null
			onerror: (() => void) | null = null
			complete = true
			set src(_value: string) {}
		}
		vi.stubGlobal('Image', LoadedImage)
		const wrapper = mount(GameView, {
			global: {
				stubs: { RouterLink: { props: ['to'], template: '<a :data-to="to"><slot /></a>' } },
			},
		})
		await flushPromises()
		await vi.advanceTimersByTimeAsync(2_000)
		await flushPromises()

		expect(serviceMocks.result).toHaveBeenCalledWith('shared-game')
		expect(wrapper.get('.game-result-tabs').text()).toContain('我的排名')
		expect(wrapper.get('.game-personal-ranking').text()).toContain('其他人的排名 #7')
		expect(wrapper.get('.game-personal-ranking').text()).toContain('其他人的排名 尚無資料')
		expect(wrapper.get('.game-personal-ranking').text()).not.toContain('#0')
		expect(wrapper.get('.game-personal-ranking').text()).not.toContain('—')
		// The picture is what was ranked: the top three are picture-first cards, the
		// rest a compact list, and every picture carries a blurred copy of itself so
		// it can be contained rather than cropped to a square.
		// The wins behind a place are noise beside the place itself.
		expect(wrapper.get('.game-personal-ranking').text()).not.toContain('勝')
		expect(wrapper.findAll('.game-personal-hero')).toHaveLength(3)
		expect(wrapper.findAll('.game-personal-rest li')).toHaveLength(2)
		const pictures = wrapper.findAll('.game-personal-media img')
		const backdrops = wrapper.findAll('.game-personal-backdrop')
		expect(pictures).toHaveLength(5)
		expect(backdrops).toHaveLength(5)
		backdrops.forEach((backdrop, index) => {
			expect(backdrop.attributes('style')).toContain(pictures[index]!.attributes('src'))
		})
		// A list frame is never the whole picture, so clicking one opens it.
		expect(wrapper.find('.game-rank-zoom').exists()).toBe(false)
		await wrapper.get('.game-personal-hero .game-personal-media').trigger('click')
		expect(wrapper.get('.game-rank-zoom img').attributes('src'))
			.toBe('https://example.test/分享結果-1.webp')
		await wrapper.get('.game-rank-zoom-close').trigger('click')
		expect(wrapper.find('.game-rank-zoom').exists()).toBe(false)

		await wrapper.get('.game-share-result').trigger('click')
		expect(share).toHaveBeenCalledWith(expect.objectContaining({
			url: expect.stringMatching(/\/zh-tw\/r\/post-1\?g=shared-game$/),
		}))
		expect(wrapper.get('.game-export-result').text()).toBe('匯出排名')
		await wrapper.get('.game-export-result').trigger('click')
		await flushPromises()
		expect(wrapper.get('.ranking-export-preview').attributes('src')).toContain('ranking-preview')
		expect((wrapper.get('.ranking-export-text').element as HTMLTextAreaElement).value).toContain('#1 分享結果 1')
		wrapper.unmount()
	})

	it('restores a shared personal result from the legacy s parameter', async () => {
		routeMock.name = 'rank-localized'
		routeMock.query = { s: 'legacy-shared-game' }
		serviceMocks.definition.mockResolvedValue(definition)
		serviceMocks.result.mockResolvedValue({
			game_serial: 'legacy-shared-game',
			post_serial: 'post-1',
			items: [{
				rank: 1,
				win_count: 1,
				global_rank: 3,
				element: { ...session('legacy-shared-game', '舊連結').elements[0], id: 1 },
			}],
		})
		const wrapper = mount(GameView, {
			global: {
				stubs: { RouterLink: { props: ['to'], template: '<a :data-to="to"><slot /></a>' } },
			},
		})
		await flushPromises()

		expect(serviceMocks.result).toHaveBeenCalledWith('legacy-shared-game')
		wrapper.unmount()
	})

	it('shows no-data labels instead of #0 or #- in ranking summaries', async () => {
		routeMock.name = 'rank-localized'
		serviceMocks.definition.mockResolvedValue(definition)
		const zeroRankReport = {
			rank: 0,
			win_rate: '0.0',
			date: '2026-08-04',
			element: {
				id: 1, title: '尚未排行', type: 'image', video_id: null, source_url: null,
				video_source: null, thumb_url: null, lowthumb_url: null, mediumthumb_url: null,
			},
		}
		serviceMocks.ranks.mockResolvedValue({
			items: [zeroRankReport], group: 'cumulative', page: 1, per_page: 20, total: 1, total_pages: 1,
		})
		serviceMocks.rank.mockResolvedValue({
			current: zeroRankReport,
			groups: { cumulative: zeroRankReport, recent_1000: null },
			history: { all: [], thousand_votes: [] },
		})
		const wrapper = mount(GameView, {
			global: { stubs: { RouterLink: { props: ['to'], template: '<a :data-to="to"><slot /></a>' } } },
		})
		await flushPromises()

		const summary = wrapper.get('.game-selected-group-ranks').text()
		expect(summary).toContain('尚無資料')
		expect(summary).not.toContain('#0')
		expect(summary).not.toContain('#—')
		wrapper.unmount()
	})

	it('offers only a one-click new-game action when replaying from a completed ranking', async () => {
		vi.useFakeTimers()
		const completedSession = session('completed-game', '完成選項')
		const completed = createInitialSnapshot(completedSession, 'old-writer', 'old-lease')
		applyLocalVote(completed, completed.current_match!.left_id, completed.current_match!.right_id, 'completed-vote')
		completed.outbox = []
		completed.server_vote_count = 1
		localStorage.setItem('2pick:game-result:post-1', JSON.stringify(completed))
		routeMock.name = 'rank-localized'
		routeMock.query = { g: 'completed-game' }
		serviceMocks.definition.mockResolvedValue(definition)
		serviceMocks.result.mockResolvedValue({
			game_serial: 'completed-game', post_serial: 'post-1',
			items: [{ rank: 1, win_count: 1, global_rank: 2, element: completedSession.elements[0] }],
		})
		class LoadedImage {
			onload: (() => void) | null = null
			onerror: (() => void) | null = null
			complete = true
			set src(_value: string) {}
		}
		vi.stubGlobal('Image', LoadedImage)
		const wrapper = mount(GameView, {
			global: { stubs: { RouterLink: { props: ['to'], template: '<a :data-to="to"><slot /></a>' } } },
		})
		await flushPromises()
		await vi.advanceTimersByTimeAsync(2_000)
		await flushPromises()

		await wrapper.get('.game-result-hero button').trigger('click')
		expect((wrapper.get('.game-restart-dialog').element as HTMLDialogElement).open).toBe(true)
		expect(wrapper.get('#game-restart-title').text()).toBe('重開一局')
		expect(wrapper.find('.game-continue-option').exists()).toBe(false)
		expect(wrapper.find('.game-dialog-ranking-link').exists()).toBe(false)

		serviceMocks.create.mockResolvedValueOnce(session('replay-game', '重新開始'))
		expect(wrapper.get('.game-restart-confirm').text()).not.toContain('按住')
		expect(wrapper.find('#game-restart-hold-hint').exists()).toBe(false)
		await wrapper.get('.game-restart-confirm').trigger('click')
		await flushPromises()
		expect(routerMocks.replace).toHaveBeenCalledWith({ path: '/zh-tw/g/post-1' })
		expect(sessionStorage.getItem('2pick:game:auto-resume:post-1')).toBe('replay-game')
		wrapper.unmount()
	})

  it('keeps the document scroll position fixed when ranking pagination replaces its data', async () => {
    routeMock.name = 'rank-localized'
    serviceMocks.definition.mockResolvedValue(definition)
    serviceMocks.rank.mockResolvedValue({ current: null, history: {} })
    const report = {
      rank: 1,
      win_rate: '75.0',
      date: '2026-08-03',
      element: {
        id: 1,
        title: '排名選項',
        type: 'image',
        video_id: null,
        source_url: null,
        video_source: null,
        thumb_url: null,
        lowthumb_url: null,
        mediumthumb_url: null,
      },
    }
    serviceMocks.ranks
      .mockResolvedValueOnce({ items: [report], page: 1, per_page: 20, total: 21, total_pages: 2 })
      .mockResolvedValueOnce({ items: [{ ...report, rank: 21 }], page: 2, per_page: 20, total: 21, total_pages: 2 })
    const scrollY = vi.spyOn(window, 'scrollY', 'get').mockReturnValue(640)
    const scrollTo = vi.spyOn(window, 'scrollTo')
    const wrapper = mount(GameView, {
      global: {
        stubs: {
          RouterLink: {
            props: ['to'],
            template: '<a :data-to="to"><slot /></a>',
          },
        },
      },
    })
    await flushPromises()

    await wrapper.get('.game-ranking-pagination button:last-child').trigger('click')
    await flushPromises()

    expect(wrapper.get('.game-ranking-pagination').text()).toContain('2 / 2')
    expect(scrollTo).toHaveBeenLastCalledWith({ top: 640, left: 0, behavior: 'instant' })

    wrapper.unmount()
    scrollY.mockRestore()
    scrollTo.mockRestore()
  })

  it('shows one all-time standing per row and opens the picture', async () => {
    routeMock.name = 'rank-localized'
    serviceMocks.definition.mockResolvedValue(definition)
    const report = {
      rank: 1,
      win_rate: '75.0',
      date: '2026-08-04',
      recent: { rank: 7, win_rate: '61.5', date: '2026-08-04' },
      element: {
        id: 1,
        title: '排名選項',
        type: 'image',
        video_id: null,
        source_url: 'https://cdn.test/full.jpg',
        video_source: null,
        thumb_url: 'https://cdn.test/thumb.jpg',
        lowthumb_url: null,
        mediumthumb_url: null,
      },
    }
    serviceMocks.ranks.mockResolvedValue({
      items: [report], group: 'cumulative', page: 1, per_page: 20, total: 1, total_pages: 1,
    })
    serviceMocks.rank.mockResolvedValue({
      current: report,
      groups: { cumulative: report, recent_1000: { ...report, rank: 7, win_rate: '61.5' } },
      history: {
        all: [{ rank: 1, win_rate: '75.0', date: '2026-08-04' }],
        thousand_votes: [{ rank: 7, win_rate: '61.5', date: '2026-08-04' }],
      },
    })

    const wrapper = mount(GameView, {
      global: {
        stubs: {
          RouterLink: {
            props: ['to'],
            template: '<a :data-to="to"><slot /></a>',
          },
        },
      },
    })
    await flushPromises()

    // One table, one request, one standing: the thousand-vote figures are not
    // shown anywhere, so a row is a title, its place and its win rate.
    expect(serviceMocks.ranks).toHaveBeenLastCalledWith('post-1', 1, 20)
    expect(wrapper.find('.game-ranking-group-tabs').exists()).toBe(false)
    expect(wrapper.get('.game-community-position').text()).toBe('1')
    expect(wrapper.find('.game-community-recent').exists()).toBe(false)
    expect(wrapper.get('.game-community-list').text()).not.toContain('最近一千筆')
    expect(wrapper.get('.game-selected-group-ranks').text()).toContain('累積排名#1')
    expect(wrapper.get('.game-selected-group-ranks').text()).not.toContain('最近一千筆')
    // The win rate is also drawn, against the absolute 0-100% scale.
    expect(wrapper.get('.game-selected-group-ranks .game-winrate-bar > span').attributes('style'))
      .toContain('width: 75%')
    expect(wrapper.get('.game-community-title .game-winrate-bar > span').attributes('style'))
      .toContain('width: 75%')
    expect(wrapper.find('.game-trend-ranges').exists()).toBe(false)

    // Clicking the picture shows the picture, at the largest size the API offers.
    expect(wrapper.find('.game-rank-zoom').exists()).toBe(false)
    await wrapper.get('.game-community-thumb').trigger('click')
    expect(wrapper.get('.game-rank-zoom img').attributes('src')).toBe('https://cdn.test/full.jpg')
    await wrapper.get('.game-rank-zoom-close').trigger('click')
    expect(wrapper.find('.game-rank-zoom').exists()).toBe(false)
    wrapper.unmount()
  })

  it('keeps keyboard help hidden until requested and always exposes the ranking link', async () => {
    const wrapper = await mountStartedGame()

    expect(wrapper.find('#game-control-help').exists()).toBe(false)
    expect(wrapper.get('a[title="排行榜"]').attributes('data-to')).toBe('/zh-tw/r/post-1')
    await wrapper.get('button[title="操作說明"]').trigger('click')

    expect(wrapper.get('#game-control-help').text()).toContain('選擇左側')
    expect(wrapper.get('#game-control-help').text()).toContain('選擇右側')
    expect(wrapper.get('.game-vote-button').text()).toBe('')

    wrapper.unmount()
  })

  it('starts candidate videos muted and plays only the hovered side after the hover delay', async () => {
    vi.useFakeTimers()
    const matchMedia = vi.spyOn(window, 'matchMedia').mockImplementation((query) => ({
      matches: query.includes('hover: hover'),
      media: query,
      onchange: null,
      addListener: vi.fn(),
      removeListener: vi.fn(),
      addEventListener: vi.fn(),
      removeEventListener: vi.fn(),
      dispatchEvent: vi.fn(),
    }))
    serviceMocks.definition.mockResolvedValue(definition)
    const videoSession = session('video-game', '影片')
    videoSession.elements.forEach((element, index) => {
      element.type = 'video'
      element.source_url = `https://example.test/video-${index + 1}.mp4`
    })
    serviceMocks.create.mockResolvedValueOnce(videoSession)
    const play = vi.spyOn(HTMLMediaElement.prototype, 'play').mockResolvedValue()
    const pause = vi.spyOn(HTMLMediaElement.prototype, 'pause').mockImplementation(() => undefined)

    const wrapper = mount(GameView, {
      attachTo: document.body,
      global: {
        mocks: { $router: { go: vi.fn() } },
        stubs: {
          RouterLink: {
            props: ['to'],
            template: '<a :data-to="to"><slot /></a>',
          },
        },
      },
    })
    await flushPromises()
    await wrapper.get('.game-start-button').trigger('click')
    await flushPromises()

    const videos = wrapper.findAll('video[data-game-media-id]')
    expect(videos).toHaveLength(2)
    expect(videos.every((video) => (video.element as HTMLVideoElement).muted)).toBe(true)
    await wrapper.findAll('.game-candidate-media')[0]!.trigger('mouseenter')
    await vi.advanceTimersByTimeAsync(250)

    expect(play).toHaveBeenCalledTimes(1)
    expect(pause).toHaveBeenCalledTimes(1)
    expect((videos[0]!.element as HTMLVideoElement).muted).toBe(false)
    expect((videos[1]!.element as HTMLVideoElement).muted).toBe(true)

    wrapper.unmount()
    matchMedia.mockRestore()
    play.mockRestore()
    pause.mockRestore()
    vi.useRealTimers()
  })
})

describe('GameView option preview and sharing', () => {
  const previewDefinition: GameDefinition = {
    ...definition,
    element1: {
      id: 11,
      url: 'https://file.2pick.app/low/267x400/one.webp',
      url2: 'https://file.2pick.app/medium/800x800/one.webp',
      title: '選項一',
      type: 'image',
      video_source: null,
      previewable: true,
    },
    element2: {
      id: 12,
      url: 'https://file.2pick.app/low/267x400/two.webp',
      url2: null,
      title: '選項二',
      type: 'image',
      video_source: null,
      previewable: true,
    },
  }

  beforeEach(() => {
    vi.clearAllMocks()
    localStorage.clear()
    sessionStorage.clear()
    routeMock.name = 'game-localized'
    routeMock.params = { locale: 'zh-tw', serial: 'post-1' }
    routeMock.query = {}
    serviceMocks.ranks.mockResolvedValue({
      items: [], page: 1, per_page: 20, total: 0, total_pages: 0,
    })
  })

  afterEach(() => {
    vi.useRealTimers()
    vi.unstubAllGlobals()
    document.body.innerHTML = ''
  })

  async function mountSetupScreen(gameDefinition: GameDefinition): Promise<VueWrapper> {
    serviceMocks.definition.mockResolvedValue(gameDefinition)
    const wrapper = mount(GameView, {
      global: {
        mocks: { $router: { go: vi.fn() } },
        stubs: {
          RouterLink: {
            props: ['to'],
            template: '<a :data-to="to"><slot /></a>',
          },
        },
      },
    })
    await flushPromises()
    return wrapper
  }

  it('previews both options on load without starting a game', async () => {
    const wrapper = await mountSetupScreen(previewDefinition)

    const cards = wrapper.findAll('.game-preview-card')
    expect(cards).toHaveLength(2)
    expect(cards[0]!.get('img').attributes('src')).toBe('https://file.2pick.app/low/267x400/one.webp')
    expect(cards[0]!.text()).toContain('選項一')
    expect(cards[1]!.text()).toContain('選項二')
    // The preview rides on the definition request; pressing start is not required.
    expect(serviceMocks.create).not.toHaveBeenCalled()

    wrapper.unmount()
  })

  it('omits the preview when the definition carries no options', async () => {
    const wrapper = await mountSetupScreen(definition)

    expect(wrapper.find('.game-preview-card').exists()).toBe(false)
    expect(wrapper.find('.game-start-button').exists()).toBe(true)

    wrapper.unmount()
  })

  it('falls back to the larger variant when a preview thumbnail fails', async () => {
    const wrapper = await mountSetupScreen(previewDefinition)

    const image = wrapper.findAll('.game-preview-card')[0]!.get('img')
    await image.trigger('error')

    expect(image.attributes('src')).toBe('https://file.2pick.app/medium/800x800/one.webp')

    wrapper.unmount()
  })

  it('copies the short /g/ link when the native share sheet is unavailable', async () => {
    const writeText = vi.fn().mockResolvedValue(undefined)
    Object.defineProperty(navigator, 'share', { value: undefined, configurable: true })
    Object.defineProperty(navigator, 'clipboard', { value: { writeText }, configurable: true })

    const wrapper = await mountSetupScreen(previewDefinition)
    await wrapper.get('.game-share-button').trigger('click')
    await flushPromises()

    expect(writeText).toHaveBeenCalledWith(`${window.location.origin}/g/post-1`)
    expect(wrapper.get('.game-share-button').text()).toContain('已複製連結')

    wrapper.unmount()
  })

  it('uses the native share sheet with the short /g/ link when available', async () => {
    const share = vi.fn().mockResolvedValue(undefined)
    Object.defineProperty(navigator, 'share', { value: share, configurable: true })

    const wrapper = await mountSetupScreen(previewDefinition)
    await wrapper.get('.game-share-button').trigger('click')
    await flushPromises()

    expect(share).toHaveBeenCalledWith(expect.objectContaining({
      title: '測試投票',
      url: `${window.location.origin}/g/post-1`,
    }))

    wrapper.unmount()
  })
  it('previews a ranked video as a still and only loads the player when asked', async () => {
    routeMock.name = 'rank-localized'
    serviceMocks.definition.mockResolvedValue(definition)
    const videoReport = {
      rank: 1,
      win_rate: '75.0',
      date: '2026-08-04',
      element: {
        id: 1,
        title: '影片選項',
        type: 'video',
        video_id: 'abc123',
        source_url: null,
        video_source: 'youtube',
        thumb_url: 'https://cdn.test/thumb.jpg',
        lowthumb_url: null,
        mediumthumb_url: null,
      },
    }
    serviceMocks.ranks.mockResolvedValue({
      items: [videoReport], group: 'cumulative', page: 1, per_page: 20, total: 1, total_pages: 1,
    })
    serviceMocks.rank.mockResolvedValue({
      current: videoReport,
      groups: { cumulative: videoReport, recent_1000: null },
      history: { all: [{ rank: 1, win_rate: '75.0', date: '2026-08-04' }], thousand_votes: [] },
    })

    const wrapper = mount(GameView, {
      global: { stubs: { RouterLink: { props: ['to'], template: '<a :data-to="to"><slot /></a>' } } },
    })
    await flushPromises()

    // Opening a ranking must not pull an embed for every row: the still shows
    // first and the iframe only arrives on click.
    const preview = wrapper.get('.game-rank-video')
    expect(preview.find('iframe').exists()).toBe(false)
    expect(preview.get('img').attributes('src')).toBe('https://cdn.test/thumb.jpg')
    expect(wrapper.find('.game-community-thumb-video').exists()).toBe(true)

    await preview.get('button').trigger('click')

    const frame = wrapper.get('.game-rank-video iframe')
    expect(frame.attributes('src')).toContain('/embed/abc123')
    expect(wrapper.find('.game-rank-video button').exists()).toBe(false)

    // A still frame is not the entry: the row's thumbnail opens the player.
    await wrapper.get('.game-community-thumb').trigger('click')
    expect(wrapper.find('.game-rank-zoom').exists()).toBe(false)
    const player = wrapper.get('.game-rank-player')
    expect(player.get('iframe').attributes('src')).toContain('/embed/abc123')
    expect(player.classes()).not.toContain('is-docked')

    // Leaving the big view docks the same iframe rather than unmounting it,
    // because a new iframe would restart the video.
    await player.get('.game-rank-player-dock').trigger('click')
    expect(wrapper.get('.game-rank-player').classes()).toContain('is-docked')
    expect(wrapper.get('.game-rank-player iframe').attributes('src')).toContain('/embed/abc123')

    await wrapper.get('.game-rank-player-expand').trigger('click')
    expect(wrapper.get('.game-rank-player').classes()).not.toContain('is-docked')

    // Only the close button ends playback.
    await wrapper.get('.game-rank-player-close').trigger('click')
    expect(wrapper.find('.game-rank-player').exists()).toBe(false)
    wrapper.unmount()
  })

  it('charts the top five and leaves everyone below it a win rate', async () => {
    routeMock.name = 'rank-localized'
    serviceMocks.definition.mockResolvedValue(definition)
    const element = {
      id: 1, title: '排名選項', type: 'image', video_id: null, source_url: null,
      video_source: null, thumb_url: null, lowthumb_url: null, mediumthumb_url: null,
    }
    const history = { all: [{ rank: 5, win_rate: '75.0', date: '2026-08-04' }], thousand_votes: [] }
    const charted = { rank: 5, win_rate: '75.0', date: '2026-08-04', element }
    serviceMocks.ranks.mockResolvedValue({
      items: [charted], group: 'cumulative', page: 1, per_page: 20, total: 1, total_pages: 1,
    })
    serviceMocks.rank.mockResolvedValue({
      current: charted, groups: { cumulative: charted, recent_1000: null }, history,
    })

    let wrapper = mount(GameView, {
      global: { stubs: { RouterLink: { props: ['to'], template: '<a :data-to="to"><slot /></a>' } } },
    })
    await flushPromises()
    expect(wrapper.find('.game-trend-chart').exists()).toBe(true)

    // Sixth place and below reads by win rate alone: a history chart for every
    // row is noise, and the original site drew one for the podium only.
    const beyond = { rank: 6, win_rate: '31.0', date: '2026-08-04', element }
    serviceMocks.ranks.mockResolvedValue({
      items: [beyond], group: 'cumulative', page: 1, per_page: 20, total: 1, total_pages: 1,
    })
    serviceMocks.rank.mockResolvedValue({
      current: beyond, groups: { cumulative: beyond, recent_1000: null }, history,
    })
    wrapper.unmount()
    wrapper = mount(GameView, {
      global: { stubs: { RouterLink: { props: ['to'], template: '<a :data-to="to"><slot /></a>' } } },
    })
    await flushPromises()

    // No chart, and no notice about there being no chart either: the absence is
    // the presentation. The trend label goes with it, so nothing is left
    // announcing a section that is not rendered.
    expect(wrapper.find('.game-trend-chart').exists()).toBe(false)
    expect(wrapper.text()).not.toContain('趨勢')
    // No chart means no reserved box either: nothing is going to arrive to fill
    // it, and a blank 9.75rem panel under the picture reads as a broken card.
    expect(wrapper.find('.game-trend-slot').exists()).toBe(false)
    expect(wrapper.get('.game-community-open').text()).toContain('31.0')
    wrapper.unmount()
  })

  it('keeps exactly one chart-slot box while a selection reloads', async () => {
    vi.useFakeTimers()
    routeMock.name = 'rank-localized'
    serviceMocks.definition.mockResolvedValue(definition)
    const element = {
      id: 1, title: '排名選項', type: 'image', video_id: null, source_url: null,
      video_source: null, thumb_url: null, lowthumb_url: null, mediumthumb_url: null,
    }
    const report = { rank: 2, win_rate: '75.0', date: '2026-08-04', element }
    serviceMocks.ranks.mockResolvedValue({
      items: [report], group: 'cumulative', page: 1, per_page: 20, total: 1, total_pages: 1,
    })
    let resolveDetails!: (value: unknown) => void
    serviceMocks.rank.mockReturnValue(new Promise((resolve) => { resolveDetails = resolve }))

    const wrapper = mount(GameView, {
      global: { stubs: { RouterLink: { props: ['to'], template: '<a :data-to="to"><slot /></a>' } } },
    })
    await flushPromises()

    // The loading state used to render alongside the empty state, stacking two
    // 240px placeholders and swinging the card 243px on every pagination click.
    expect(wrapper.findAll('.game-trend-slot')).toHaveLength(1)
    // A read that has barely started shows no spinner and no "no data" note: a
    // cached read returns inside this window, and flashing either one and back
    // again is what made switching selections look like tearing.
    expect(wrapper.find('.game-trend-loader').exists()).toBe(false)
    expect(wrapper.get('.game-trend-slot').text()).toBe('')

    // The standing comes off the selected row, so it and its bar stay put for
    // the whole read instead of blanking and moving the card twice per click.
    expect(wrapper.get('.game-selected-group-ranks').text()).toContain('#2')
    expect(wrapper.find('.game-selected-group-ranks .game-winrate-bar').exists()).toBe(true)

    await vi.advanceTimersByTimeAsync(260)
    expect(wrapper.findAll('.game-trend-slot')).toHaveLength(1)
    expect(wrapper.find('.game-trend-loader').exists()).toBe(true)
    expect(wrapper.get('.game-selected-group-ranks').text()).toContain('#2')

    resolveDetails({
      current: report,
      groups: { cumulative: report, recent_1000: null },
      history: { all: [{ rank: 2, win_rate: '75.0', date: '2026-08-04' }], thousand_votes: [] },
    })
    await flushPromises()

    expect(wrapper.findAll('.game-trend-slot')).toHaveLength(1)
    expect(wrapper.find('.game-trend-chart').exists()).toBe(true)
    wrapper.unmount()
    vi.useRealTimers()
  })
})


/**
 * The door code.
 *
 * A protected post answers 404 to a visitor who has not unlocked it — the same answer a
 * wrong link gets, because the API will not tell a stranger which serials are real. So the
 * page has to open the prompt on that 404 rather than the error card, and only fall back
 * to the error once the unlock itself says there is no such post.
 */
describe('GameView door code', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    localStorage.clear()
    sessionStorage.clear()
    resetPostAccessForTests()
  })

  function mountView(): VueWrapper {
    return mount(GameView, {
      global: {
        mocks: { $router: { go: vi.fn() } },
        stubs: {
          RouterLink: { props: ['to'], template: '<a :data-to="to"><slot /></a>' },
        },
      },
    })
  }

  function notFound(): APIError {
    return new APIError(404, { error: { code: 'not_found', message: 'no' } } as never)
  }

  it('asks for the door code when the post answers not found', async () => {
    serviceMocks.definition.mockRejectedValue(notFound())

    const wrapper = mountView()
    await flushPromises()

    expect(wrapper.find('.game-door-code').exists()).toBe(true)
    expect(wrapper.find('.game-error-state').exists()).toBe(false)
  })

  // Anything other than a 404 is final: the post is not hidden, the API is unwell.
  it('shows the error card for any other failure', async () => {
    serviceMocks.definition.mockRejectedValue(
      new APIError(500, { error: { code: 'internal', message: 'no' } } as never))

    const wrapper = mountView()
    await flushPromises()

    expect(wrapper.find('.game-door-code').exists()).toBe(false)
    expect(wrapper.find('.game-error-state').exists()).toBe(true)
  })

  it('loads the post once the code is accepted', async () => {
    serviceMocks.definition.mockRejectedValueOnce(notFound()).mockResolvedValue(definition)
    unlockMocks.unlockPost.mockResolvedValue({ ok: true })

    const wrapper = mountView()
    await flushPromises()
    await wrapper.get('.game-door-code-input').setValue('door-code')
    await wrapper.get('.game-door-code-form').trigger('submit')
    await flushPromises()

    expect(unlockMocks.unlockPost).toHaveBeenCalledWith('post-1', 'door-code', undefined, expect.anything())
    expect(wrapper.find('.game-door-code').exists()).toBe(false)
    expect(wrapper.text()).toContain(definition.title)
  })

  it('keeps the prompt up and says so when the code is wrong', async () => {
    serviceMocks.definition.mockRejectedValue(notFound())
    unlockMocks.unlockPost.mockResolvedValue({ ok: false, kind: 'wrong-password' })

    const wrapper = mountView()
    await flushPromises()
    await wrapper.get('.game-door-code-input').setValue('not-the-code')
    await wrapper.get('.game-door-code-form').trigger('submit')
    await flushPromises()

    expect(wrapper.find('.game-door-code').exists()).toBe(true)
    expect(wrapper.get('.game-door-code-error').text()).toBe(translate('zh_TW', 'gameDoorCodeWrong'))
  })

  // The unlock is what tells a protected post from one that does not exist. Once it says
  // not found, there is nothing left to type.
  it('falls back to the error card when the post really does not exist', async () => {
    serviceMocks.definition.mockRejectedValue(notFound())
    unlockMocks.unlockPost.mockResolvedValue({ ok: false, kind: 'not-found' })

    const wrapper = mountView()
    await flushPromises()
    await wrapper.get('.game-door-code-input').setValue('anything')
    await wrapper.get('.game-door-code-form').trigger('submit')
    await flushPromises()

    expect(wrapper.find('.game-door-code').exists()).toBe(false)
    expect(wrapper.find('.game-error-state').exists()).toBe(true)
  })

  // An empty field must not spend one of the ten attempts a minute the post allows.
  it('sends nothing when the field is empty', async () => {
    serviceMocks.definition.mockRejectedValue(notFound())

    const wrapper = mountView()
    await flushPromises()
    await wrapper.get('.game-door-code-form').trigger('submit')
    await flushPromises()

    expect(unlockMocks.unlockPost).not.toHaveBeenCalled()
  })
})

describe('GameView ad slots', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    localStorage.clear()
    sessionStorage.clear()
    routeMock.name = 'rank-localized'
    routeMock.params = { locale: 'zh-tw', serial: 'post-1' }
    routeMock.query = {}
    window.__APP_CONFIG__ = {
      apiBaseUrl: '/api/v1',
      ads: { publisherId: 'ca-pub-1', slots: { rankList: '10', gameResult: '11' } },
    } as Window['__APP_CONFIG__']
    // Never fires, so no test loads the real tag.
    vi.stubGlobal('IntersectionObserver', class {
      observe(): void {}
      disconnect(): void {}
      unobserve(): void {}
    })
  })

  afterEach(() => {
    vi.unstubAllGlobals()
    window.__APP_CONFIG__ = undefined
    document.body.innerHTML = ''
  })

  function reports(count: number) {
    return Array.from({ length: count }, (_, index) => ({
      rank: index + 1,
      win_rate: '50.0',
      date: '2026-08-04',
      element: {
        id: index + 1,
        title: `選項 ${index + 1}`,
        type: 'image',
        source_url: `https://cdn.test/${index + 1}.webp`,
        thumb_url: null,
        lowthumb_url: null,
        mediumthumb_url: null,
        video_id: null,
        video_source: null,
      },
    }))
  }

  async function mountRankPage(gameDefinition: GameDefinition, rows: number): Promise<VueWrapper> {
    serviceMocks.definition.mockResolvedValue(gameDefinition)
    const items = reports(rows)
    serviceMocks.ranks.mockResolvedValue({
      items, group: 'cumulative', page: 1, per_page: 20, total: items.length, total_pages: 1,
    })
    const wrapper = mount(GameView, {
      global: { stubs: { RouterLink: { props: ['to'], template: '<a :data-to="to"><slot /></a>' } } },
    })
    await flushPromises()
    return wrapper
  }

  it('breaks the ranking list after the tenth row', async () => {
    const wrapper = await mountRankPage(definition, 14)

    const rows = wrapper.findAll('.game-community-list > li')
    expect(rows[10]!.classes()).toContain('game-community-ad')
    expect(rows[10]!.find('.ad-slot').exists()).toBe(true)
    expect(wrapper.findAll('.game-community-ad')).toHaveLength(1)
    wrapper.unmount()
  })

  it('leaves a short list unbroken', async () => {
    const wrapper = await mountRankPage(definition, 6)

    expect(wrapper.findAll('.game-community-ad')).toHaveLength(0)
    wrapper.unmount()
  })

  it('shows no ad at all on an 18+ post', async () => {
    // AdSense does not allow its units on adult content, and the penalty is the
    // whole account, so a censored post carries no slot on either of its pages.
    // Signed in, because a visitor gets the sign-in prompt instead of the list.
    authMocks.authenticated = true
    const wrapper = await mountRankPage({ ...definition, is_censored: true }, 14)

    expect(wrapper.findAll('.ad-slot')).toHaveLength(0)
    wrapper.unmount()
  })
})

/*
18+ POSTS: PREVIEW FOR ANYONE, EVERYTHING ELSE FOR AN ACCOUNT.

The home page lists adult posts and links to this page, so the preview has to survive — a
visitor who clicks through must see what they clicked, blurred. What must not survive is
anything past it: no game may start, no ranking may load, and a game saved from an earlier
session must not put a playable board in front of the prompt.
*/
describe('GameView adult content', () => {
	const adultDefinition: GameDefinition = {
		...definition,
		is_censored: true,
		element1: {
			id: 1, url: 'https://cdn.test/1.webp', url2: null, title: '選項 1',
			type: 'image', video_source: null, previewable: true,
		},
		element2: {
			id: 2, url: 'https://cdn.test/2.webp', url2: null, title: '選項 2',
			type: 'image', video_source: null, previewable: true,
		},
	}

	beforeEach(() => {
		vi.clearAllMocks()
		localStorage.clear()
		sessionStorage.clear()
		routeMock.name = 'game-localized'
		routeMock.params = { locale: 'zh-tw', serial: 'post-1' }
		routeMock.query = {}
		routeMock.fullPath = '/zh-tw/g/post-1'
	})

	// JSON rather than the object's toString, so the redirect target is readable.
	function mountAdultView() {
		return mount(GameView, {
			global: {
				mocks: { $router: { go: vi.fn() } },
				stubs: {
					RouterLink: { props: ['to'], template: '<a :data-to="JSON.stringify(to)"><slot /></a>' },
				},
			},
		})
	}

	async function mountAdultPage(): Promise<VueWrapper> {
		serviceMocks.definition.mockResolvedValue(adultDefinition)
		const wrapper = mountAdultView()
		await flushPromises()
		return wrapper
	}

	it('previews the two options, blurred, to a visitor', async () => {
		const wrapper = await mountAdultPage()

		const previews = wrapper.findAll('.game-preview-media')
		expect(previews).toHaveLength(2)
		for (const preview of previews) expect(preview.classes()).toContain('is-censored')
		wrapper.unmount()
	})

	it('offers a visitor the sign-in page instead of the start button', async () => {
		const wrapper = await mountAdultPage()

		expect(wrapper.find('.game-setup-panel .game-count-options').exists()).toBe(false)
		expect(wrapper.find('.game-sign-in-hint').text()).toBe(translate('zh_TW', 'gameSignInRequiredHint'))

		const link = wrapper.get('.game-start-button')
		expect(link.element.tagName).toBe('A')
		expect(JSON.parse(link.attributes('data-to')!)).toEqual({
			path: '/zh-tw/login',
			query: { redirect: '/zh-tw/g/post-1' },
		})
		expect(serviceMocks.create).not.toHaveBeenCalled()
		wrapper.unmount()
	})

	it('lets an account start the game as usual', async () => {
		authMocks.authenticated = true
		const wrapper = await mountAdultPage()

		expect(wrapper.find('.game-setup-panel .game-count-options').exists()).toBe(true)
		expect(wrapper.get('.game-start-button').element.tagName).toBe('BUTTON')
		wrapper.unmount()
	})

	/*
	A saved game is what makes this more than a rendering rule.

	The snapshot lives in localStorage, so signing out does not remove it, and the board
	branch renders whenever one exists. Without the gate running before the snapshot is
	read, a visitor would resume the game and vote — locally, since the server answers 401.
	*/
	it('does not resume a game saved before the account signed out', async () => {
		authMocks.authenticated = true
		serviceMocks.definition.mockResolvedValue(adultDefinition)
		serviceMocks.create.mockResolvedValueOnce(session('game-adult', '選項'))
		const signedIn = mountAdultView()
		await flushPromises()
		await signedIn.get('.game-start-button').trigger('click')
		await flushPromises()
		expect(signedIn.find('.game-candidate-media').exists()).toBe(true)
		signedIn.unmount()

		authMocks.authenticated = false
		const visitor = await mountAdultPage()

		expect(visitor.find('.game-candidate-media').exists()).toBe(false)
		expect(visitor.find('.game-sign-in-hint').exists()).toBe(true)
		visitor.unmount()
	})

	// The page cannot decide while the session is still resolving: a signed-in visitor
	// would see the prompt flash past on every load.
	it('waits for the session before it decides', async () => {
		authMocks.loading = true
		const wrapper = await mountAdultPage()

		expect(authMocks.refreshAuthState).toHaveBeenCalledWith('zh_TW')
		expect(wrapper.find('.game-sign-in-hint').exists()).toBe(false)
		wrapper.unmount()
	})

	describe('the ranking page', () => {
		beforeEach(() => {
			routeMock.name = 'rank-localized'
			routeMock.fullPath = '/zh-tw/r/post-1'
		})

		it('locks itself and asks a visitor to sign in', async () => {
			const wrapper = await mountAdultPage()

			const card = wrapper.get('.game-sign-in-required')
			expect(card.find('.game-sign-in-hint').text()).toBe(translate('zh_TW', 'gameRankSignInRequiredHint'))
			expect(wrapper.find('.game-community-list').exists()).toBe(false)
			expect(serviceMocks.ranks).not.toHaveBeenCalled()
			wrapper.unmount()
		})

		it('opens for an account', async () => {
			authMocks.authenticated = true
			serviceMocks.ranks.mockResolvedValue({
				items: [], group: 'cumulative', page: 1, per_page: 20, total: 0, total_pages: 0,
			})
			const wrapper = await mountAdultPage()

			expect(wrapper.find('.game-sign-in-required').exists()).toBe(false)
			expect(serviceMocks.ranks).toHaveBeenCalled()
			wrapper.unmount()
		})
	})
})

// @vitest-environment happy-dom

import { flushPromises, mount } from '@vue/test-utils'
import { defineComponent, ref } from 'vue'
import { createMemoryHistory, createRouter } from 'vue-router'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import App from './App.vue'
import type { RanksPage } from './services/publicContent'
import GameView from './views/GameView.vue'

const navigationMocks = vi.hoisted(() => ({
  definition: vi.fn(),
  create: vi.fn(),
  ranks: vi.fn(),
  rank: vi.fn(),
}))

vi.mock('./services/gameplay', async (importOriginal) => ({
  ...await importOriginal<typeof import('./services/gameplay')>(),
  createGameplayService: () => ({
    definition: navigationMocks.definition,
    create: navigationMocks.create,
    resume: vi.fn(),
    submitVotes: vi.fn(),
  }),
}))

vi.mock('./services/publicContent', async (importOriginal) => ({
  ...await importOriginal<typeof import('./services/publicContent')>(),
  createPublicContentService: () => ({
    ranks: navigationMocks.ranks,
    rank: navigationMocks.rank,
  }),
}))

vi.mock('./components/CommentSection.vue', () => ({
  default: {
    props: ['postSerial', 'locale', 'localChampions'],
    template: '<section class="comments-section-stub"></section>',
  },
}))

describe('App navigation cache', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    localStorage.clear()
    sessionStorage.clear()
  })

  it('restores the same home view instance and its state after returning from ranking', async () => {
    let homeMounts = 0
    const HomeView = defineComponent({
      name: 'HomeView',
      setup() {
        homeMounts += 1
        return { search: ref('') }
      },
      template: '<input class="cached-home-search" v-model="search">',
    })
    const RankView = defineComponent({
      name: 'RankView',
      template: '<div class="rank-view">Ranking</div>',
    })
    const router = createRouter({
      history: createMemoryHistory(),
      routes: [
        { path: '/', name: 'home', component: HomeView, meta: { viewKey: 'home', keepAlive: true } },
        { path: '/r/post-1', name: 'rank', component: RankView },
      ],
    })
    await router.push('/')
    await router.isReady()
    const wrapper = mount(App, {
      global: {
        plugins: [router],
        stubs: { AppHeader: true, AppFooter: true, Transition: false },
      },
    })
    await wrapper.get('.cached-home-search').setValue('保留的搜尋')

    await router.push('/r/post-1')
    await flushPromises()
    await new Promise((resolve) => window.setTimeout(resolve, 0))
    await flushPromises()
    expect(router.currentRoute.value.path).toBe('/r/post-1')
    expect(wrapper.find('.rank-view').exists()).toBe(true)
    router.back()
    await new Promise((resolve) => window.setTimeout(resolve, 0))
    await flushPromises()

    expect(router.currentRoute.value.path).toBe('/')
    expect(wrapper.get<HTMLInputElement>('.cached-home-search').element.value).toBe('保留的搜尋')
    expect(homeMounts).toBe(1)
    wrapper.unmount()
  })

  it('renders ranking when navigating from the game route that uses the same view component', async () => {
    navigationMocks.definition.mockResolvedValue({
      title: '測試投票', serial: 'post-1', description: '', is_censored: false,
      elements_count: 8, max_elements: 8,
    })
    navigationMocks.ranks.mockResolvedValue({
      items: [{
        rank: 1, win_rate: '75.0', date: '2026-08-03',
        element: {
          title: '第一名', type: 'image', id: 1, video_id: null,
          source_url: 'https://example.test/one.webp', video_source: null,
          thumb_url: 'https://example.test/one.webp',
          lowthumb_url: null, mediumthumb_url: null,
        },
      }],
      page: 1, per_page: 20, total: 2, total_pages: 2,
    })
    navigationMocks.rank.mockResolvedValue({ current: null, history: {} })
    navigationMocks.create.mockResolvedValue({
      game_serial: 'game-1', server_vote_count: 0,
      post: {
        title: '測試投票', serial: 'post-1', description: '', is_censored: false,
        elements_count: 8, max_elements: 8,
      },
      elements: Array.from({ length: 8 }, (_, index) => ({
        id: index + 1, title: `選項 ${index + 1}`, type: 'image',
        source_url: `https://example.test/${index + 1}.webp`, thumb_url: null,
        mediumthumb_url: null, lowthumb_url: null,
        video_start_second: null, video_end_second: null, video_source: null,
        video_id: null, video_duration_second: null,
      })),
    })
    const router = createRouter({
      history: createMemoryHistory(),
      routes: [
        { path: '/zh-tw/g/:serial', name: 'game-localized', component: GameView },
        { path: '/zh-tw/r/:serial', name: 'rank-localized', component: GameView },
      ],
    })
    await router.push('/zh-tw/g/post-1')
    await router.isReady()
    const wrapper = mount(App, {
      global: {
        plugins: [router],
        stubs: { AppHeader: true, AppFooter: true, Transition: false },
      },
    })
    await flushPromises()

    await wrapper.get('.game-start-button').trigger('click')
    await flushPromises()
    expect(wrapper.find('.game-arena').exists()).toBe(true)
    // The control bar has no ranking link any more; the way through mid-game is the
    // restart dialog, which still offers one next to "continue".
    await wrapper.get('button[title="重整遊戲"]').trigger('click')
    await wrapper.get('.game-dialog-ranking-link').trigger('click')
    await flushPromises()

    expect(router.currentRoute.value.path).toBe('/zh-tw/r/post-1')
    expect(wrapper.find('.game-public-ranking-heading').exists()).toBe(true)
    expect(wrapper.get('.game-community-list').text()).toContain('第一名')

    let resolveSecondPage!: (page: RanksPage) => void
    navigationMocks.ranks.mockReturnValueOnce(new Promise<RanksPage>((resolve) => {
      resolveSecondPage = resolve
    }))
    await wrapper.findAll('.game-ranking-pagination button').at(-1)!.trigger('click')

    expect(wrapper.find('.game-community-list').exists()).toBe(true)
    expect(wrapper.get('.game-community-list').text()).toContain('第一名')
    expect(wrapper.get('.game-community-layout').attributes('aria-busy')).toBe('true')

    resolveSecondPage({
      items: [{
        rank: 2, win_rate: '70.0', date: '2026-08-03',
        element: {
          title: '第二名', type: 'image', id: 2, video_id: null,
          source_url: 'https://example.test/two.webp', video_source: null,
          thumb_url: 'https://example.test/two.webp',
          lowthumb_url: null, mediumthumb_url: null,
        },
      }],
      page: 2, per_page: 20, total: 2, total_pages: 2,
    })
    await flushPromises()
    expect(wrapper.get('.game-community-list').text()).toContain('第二名')
    wrapper.unmount()
  })
})

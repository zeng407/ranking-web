// @vitest-environment happy-dom

import { flushPromises, mount, type VueWrapper } from '@vue/test-utils'
import { nextTick } from 'vue'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import type { PublicPost } from '../services/publicContent'
import HomeView from './HomeView.vue'

const mocks = vi.hoisted(() => ({
  route: {
    path: '/zh-tw/',
    params: { locale: 'zh-tw' } as Record<string, string>,
    query: {} as Record<string, string>,
    meta: { viewKey: 'home' } as Record<string, string>,
  },
  setRoute: undefined as undefined | ((route: {
    path?: string
    params?: Record<string, string>
    query?: Record<string, string>
    meta?: Record<string, string>
  }) => void),
  push: vi.fn(),
  posts: vi.fn(),
  hotTags: vi.fn(),
  carouselItems: vi.fn(),
  champions: vi.fn(),
}))

vi.mock('vue-router', async (importOriginal) => {
  const { reactive } = await import('vue')
  const route = reactive(mocks.route)
  mocks.setRoute = (nextRoute) => Object.assign(route, nextRoute)
  return {
    ...await importOriginal<typeof import('vue-router')>(),
    useRoute: () => route,
    useRouter: () => ({ push: mocks.push }),
  }
})

vi.mock('../services/publicContent', async (importOriginal) => ({
  ...await importOriginal<typeof import('../services/publicContent')>(),
  createPublicContentService: () => ({
    posts: mocks.posts,
    hotTags: mocks.hotTags,
    carouselItems: mocks.carouselItems,
    champions: mocks.champions,
  }),
}))

function post(serial: string, title: string): PublicPost {
  const element = (id: number, name: string) => ({
    id,
    url: `https://example.test/${serial}-${id}.webp`,
    url2: null,
    title: name,
    type: 'image',
    video_source: null,
    previewable: true,
  })
  return {
    title,
    serial,
    is_private: false,
    description: '',
    element1: element(1, `${title} A`),
    element2: element(2, `${title} B`),
    created_at: '2026-08-03T00:00:00Z',
    updated_at: '2026-08-03T00:00:00Z',
    play_count: 10,
    elements_count: 8,
    tags: ['動漫'],
    is_censored: 0,
  }
}

function page(items: PublicPost[], current = 1, totalPages = 1) {
  return { items, page: current, per_page: 15, total: items.length, total_pages: totalPages }
}

async function mountHome(): Promise<VueWrapper> {
  const wrapper = mount(HomeView, {
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
  return wrapper
}

describe('HomeView regression behavior', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    mocks.route.path = '/zh-tw/'
    mocks.route.params = { locale: 'zh-tw' }
    mocks.route.query = {}
    mocks.route.meta = { viewKey: 'home' }
    mocks.posts.mockResolvedValue(page([post('post-1', '第一個投票')]))
    mocks.hotTags.mockResolvedValue({ 動漫: 12, 音樂: 8 })
    mocks.carouselItems.mockResolvedValue([])
    mocks.champions.mockResolvedValue([])
  })

  afterEach(() => {
    document.body.innerHTML = ''
  })

  it('searches with the original k query through SPA navigation', async () => {
    const wrapper = await mountHome()

    await wrapper.get('input[name="k"]').setValue('  貓咪  ')
    await wrapper.get('form[role="search"]').trigger('submit')

    expect(mocks.push).toHaveBeenCalledWith({ path: '/zh-tw/', query: { k: '貓咪' } })
    expect(mocks.posts).toHaveBeenCalledWith({
      sortBy: 'hot', range: 'week', keyword: '', page: 1,
    })
    wrapper.unmount()
  })

  it('turns a hot tag into the same searchable k query', async () => {
    const wrapper = await mountHome()

    await wrapper.findAll('.tag-chip').find((button) => button.text() === '#動漫')!.trigger('click')

    expect(mocks.push).toHaveBeenCalledWith({ path: '/zh-tw/', query: { k: '動漫' } })
    wrapper.unmount()
  })

  it('keeps hot/new switches as client-side router links', async () => {
    mocks.route.query = { k: '動漫' }
    const wrapper = await mountHome()
    const links = wrapper.findAll('.content-tabs a')

    expect(links[0]!.attributes('data-to')).toBe('/zh-tw/hot?k=%E5%8B%95%E6%BC%AB')
    expect(links[1]!.attributes('data-to')).toBe('/zh-tw/new?k=%E5%8B%95%E6%BC%AB')
    wrapper.unmount()
  })

  it('keeps the existing cards mounted while hot/new content is loading', async () => {
    let resolveLatest!: (value: ReturnType<typeof page>) => void
    mocks.posts
      .mockResolvedValueOnce(page([post('hot-post', '熱門投票')]))
      .mockReturnValueOnce(new Promise((resolve) => {
        resolveLatest = resolve
      }))
    const wrapper = await mountHome()

    mocks.setRoute!({ path: '/zh-tw/new' })
    await nextTick()

    expect(mocks.posts).toHaveBeenLastCalledWith({
      sortBy: 'new', range: 'week', keyword: '', page: 1,
    })
    expect(wrapper.findAll('.vote-card')).toHaveLength(1)
    expect(wrapper.text()).toContain('熱門投票')

    resolveLatest(page([post('new-post', '最新投票')]))
    await flushPromises()

    expect(wrapper.findAll('.vote-card')).toHaveLength(1)
    expect(wrapper.text()).toContain('最新投票')
    expect(wrapper.text()).not.toContain('熱門投票')
    wrapper.unmount()
  })

  it('ignores an outdated response after switching back to the loaded tab', async () => {
    let resolveLatest!: (value: ReturnType<typeof page>) => void
    mocks.posts
      .mockResolvedValueOnce(page([post('hot-post', '熱門投票')]))
      .mockReturnValueOnce(new Promise((resolve) => {
        resolveLatest = resolve
      }))
    const wrapper = await mountHome()

    mocks.setRoute!({ path: '/zh-tw/new' })
    await nextTick()
    mocks.setRoute!({ path: '/zh-tw/hot' })
    await nextTick()
    resolveLatest(page([post('new-post', '已過期的最新投票')]))
    await flushPromises()

    expect(wrapper.text()).toContain('熱門投票')
    expect(wrapper.text()).not.toContain('已過期的最新投票')
    expect(wrapper.get('.public-content-section').attributes('aria-busy')).toBe('false')
    wrapper.unmount()
  })

  it('opens both voting and ranking through client-side router links', async () => {
    const wrapper = await mountHome()
    const card = wrapper.get('.vote-card')

    expect(card.get('.vote-card-main').attributes('data-to')).toBe('/zh-tw/g/post-1')
    expect(card.findAll('.vote-card-actions a')[0]!.attributes('data-to')).toBe('/zh-tw/g/post-1')
    expect(card.findAll('.vote-card-actions a')[1]!.attributes('data-to')).toBe('/zh-tw/r/post-1')

    wrapper.unmount()
  })

  it('uses the default image for missing and failed vote-card previews', async () => {
    const item = post('broken-images', '圖片失效測試')
    item.element1.url = null
    item.element2.url = 'https://example.test/broken.webp'
    mocks.posts.mockResolvedValue(page([item]))
    const wrapper = await mountHome()
    const images = wrapper.findAll('.vote-card-media img')

    expect(images).toHaveLength(2)
    expect(images[0]!.attributes('src')).toBe('/image-placeholder.svg')
    await images[1]!.trigger('error')
    expect(images[1]!.attributes('src')).toBe('/image-placeholder.svg')
    expect(images[1]!.attributes('data-fallback-applied')).toBe('true')
    wrapper.unmount()
  })

  it('renders playable featured YouTube content and switches it with side arrows', async () => {
    mocks.carouselItems.mockResolvedValue([
      {
        title: '精選一', description: null, image_url: null, video_url: null,
        position: 1, type: 'video', video_source: 'youtube', video_id: 'video-1', video_start_second: '10',
      },
      {
        title: '精選二', description: null, image_url: 'https://example.test/two.webp', video_url: null,
        position: 2, type: 'image', video_source: null, video_id: null, video_start_second: null,
      },
    ])
    const wrapper = await mountHome()

    expect(wrapper.get('.highlight-carousel-card iframe').attributes('src')).toContain('youtube-nocookie.com/embed/video-1')
    await wrapper.get('.highlight-carousel-arrow.is-next').trigger('click')
    expect(wrapper.get('.highlight-carousel-card img').attributes('src')).toBe('https://example.test/two.webp')
    wrapper.unmount()
  })

  it('shows both winner and loser in the recent champion marquee', async () => {
    mocks.champions.mockResolvedValue([{
      post_title: '冠軍賽', post_serial: 'post-1', datetime: '2026-08-03T00:00:00Z',
      thumb_url: null, key: 'champion-1',
      left: { name: '勝者', thumb_url: 'https://example.test/winner.webp', is_winner: true },
      right: { name: '敗者', thumb_url: 'https://example.test/loser.webp', is_winner: false },
    }])
    const wrapper = await mountHome()

    const firstCard = wrapper.get('.champion-marquee-track > a')
    expect(firstCard.text()).toContain('贏家勝者')
    expect(firstCard.text()).toContain('輸家敗者')
    wrapper.unmount()
  })

  it('loads another page without duplicating an existing post', async () => {
    mocks.posts
      .mockResolvedValueOnce(page([post('post-1', '第一個投票')], 1, 2))
      .mockResolvedValueOnce({
        ...page([post('post-1', '第一個投票'), post('post-2', '第二個投票')], 2, 2),
        total: 2,
      })
    const wrapper = await mountHome()

    expect(wrapper.findAll('.vote-card')).toHaveLength(1)
    expect(wrapper.get('.load-more-posts button').text()).toBe('載入更多')
    await wrapper.get('.load-more-posts button').trigger('click')
    await flushPromises()

    expect(mocks.posts).toHaveBeenLastCalledWith({
      sortBy: 'hot', range: 'week', keyword: '', page: 2,
    })
    expect(wrapper.findAll('.vote-card')).toHaveLength(2)
    expect(wrapper.text()).toContain('第二個投票')
    wrapper.unmount()
  })

  // These two stay last on purpose. HomeView keeps extrasRequestStarted,
  // lastLoadedRequestKey and pendingResetRequestKey at module scope, so every
  // mount carries state into the next test and the hot/new deferred-response
  // tests above fail if anything mounts before them.
  it('gives the page exactly one h1, holding the browse heading', async () => {
    const wrapper = await mountHome()

    // The browse heading renders at 56px and is what the page is about, but it
    // used to be an h2 while the page carried no h1 at all.
    const headings = wrapper.findAll('h1')
    expect(headings).toHaveLength(1)
    expect(headings[0]!.text()).toBe('本週熱門投票')
    wrapper.unmount()
  })

  it('marks opening a vote as the primary card action and ranking as secondary', async () => {
    const wrapper = await mountHome()

    // Both actions used to be identical 50/50 links, so the card never said
    // which of the two it wanted the reader to take.
    const primary = wrapper.get('.vote-card-actions .vote-card-action-primary')
    const quiet = wrapper.get('.vote-card-actions .vote-card-action-quiet')

    expect(primary.attributes('data-to')).toBe('/zh-tw/g/post-1')
    expect(quiet.attributes('data-to')).toBe('/zh-tw/r/post-1')
    wrapper.unmount()
  })
})

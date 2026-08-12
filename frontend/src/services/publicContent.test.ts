import { describe, expect, it, vi } from 'vitest'

import { createAPIClient } from '../lib/api'
import {
  carouselYoutubeEmbedURL,
  createPublicContentService,
  preferredElementImage,
  type CarouselItem,
  type PostElement,
} from './publicContent'

describe('public content service', () => {
  it('builds the versioned posts query without leaking undefined values', async () => {
    const fetchMock = vi.fn<typeof fetch>().mockResolvedValue(
      new Response(JSON.stringify({
        data: { items: [], page: 2, per_page: 10, total: 0, total_pages: 0 },
        meta: { request_id: 'request-1' },
      }), { status: 200, headers: { 'Content-Type': 'application/json' } }),
    )
    const service = createPublicContentService(createAPIClient('/api/v1', fetchMock))

    await service.posts({ sortBy: 'new', range: 'month', page: 2, perPage: 10 })

    expect(fetchMock).toHaveBeenCalledWith(
      '/api/v1/posts?sort_by=new&page=2&per_page=10',
      expect.objectContaining({ method: 'GET', credentials: 'omit' }),
    )
  })

  it('uses the original get-posts k parameter for keyword and tag searches', async () => {
    const fetchMock = vi.fn<typeof fetch>().mockResolvedValue(
      new Response(JSON.stringify({
        data: { items: [], page: 1, per_page: 15, total: 0, total_pages: 0 },
      }), { status: 200, headers: { 'Content-Type': 'application/json' } }),
    )
    const service = createPublicContentService(createAPIClient('/api/v1', fetchMock))

    await service.posts({ keyword: '#動漫 二選一' })

    expect(fetchMock).toHaveBeenCalledWith(
      '/api/v1/posts?sort_by=hot&range=week&page=1&per_page=15&k=%23%E5%8B%95%E6%BC%AB+%E4%BA%8C%E9%81%B8%E4%B8%80',
      expect.objectContaining({ method: 'GET', credentials: 'omit' }),
    )
  })

  it('prefers the resized image when one is available', () => {
    const element = {
      id: 1,
      url: 'https://example.test/source.jpg',
      url2: 'https://example.test/medium.jpg',
      title: 'candidate',
      type: 'image',
      video_source: null,
      previewable: true,
    } satisfies PostElement

    expect(preferredElementImage(element)).toBe('https://example.test/medium.jpg')
    expect(preferredElementImage({ ...element, url2: null })).toBe('https://example.test/source.jpg')
  })

  it('builds a paginated cumulative-rank query by default', async () => {
    const fetchMock = vi.fn<typeof fetch>().mockResolvedValue(
      new Response(JSON.stringify({
        data: { items: [], page: 3, per_page: 24, total: 0, total_pages: 0 },
      }), { status: 200, headers: { 'Content-Type': 'application/json' } }),
    )
    const service = createPublicContentService(createAPIClient('/api/v1', fetchMock))

    await service.ranks('post-serial', 'cumulative', 3, 24)

    expect(fetchMock).toHaveBeenCalledWith(
      '/api/v1/ranks?post_serial=post-serial&group=cumulative&page=3&per_page=24',
      expect.objectContaining({ method: 'GET', credentials: 'omit' }),
    )
  })

  it('requests the recent one-thousand-vote ranking group', async () => {
    const fetchMock = vi.fn<typeof fetch>().mockResolvedValue(
      new Response(JSON.stringify({
        data: { items: [], page: 1, per_page: 20, total: 0, total_pages: 0 },
      }), { status: 200, headers: { 'Content-Type': 'application/json' } }),
    )
    const service = createPublicContentService(createAPIClient('/api/v1', fetchMock))

    await service.ranks('post-serial', 'recent_1000')

    expect(fetchMock).toHaveBeenCalledWith(
      '/api/v1/ranks?post_serial=post-serial&group=recent_1000&page=1&per_page=20',
      expect.objectContaining({ method: 'GET', credentials: 'omit' }),
    )
  })

  it('builds a privacy-enhanced YouTube carousel URL with its configured start time', () => {
    const item = {
      title: 'Featured video', description: null, image_url: null, video_url: null,
      position: 1, type: 'video', video_source: 'youtube', video_id: 'video-id',
      video_start_second: '38',
    } satisfies CarouselItem

    expect(carouselYoutubeEmbedURL(item)).toBe(
      'https://www.youtube-nocookie.com/embed/video-id?controls=1&playsinline=1&rel=0&start=38',
    )
    expect(carouselYoutubeEmbedURL({ ...item, video_source: 'twitch_video' })).toBeNull()
  })
})

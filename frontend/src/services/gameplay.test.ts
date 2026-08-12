import { describe, expect, it } from 'vitest'

import { gamePreviewImage, preferredGameImage, youtubeEmbedURL, type GameElement } from './gameplay'

describe('gameplay service', () => {
  it('prefers the smallest S3 thumbnail, then larger ones, then the source', () => {
    const element = {
      id: 1,
      source_url: 'https://file.2pick.app/source.webp',
      thumb_url: 'https://file.2pick.app/thumb.webp',
      mediumthumb_url: null,
      lowthumb_url: null,
      title: 'Candidate',
      type: 'image',
      video_start_second: null,
      video_end_second: null,
      video_source: null,
      video_id: null,
      video_duration_second: null,
    } satisfies GameElement

    expect(preferredGameImage(element)).toBe('https://file.2pick.app/thumb.webp')
    expect(preferredGameImage({ ...element, lowthumb_url: 'https://file.2pick.app/low.webp' }))
      .toBe('https://file.2pick.app/low.webp')
    expect(preferredGameImage({ ...element, mediumthumb_url: 'https://file.2pick.app/medium.webp' }))
      .toBe('https://file.2pick.app/medium.webp')
    // Falls through to the source only when every derivative is missing.
    expect(preferredGameImage({ ...element, thumb_url: null })).toBe('https://file.2pick.app/source.webp')
  })

  it('builds a muted JS-controlled YouTube player for hover switching', () => {
    const element = {
      id: 2,
      source_url: null,
      thumb_url: null,
      mediumthumb_url: null,
      lowthumb_url: null,
      title: 'Video',
      type: 'video',
      video_start_second: 12,
      video_end_second: 45,
      video_source: 'youtube_embed',
      video_id: 'video-id',
      video_duration_second: 33,
    } satisfies GameElement

    const url = youtubeEmbedURL(element)
    expect(url).toContain('autoplay=1')
    expect(url).toContain('mute=1')
    expect(url).toContain('enablejsapi=1')
    expect(url).toContain('playlist=video-id')
    expect(gamePreviewImage(element)).toBe('https://i.ytimg.com/vi/video-id/hqdefault.jpg')
  })
})

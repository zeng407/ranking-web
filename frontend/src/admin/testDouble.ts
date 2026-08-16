import { vi } from 'vitest'

import type {
  AdminOutcome,
  AdminPost,
  AdminPostDetail,
  AdminService,
  AdminUser,
  Announcement,
  CarouselItem,
} from '../services/admin'

/**
 * A stand-in for the admin service, for the screen tests.
 *
 * Shared rather than repeated in each test file: the interface has twenty methods, and a
 * per-file copy would mean four places to edit whenever one is added — which is how a fake
 * ends up answering a shape the real service stopped returning.
 *
 * Imported only by tests. Nothing in the bundle reaches it.
 */

export function ok<T>(value: T): AdminOutcome<T> {
  return { ok: true, value }
}

export const adminPost: AdminPost = {
  serial: 'abcdefgh',
  title: 'a title',
  description: 'a description',
  access_policy: 'public',
  is_censored: false,
  play_count: 500,
  owner: { id: 7, name: 'an author', email: 'author@example.test' },
}

export const adminPostDetail: AdminPostDetail = {
  serial: 'abcdefgh',
  title: 'a title',
  description: 'a description',
  access_policy: 'public',
  has_password: false,
  tags: ['cats', 'dogs'],
  play_count: 500,
  this_week_play_count: 20,
  last_week_play_count: 30,
}

export const adminUser: AdminUser = {
  id: 42,
  name: 'a member',
  email: 'member@example.test',
  avatar_url: '',
  roles: [],
  post_count: 3,
}

export const carouselItem: CarouselItem = {
  id: 3,
  position: 1,
  type: 'image',
  title: 'a slide',
  description: 'what it shows',
  image_url: 'https://file.2pick.test/a.png',
  video_url: '',
  video_start_second: null,
  video_end_second: null,
  is_active: true,
}

export const announcement: Announcement = {
  id: 'an-announcement',
  content: 'the site is up',
  image_url: '',
  created_at: '2026-08-13T00:00:00Z',
  keep_minutes: 60,
}

export function fakeAdminService(overrides: Partial<AdminService> = {}): AdminService {
  return {
    posts: vi.fn().mockResolvedValue(ok({ posts: [adminPost], total: 1, page: 1, per_page: 20 })),
    post: vi.fn().mockResolvedValue(ok(adminPostDetail)),
    updatePost: vi.fn().mockResolvedValue(ok(adminPostDetail)),
    deletePost: vi.fn().mockResolvedValue(ok(undefined)),
    elements: vi.fn().mockResolvedValue(ok({ elements: [], total: 0, page: 1, per_page: 20 })),
    updateElement: vi.fn().mockResolvedValue(ok(undefined)),
    deleteElement: vi.fn().mockResolvedValue(ok(undefined)),

    users: vi.fn().mockResolvedValue(ok({ users: [adminUser], total: 1, page: 1, per_page: 20 })),
    banUser: vi.fn().mockResolvedValue(ok(undefined)),
    unbanUser: vi.fn().mockResolvedValue(ok(undefined)),

    carouselItems: vi.fn().mockResolvedValue(ok([carouselItem])),
    createCarouselItem: vi.fn().mockResolvedValue(ok(carouselItem)),
    updateCarouselItem: vi.fn().mockResolvedValue(ok(carouselItem)),
    deleteCarouselItem: vi.fn().mockResolvedValue(ok(undefined)),
    reorderCarouselItems: vi.fn().mockResolvedValue(ok([carouselItem])),

    announcement: vi.fn().mockResolvedValue(ok(null)),
    publishAnnouncement: vi.fn().mockResolvedValue(ok(announcement)),

    grantAssetPass: vi.fn().mockResolvedValue(ok(undefined)),
    revokeAssetPass: vi.fn().mockResolvedValue(ok(undefined)),
    ...overrides,
  } as AdminService
}

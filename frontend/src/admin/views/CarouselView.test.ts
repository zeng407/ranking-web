// @vitest-environment happy-dom

import { afterEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'

import CarouselView from './CarouselView.vue'
import { carouselItem, fakeAdminService, ok } from '../testDouble'
import type { AdminService, CarouselItem } from '../../services/admin'

const second: CarouselItem = { ...carouselItem, id: 4, position: 2, title: 'another slide' }
const third: CarouselItem = { ...carouselItem, id: 5, position: 3, title: 'a third slide' }

function threeItems() {
  return ok([carouselItem, second, third])
}

async function render(service: AdminService) {
  const wrapper = mount(CarouselView, { props: { service } })
  await flushPromises()
  return wrapper
}

function buttonWith(wrapper: Awaited<ReturnType<typeof render>>, label: string) {
  const button = wrapper.findAll('button').find((candidate) => candidate.text() === label)
  if (!button) throw new Error(`no button labelled ${label}`)
  return button
}

/** Rows in the body, skipping the "nothing here" row the template renders when empty. */
function rows(wrapper: Awaited<ReturnType<typeof render>>) {
  return wrapper.findAll('tbody tr')
}

async function dragRow(wrapper: Awaited<ReturnType<typeof render>>, from: number, to: number) {
  await rows(wrapper)[from]?.find('.admin-drag-handle').trigger('dragstart')
  await rows(wrapper)[to]?.trigger('drop')
  await flushPromises()
}

afterEach(() => {
  vi.unstubAllGlobals()
})

describe('CarouselView', () => {
  it('lists the slides in stored order', async () => {
    const wrapper = await render(fakeAdminService({ carouselItems: vi.fn().mockResolvedValue(threeItems()) }))

    expect(rows(wrapper)).toHaveLength(3)
    // Titles are editable in place, so they live in the inputs rather than in the text.
    expect(rows(wrapper).map((row) => (row.find('input[type="text"]').element as HTMLInputElement).value))
      .toEqual(['a slide', 'another slide', 'a third slide'])
    expect(wrapper.text()).toContain('啟用')
  })

  it('creates a slide from the form and clears it', async () => {
    const service = fakeAdminService()
    const wrapper = await render(service)

    const [title, description] = wrapper.findAll('.admin-card input[type="text"]')
    await title?.setValue('  a new slide  ')
    await description?.setValue('what it shows')
    await wrapper.find('.admin-card input[type="url"]').setValue('https://file.2pick.test/b.png')
    await buttonWith(wrapper, '新增').trigger('click')
    await flushPromises()

    expect(service.createCarouselItem).toHaveBeenCalledWith({
      type: 'image',
      title: 'a new slide',
      description: 'what it shows',
      image_url: 'https://file.2pick.test/b.png',
      video_url: '',
      video_start_second: null,
      video_end_second: null,
      is_active: true,
    })
    expect((title?.element as HTMLInputElement).value).toBe('')
  })

  // One request for the whole order, and the answer becomes the list: the original sent one
  // request per slide, so a failure part-way left an order nobody chose.
  it('sends the entire order once when a row is dragged', async () => {
    const service = fakeAdminService({
      carouselItems: vi.fn().mockResolvedValue(threeItems()),
      reorderCarouselItems: vi.fn().mockResolvedValue(ok([second, third, carouselItem])),
    })
    const wrapper = await render(service)

    await dragRow(wrapper, 0, 2)

    expect(service.reorderCarouselItems).toHaveBeenCalledTimes(1)
    expect(service.reorderCarouselItems).toHaveBeenCalledWith([
      { id: 4, position: 1 }, { id: 5, position: 2 }, { id: 3, position: 3 },
    ])
    expect(wrapper.text()).toContain('已更新順序。')
  })

  it('sends nothing when a row is dropped on itself', async () => {
    const service = fakeAdminService({ carouselItems: vi.fn().mockResolvedValue(threeItems()) })
    const wrapper = await render(service)

    await dragRow(wrapper, 1, 1)

    expect(service.reorderCarouselItems).not.toHaveBeenCalled()
  })

  // What the drag showed locally is not what the server holds, so a failure reloads rather
  // than leaving the moderator looking at an order that was never stored.
  it('reloads the stored order when the reorder fails', async () => {
    const service = fakeAdminService({
      carouselItems: vi.fn().mockResolvedValue(threeItems()),
      reorderCarouselItems: vi.fn().mockResolvedValue({ ok: false, kind: 'unavailable' }),
    })
    const wrapper = await render(service)

    await dragRow(wrapper, 0, 2)

    expect(service.carouselItems).toHaveBeenCalledTimes(2)
    expect(wrapper.find('.admin-alert.error').text()).toContain('伺服器暫時無法處理')
  })

  it('saves an edited slide and toggles one off', async () => {
    const service = fakeAdminService()
    const wrapper = await render(service)

    await rows(wrapper)[0]?.find('input[type="text"]').setValue('  a renamed slide  ')
    await buttonWith(wrapper, '儲存').trigger('click')
    await flushPromises()

    expect(service.updateCarouselItem).toHaveBeenLastCalledWith(3, {
      title: 'a renamed slide',
      description: 'what it shows',
      video_start_second: null,
      video_end_second: null,
    })

    await buttonWith(wrapper, '停用').trigger('click')
    await flushPromises()

    expect(service.updateCarouselItem).toHaveBeenLastCalledWith(3, { is_active: false })
  })

  it('asks before deleting a slide', async () => {
    const service = fakeAdminService()
    const wrapper = await render(service)
    vi.stubGlobal('confirm', vi.fn().mockReturnValue(false))

    await buttonWith(wrapper, '刪除').trigger('click')
    await flushPromises()

    expect(service.deleteCarouselItem).not.toHaveBeenCalled()
  })
})

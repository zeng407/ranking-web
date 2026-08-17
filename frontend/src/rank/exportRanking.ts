export interface RankingExportItem {
  rank: number
  title: string
  imageUrl: string | null
}

export interface PersonalRankingExport {
  imageUrl: string
  blob: Blob
  filename: string
  text: string
}

export interface RankingExportSlot {
  rank: number
  column: number
  row: number
  columnSpan: number
}

const canvasWidth = 1260
const sidePadding = 30
const gap = 18
const headerHeight = 150
const heroHeight = 620
const itemHeight = 310

export function rankingExportLayout(count: number): RankingExportSlot[] {
  const total = Math.min(10, Math.max(0, count))
  return Array.from({ length: total }, (_, index) => index === 0
    ? { rank: 1, column: 0, row: 0, columnSpan: 3 }
    : { rank: index + 1, column: (index - 1) % 3, row: Math.floor((index - 1) / 3) + 1, columnSpan: 1 })
}

/**
 * Truncates to a byte budget without splitting a character.
 *
 * Filesystem name limits are in bytes (255 on ext4 and APFS), and a CJK title is
 * three bytes per character — 80 characters of Chinese plus the prefix and suffix
 * is already over the limit. Slicing by code unit would also cut an emoji in half
 * and leave a lone surrogate in the name.
 */
function truncateToBytes(value: string, limit: number): string {
  let bytes = 0
  let result = ''
  for (const character of value) {
    const size = new TextEncoder().encode(character).length
    if (bytes + size > limit) break
    bytes += size
    result += character
  }
  return result
}

export function rankingExportFilename(title: string): string {
  const cleaned = title
    .normalize('NFKC')
    // Path separators, the characters Windows reserves, and control characters.
    .replace(/[\\/:*?"<>|：\u0000-\u001f\u007f]+/g, '-')
    .replace(/\s+/g, '-')
    .replace(/-+/g, '-')
    .replace(/^-|-$/g, '')
  // Trailing dots and spaces are dropped by Windows, and a name that is only
  // dots is not a name at all.
  const safeTitle = truncateToBytes(cleaned, 150).replace(/^-|[-.\s]+$/g, '') || 'ranking'
  // CON, PRN, AUX, NUL, COM1-9 and LPT1-9 cannot be filenames on Windows even
  // with an extension; the prefix below already keeps us clear of them.
  return `2pick-${safeTitle}-top10.png`
}

export function rankingExportText(sourceItems: RankingExportItem[]): string {
  return sourceItems
    .slice(0, 10)
    .map((item) => `#${item.rank} ${item.title}`)
    .join('\n')
}

export async function createPersonalRankingExport(
  title: string,
  sourceItems: RankingExportItem[],
): Promise<PersonalRankingExport> {
  const items = sourceItems.slice(0, 10)
  if (!items.length) throw new Error('ranking is empty')
  const slots = rankingExportLayout(items.length)
  const rows = Math.max(...slots.map((slot) => slot.row))
  const height = headerHeight + sidePadding + heroHeight + (rows * (itemHeight + gap)) + sidePadding
  const canvas = document.createElement('canvas')
  canvas.width = canvasWidth
  canvas.height = height
  const context = canvas.getContext('2d')
  if (!context) throw new Error('canvas is unavailable')

  const background = context.createLinearGradient(0, 0, canvasWidth, height)
  background.addColorStop(0, '#111827')
  background.addColorStop(1, '#09090b')
  context.fillStyle = background
  context.fillRect(0, 0, canvasWidth, height)
  context.fillStyle = '#ffffff'
  context.font = '800 46px system-ui, sans-serif'
  context.fillText('2PICK', sidePadding, 66)
  context.font = '700 34px system-ui, sans-serif'
  context.fillText(truncate(title, 38), sidePadding, 116)

  for (const [index, slot] of slots.entries()) {
    const item = items[index]!
    const columnWidth = (canvasWidth - sidePadding * 2 - gap * 2) / 3
    const x = sidePadding + slot.column * (columnWidth + gap)
    const y = headerHeight + sidePadding + (slot.row === 0 ? 0 : heroHeight + gap + (slot.row - 1) * (itemHeight + gap))
    const width = slot.columnSpan === 3 ? canvasWidth - sidePadding * 2 : columnWidth
    const boxHeight = slot.row === 0 ? heroHeight : itemHeight
    await drawRankingItem(context, item, x, y, width, boxHeight)
  }

  const blob = await new Promise<Blob>((resolve, reject) => {
    canvas.toBlob((value) => value ? resolve(value) : reject(new Error('could not encode ranking image')), 'image/png')
  })
  return {
    imageUrl: URL.createObjectURL(blob),
    blob,
    filename: rankingExportFilename(title),
    text: rankingExportText(items),
  }
}

export function downloadPersonalRankingExport(result: PersonalRankingExport): void {
  const anchor = document.createElement('a')
  anchor.href = result.imageUrl
  anchor.download = result.filename
  document.body.appendChild(anchor)
  anchor.click()
  anchor.remove()
}

export function disposePersonalRankingExport(result: PersonalRankingExport): void {
  URL.revokeObjectURL(result.imageUrl)
}

export async function exportPersonalRankingPNG(title: string, sourceItems: RankingExportItem[]): Promise<void> {
  const result = await createPersonalRankingExport(title, sourceItems)
  downloadPersonalRankingExport(result)
  window.setTimeout(() => disposePersonalRankingExport(result), 20_000)
}

async function drawRankingItem(
  context: CanvasRenderingContext2D,
  item: RankingExportItem,
  x: number,
  y: number,
  width: number,
  height: number,
): Promise<void> {
  context.save()
  roundedRect(context, x, y, width, height, 24)
  context.clip()
  context.fillStyle = item.rank === 1 ? '#7c2d12' : '#1f2937'
  context.fillRect(x, y, width, height)
  const image = item.imageUrl ? await loadExportImage(item.imageUrl) : null
  if (image) drawCover(context, image, x, y, width, height)
  const shade = context.createLinearGradient(0, y, 0, y + height)
  shade.addColorStop(0, 'rgba(0,0,0,.08)')
  shade.addColorStop(1, 'rgba(0,0,0,.82)')
  context.fillStyle = shade
  context.fillRect(x, y, width, height)
  context.restore()

  context.fillStyle = '#ffffff'
  context.font = item.rank === 1 ? '900 70px system-ui, sans-serif' : '900 42px system-ui, sans-serif'
  context.fillText(`#${item.rank}`, x + 24, y + (item.rank === 1 ? 82 : 54))
  context.font = item.rank === 1 ? '800 46px system-ui, sans-serif' : '750 28px system-ui, sans-serif'
  context.fillText(truncate(item.title, item.rank === 1 ? 34 : 16), x + 24, y + height - 28)
}

function loadExportImage(url: string): Promise<HTMLImageElement | null> {
  return new Promise((resolve) => {
    const image = new Image()
    const timeout = window.setTimeout(() => resolve(null), 8_000)
    image.crossOrigin = 'anonymous'
    image.onload = () => {
      window.clearTimeout(timeout)
      resolve(image)
    }
    image.onerror = () => {
      window.clearTimeout(timeout)
      resolve(null)
    }
    image.src = url
  })
}

function drawCover(context: CanvasRenderingContext2D, image: HTMLImageElement, x: number, y: number, width: number, height: number): void {
  const scale = Math.max(width / image.naturalWidth, height / image.naturalHeight)
  const sourceWidth = width / scale
  const sourceHeight = height / scale
  context.drawImage(
    image,
    (image.naturalWidth - sourceWidth) / 2,
    (image.naturalHeight - sourceHeight) / 2,
    sourceWidth,
    sourceHeight,
    x,
    y,
    width,
    height,
  )
}

function roundedRect(context: CanvasRenderingContext2D, x: number, y: number, width: number, height: number, radius: number): void {
  context.beginPath()
  context.roundRect(x, y, width, height, radius)
  context.closePath()
}

function truncate(value: string, length: number): string {
  return value.length > length ? `${value.slice(0, length - 1)}…` : value
}

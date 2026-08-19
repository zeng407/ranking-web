import Viewer from 'viewerjs'
import 'viewerjs/dist/viewer.css'

export interface ViewerPicture {
  /** Full-size image URL. */
  image: string
  /** Caption shown above the picture, e.g. "#13 Assam milk tea". */
  title: string
}

let current: Viewer | null = null

/**
 * Open one picture full screen with viewer.js: zoom, rotate, drag, keyboard.
 *
 * The viewer owns its own overlay, so there is no markup for it in the views.
 * Its backdrop closes on a click anywhere outside the picture, which a plain
 * overlay could not do: an `<img>` sized to the viewport keeps its transparent
 * letterbox area clickable, and those clicks never reach the backdrop.
 */
export function openImageViewer(picture: ViewerPicture): void {
  closeImageViewer()

  // A detached host: viewer.js only reads the images out of it and appends its
  // own overlay to the body, so the host never has to be in the document.
  const host = document.createElement('div')
  const image = document.createElement('img')
  image.src = picture.image
  image.alt = picture.title
  host.appendChild(image)

  const viewer = new Viewer(host, {
    // No filmstrip or slideshow: a single picture is opened at a time.
    navbar: false,
    loop: false,
    toolbar: {
      zoomIn: true,
      zoomOut: true,
      oneToOne: true,
      reset: true,
      rotateLeft: true,
      rotateRight: true,
    },
    title: () => picture.title,
    viewed() {
      // viewer.js never opens an image above its natural size, and some posts
      // store a source file smaller than the thumbnail the card already shows:
      // opening one then made the picture shrink. Scale it up to the frame the
      // page used before instead, which is what the list card promised.
      if (!image.naturalWidth || !image.naturalHeight) return
      const cover = Math.min(
        Math.min(window.innerWidth * 0.92, 960) / image.naturalWidth,
        Math.min(window.innerHeight * 0.78, 672) / image.naturalHeight,
      )
      if (cover > 1) viewer.zoomTo(cover)
    },
    hidden() {
      // destroy() dispatches this event too; closeImageViewer() clears `current`
      // before destroying so that path returns here instead of recursing.
      if (current !== viewer) return
      current = null
      viewer.destroy()
    },
  })

  current = viewer
  viewer.show()
}

/** Close the picture opened by {@link openImageViewer}, if one is open. */
export function closeImageViewer(): void {
  if (!current) return
  const viewer = current
  current = null
  viewer.destroy()
}

/** Whether a picture is open, so key handlers can leave the keys to the viewer. */
export function isImageViewerOpen(): boolean {
  return current !== null
}

export const TOGAWA_AD_SLOT_WIDTH = 300;
export const TOGAWA_AD_SLOT_GAP = 16;

// The grid itself is laid out by CSS (`repeat(auto-fit, 300px)`), so the row always
// expands horizontally even before this module runs. We mirror the same arithmetic
// here only to decide how many slots may be revealed: revealing more slots than the
// row can fit is what pushes the extras onto new rows and stacks them into a column.
export function togawaVisibleCount(width, slotCount) {
  const perRow = Math.floor((width + TOGAWA_AD_SLOT_GAP) / (TOGAWA_AD_SLOT_WIDTH + TOGAWA_AD_SLOT_GAP));
  return Math.max(0, Math.min(slotCount, perRow));
}

// Reveal exactly the slots that fit on one row and request the deferred ones once.
// Returns the number of visible slots, or -1 when the grid is not laid out yet.
export function applyTogawaLayout(grid, page, googletag, options = {}) {
  if (!grid || !googletag) return -1;

  const wrappers = options.wrappers || Array.from(grid.querySelectorAll(`.${page}-togawa-ad-slot`));
  const width = grid.clientWidth;
  // Hidden (v-show/v-cloak) or not measured yet: leave the markup untouched rather
  // than collapsing to a stale one-column layout we would never recompute.
  if (width < TOGAWA_AD_SLOT_WIDTH) return -1;

  const count = togawaVisibleCount(width, wrappers.length);
  const firstIsDeferred = options.displayFirstSlot === true;

  wrappers.forEach((wrapper, index) => {
    wrapper.style.display = index < count ? '' : 'none';
    // Without displayFirstSlot the first slot retains the partial's direct display path.
    if (index >= count || (index === 0 && !firstIsDeferred) || wrapper.dataset.gptRequested) return;

    wrapper.dataset.gptRequested = 'true';
    googletag.cmd.push(() => {
      if (!grid.isConnected || wrapper.style.display === 'none' || grid.clientWidth < TOGAWA_AD_SLOT_WIDTH) {
        delete wrapper.dataset.gptRequested;
        return;
      }
      googletag.display(wrapper.getAttribute(`data-${page}-togawa-slot`));
    });
  });

  return count;
}

// Call after Vue mounts so the observer owns the rendered grid, not the Blade DOM.
// ResizeObserver also fires when the grid goes from hidden to visible, so a grid
// measured at 0 width recovers on its own instead of staying a single column.
export default function observeTogawaAds(grid, page, googletag, options = {}) {
  if (!grid) return null;

  const wrappers = Array.from(grid.querySelectorAll(`.${page}-togawa-ad-slot`));
  const observer = new ResizeObserver(() => {
    applyTogawaLayout(grid, page, googletag, { ...options, wrappers });
  });
  observer.observe(grid);
  return observer;
}

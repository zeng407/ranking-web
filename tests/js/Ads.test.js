const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const { describe, test } = require('node:test');

function read(relativePath) {
  return fs.readFileSync(path.resolve(__dirname, '../..', relativePath), 'utf8');
}

describe('GAM custom ad integration', () => {
  const partial = read('resources/views/ads/gam_togawa_300x250.blade.php');

  test('keeps the reserved 300x250 space when an ad request fails', () => {
    assert.doesNotMatch(partial, /gam-togawa-ad d-flex/);
    assert.match(partial, /min-height: 250px/);
    assert.doesNotMatch(partial, /setProperty\('display', 'none'/);
    assert.doesNotMatch(partial, /setAttribute\('aria-hidden'/);
    assert.match(partial, /event\.isEmpty/);
  });

  test('retries a failed visible slot once after at least 30 seconds', () => {
    assert.match(partial, /MAX_RETRY_COUNT = 1/);
    assert.match(partial, /RETRY_DELAY_MS = 30 \* 1000/);
    assert.match(partial, /retryCount >= MAX_RETRY_COUNT/);
    assert.match(partial, /googletag\.pubads\(\)\.refresh\(\[slot\]\)/);
    assert.match(partial, /document\.visibilityState === 'visible'/);
    assert.match(partial, /addEventListener\('slotOnload'/);
  });

  test('waits until the slot participates in layout before displaying it', () => {
    const visibilityCheck = partial.indexOf('if (initialized || !isContainerVisible())');
    const defineSlot = partial.indexOf('googletag.defineSlot');
    const displaySlot = partial.indexOf('googletag.display(slotId)');

    assert.notEqual(visibilityCheck, -1);
    assert.match(partial, /container\.getClientRects\(\)\.length > 0/);
    assert.ok(visibilityCheck < defineSlot);
    assert.ok(defineSlot < displaySlot);
    assert.match(partial, /new MutationObserver\(initializeVisibleSlot\)/);
    assert.match(partial, /attributeFilter: \['class', 'style', 'v-cloak'\]/);
  });

  test('game page keeps the custom slot behind its game-ready visibility condition', () => {
    const gameView = read('resources/views/game/show.blade.php');

    assert.match(
      gameView,
      /<div v-show="game && !creatingGame && !finishingGame">\s*@include\('ads\.gam_togawa_300x250'\)/
    );
  });

  test('home page never refreshes the commented-out slot1', () => {
    const homeView = read('resources/views/home.blade.php');

    assert.doesNotMatch(homeView, /refresh\(\[slot1\]\)/);
    assert.match(homeView, /refresh\(\[slot2\]\)/);
  });
});

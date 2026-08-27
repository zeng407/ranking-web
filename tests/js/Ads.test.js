const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const { describe, test } = require('node:test');

function read(relativePath) {
  return fs.readFileSync(path.resolve(__dirname, '../..', relativePath), 'utf8');
}

describe('GAM custom ad integration', () => {
  const partial = read('resources/views/ads/gam_togawa_300x250.blade.php');

  test('collapses an empty slot despite Bootstrap display utilities', () => {
    assert.doesNotMatch(partial, /gam-togawa-ad d-flex/);
    assert.match(
      partial,
      /wrapper\.style\.setProperty\('display', 'none', 'important'\)/
    );
    assert.match(partial, /event\.isEmpty/);
  });

  test('defines and displays the custom slot directly without a visibility observer', () => {
    const containerLookup = partial.indexOf('document.getElementById(slotId)');
    const defineSlot = partial.indexOf('googletag.defineSlot');
    const displaySlot = partial.indexOf('googletag.display(slotId)');

    assert.notEqual(containerLookup, -1);
    assert.ok(containerLookup < defineSlot);
    assert.ok(defineSlot < displaySlot);
    assert.doesNotMatch(partial, /MutationObserver/);
    assert.doesNotMatch(partial, /getClientRects/);
  });

  test('game page defers the custom slot until a game has started', () => {
    const gameView = read('resources/views/game/show.blade.php');

    assert.match(
      gameView,
      /v-show="game && !creatingGame && !finishingGame">\s*@include\('ads\.gam_togawa_300x250', \['deferDisplay' => true\]\)/
    );
    assert.doesNotMatch(gameView, /ranking:game-ads-ready/);
    assert.match(partial, /\$togawaDeferDisplay = \$deferDisplay \?\? false/);
    assert.match(partial, /@if \(!\$togawaDeferDisplay\)\s*googletag\.display\(slotId\)/);
  });

  test('home page never refreshes the commented-out slot1', () => {
    const homeView = read('resources/views/home.blade.php');

    assert.doesNotMatch(homeView, /refresh\(\[slot1\]\)/);
    assert.match(homeView, /refresh\(\[slot2\]\)/);
  });

  test('rank page keeps independently displayed slots out of SRA', () => {
    const rankView = read('resources/views/game/rank.blade.php');

    assert.doesNotMatch(rankView, /singleRequest\s*:\s*true/);
    assert.match(rankView, /collapseDiv\s*:\s*'ON_NO_FILL'/);
    assert.doesNotMatch(rankView, /ranking:rank-ads-ready/);
    assert.match(rankView, /googletag\.display\('div-gpt-ad-1782518224225-0'\)/);
    assert.match(rankView, /@include\('ads\.gam_togawa_300x250'\)/);
  });
});

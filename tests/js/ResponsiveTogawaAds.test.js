const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const { test } = require('node:test');

function loadModule(ResizeObserver) {
  const source = fs.readFileSync(path.resolve(__dirname, '../../resources/js/responsiveTogawaAds.js'), 'utf8')
    .replace(/^export default function observeTogawaAds/m, 'function observeTogawaAds')
    .replace(/^export /gm, '');
  return new Function('ResizeObserver', `${source}\nreturn { observeTogawaAds, applyTogawaLayout, togawaVisibleCount };`)(ResizeObserver);
}

function makeGrid() {
  const wrappers = [1, 2, 3].map(id => ({
    dataset: {}, style: {}, getAttribute() { return `slot-${id}`; },
  }));
  return {
    wrappers,
    grid: { clientWidth: 0, isConnected: true, style: {}, querySelectorAll: () => wrappers },
  };
}

for (const page of ['home', 'rank']) {
  test(`${page}: hidden grid becomes three ads without a window resize`, () => {
    let notify;
    const { observeTogawaAds } = loadModule(class {
      constructor(callback) { notify = callback; }
      observe() {}
    });
    const { grid, wrappers } = makeGrid();
    const requests = [];
    const queue = [];
    observeTogawaAds(grid, page, { cmd: queue, display: id => requests.push(id) });
    notify();
    assert.equal(queue.length, 0, 'Hidden grids must not request additional ads');

    grid.clientWidth = 950;
    notify();
    notify();
    assert.equal(queue.length, 2, 'First slot is already handled by Blade; extras queue once');
    queue.splice(0).forEach(callback => callback());
    assert.deepEqual(requests, ['slot-2', 'slot-3']);

    for (const [width, count] of [[615, 1], [616, 2], [931, 2], [932, 3]]) {
      grid.clientWidth = width;
      notify();
      assert.equal(wrappers.filter(w => w.style.display !== 'none').length, count);
    }
    assert.equal(queue.length, 0, 'Layout changes must not refresh loaded ads');
  });
}

test('game: every slot is deferred, so the first one is requested too', () => {
  const { applyTogawaLayout } = loadModule(class { observe() {} });
  const { grid, wrappers } = makeGrid();
  const requests = [];
  const queue = [];
  const googletag = { cmd: queue, display: id => requests.push(id) };

  grid.clientWidth = 950;
  assert.equal(applyTogawaLayout(grid, 'game', googletag, { displayFirstSlot: true }), 3);
  queue.splice(0).forEach(callback => callback());
  assert.deepEqual(requests, ['slot-1', 'slot-2', 'slot-3']);
  assert.equal(wrappers.filter(w => w.style.display !== 'none').length, 3);
});

test('a hidden grid keeps its markup instead of collapsing to one column', () => {
  const { applyTogawaLayout } = loadModule(class { observe() {} });
  const { grid, wrappers } = makeGrid();
  const googletag = { cmd: [], display: () => {} };

  grid.clientWidth = 950;
  applyTogawaLayout(grid, 'game', googletag, { displayFirstSlot: true });
  assert.equal(wrappers.filter(w => w.style.display !== 'none').length, 3);

  // v-show hides the grid: measuring it now must not hide the slots we already filled.
  grid.clientWidth = 0;
  assert.equal(applyTogawaLayout(grid, 'game', googletag, { displayFirstSlot: true }), -1);
  assert.equal(wrappers.filter(w => w.style.display !== 'none').length, 3);
});

test('visible slots never exceed the columns the row can fit', () => {
  const { togawaVisibleCount } = loadModule(class { observe() {} });
  assert.equal(togawaVisibleCount(0, 3), 0);
  assert.equal(togawaVisibleCount(300, 3), 1);
  assert.equal(togawaVisibleCount(615, 3), 1);
  assert.equal(togawaVisibleCount(616, 3), 2);
  assert.equal(togawaVisibleCount(932, 3), 3);
  assert.equal(togawaVisibleCount(5000, 3), 3, 'Capped by the number of slots on the page');
});

test('each grid expands horizontally from CSS alone', () => {
  for (const view of ['resources/views/home.blade.php', 'resources/views/game/rank.blade.php', 'resources/views/game/show.blade.php']) {
    const markup = fs.readFileSync(path.resolve(__dirname, '../..', view), 'utf8');
    assert.match(markup, /grid-template-columns: repeat\(auto-fit, 300px\)/,
      `${view} must let CSS pack the slots into a row`);
  }
});

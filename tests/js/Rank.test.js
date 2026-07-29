const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const { describe, test, beforeEach } = require('node:test');
const { parseComponent } = require('vue-template-compiler');

function loadRankComponent() {
  const rankPath = path.resolve(__dirname, '../../resources/js/components/Rank.vue');
  const source = fs.readFileSync(rankPath, 'utf8');
  const descriptor = parseComponent(source);
  const script = descriptor.script.content
    .replace(/^import .*;$/gm, '')
    .replace('export default', 'module.exports =');

  assert.equal(/(^|\n)\s*import\s/.test(script), false, 'All Rank.vue imports must be stubbed');

  const rankModule = { exports: {} };
  const evaluate = new Function(
    'module',
    'exports',
    'Swal',
    'CountWords',
    'ICountUp',
    'Chart',
    'Vue',
    `${script}\n//# sourceURL=Rank.vue`
  );
  evaluate(rankModule, rankModule.exports, {}, {}, {}, {}, {});
  return rankModule.exports;
}

function createMemoryStorage() {
  const values = new Map();
  return {
    getItem(key) {
      return values.has(key) ? values.get(key) : null;
    },
    setItem(key, value) {
      values.set(key, String(value));
    },
  };
}

const Rank = loadRankComponent();

function createRankVm(overrides = {}) {
  const vm = Object.assign(
    Rank.data.call({}),
    {
      postSerial: 'post-serial',
      gameSerial: 'game-serial',
    },
    overrides
  );

  Object.entries(Rank.methods).forEach(([name, method]) => {
    vm[name] = method.bind(vm);
  });
  return vm;
}

describe('Rank.vue local results', { concurrency: false }, () => {
  beforeEach(() => {
    global.localStorage = createMemoryStorage();
  });

  test('loads the dedicated completed result before the legacy game state', () => {
    const completedElements = Array.from({ length: 12 }, (_, index) => ({
      id: index + 1,
      local_win_count: index,
    }));
    localStorage.setItem('gameresult_post-serial', JSON.stringify({
      gameSerial: 'game-serial',
      localElements: completedElements,
    }));
    localStorage.setItem('gamestate_post-serial', JSON.stringify({
      gameSerial: 'game-serial',
      localElements: [{ id: 999, local_win_count: 999 }],
    }));

    const vm = createRankVm();
    const ranks = vm.loadRankFromLocal();

    assert.equal(ranks.length, 10);
    assert.deepEqual(ranks.map(rank => rank.id), [12, 11, 10, 9, 8, 7, 6, 5, 4, 3]);
    assert.deepEqual(vm.localRanks, ranks);
  });

  test('falls back to the legacy game state when no completed result exists', () => {
    localStorage.setItem('gamestate_post-serial', JSON.stringify({
      gameSerial: 'game-serial',
      localElements: [
        { id: 1, local_win_count: 1 },
        { id: 2, local_win_count: 3 },
      ],
    }));

    const vm = createRankVm();
    const ranks = vm.loadRankFromLocal();

    assert.deepEqual(ranks.map(rank => rank.id), [2, 1]);
  });

  test('ignores local data belonging to another game serial', () => {
    localStorage.setItem('gameresult_post-serial', JSON.stringify({
      gameSerial: 'another-game',
      localElements: [{ id: 1, local_win_count: 10 }],
    }));

    const vm = createRankVm();

    assert.equal(vm.loadRankFromLocal(), false);
    assert.deepEqual(vm.localRanks, []);
  });
});

const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const { describe, test, beforeEach } = require('node:test');
const { parseComponent } = require('vue-template-compiler');

const swalCalls = [];
const Swal = {
  fire(options) {
    swalCalls.push(options);
    return Promise.resolve();
  },
};

function loadGameComponent() {
  const gamePath = path.resolve(__dirname, '../../resources/js/components/Game.vue');
  const source = fs.readFileSync(gamePath, 'utf8');
  const descriptor = parseComponent(source);
  let script = descriptor.script.content
    .replace(/^import .*;$/gm, '')
    .replace('export default', 'module.exports =');

  assert.equal(/(^|\n)\s*import\s/.test(script), false, 'All Game.vue imports must be stubbed');

  const gameModule = { exports: {} };
  const evaluate = new Function(
    'module',
    'exports',
    'Swal',
    'ICountUp',
    'QRCode',
    `${script}\n//# sourceURL=Game.vue`
  );
  evaluate(gameModule, gameModule.exports, Swal, {}, {});
  return gameModule.exports;
}

function createMemoryStorage() {
  const values = new Map();
  return {
    clear() {
      values.clear();
    },
    getItem(key) {
      return values.has(key) ? values.get(key) : null;
    },
    removeItem(key) {
      values.delete(key);
    },
    setItem(key, value) {
      values.set(key, String(value));
    },
  };
}

function deferred() {
  let resolve;
  let reject;
  const promise = new Promise((resolvePromise, rejectPromise) => {
    resolve = resolvePromise;
    reject = rejectPromise;
  });
  return { promise, resolve, reject };
}

function nextEventLoop() {
  return new Promise(resolve => setImmediate(resolve));
}

const Game = loadGameComponent();

function createGameVm(overrides = {}) {
  const vm = Object.assign(
    Game.data.call({
      propsGameRoomSerial: null,
      propsEnableClientMode: true,
    }),
    {
      postSerial: 'post-serial',
      gameSerial: 'game-serial',
      batchVoteEndpoint: '/api/game/batch-vote',
      elementsCount: 2,
      localElements: [
        {
          id: 1,
          local_eliminated: false,
          local_is_ready: true,
          local_played: 0,
          local_win_count: 0,
        },
        {
          id: 2,
          local_eliminated: false,
          local_is_ready: true,
          local_played: 0,
          local_win_count: 0,
        },
      ],
      clientState: {
        stage: 1,
        stageStartCount: 2,
        matchesInStage: 0,
        targetMatches: 1,
      },
      $cookies: {
        remove() {},
        set() {},
      },
      $t(value) {
        return value;
      },
    },
    overrides
  );

  Object.entries(Game.methods).forEach(([name, method]) => {
    vm[name] = method.bind(vm);
  });
  return vm;
}

describe('Game.vue batch vote', { concurrency: false }, () => {
  beforeEach(() => {
    global.localStorage = createMemoryStorage();
    global.axios = undefined;
    global.$ = () => ({ modal() {} });
    swalCalls.length = 0;
  });

  test('successful request removes only its snapshot and keeps votes appended in flight', async () => {
    const request = deferred();
    let requestData;
    global.axios = {
      post(_url, data) {
        requestData = data;
        return request.promise;
      },
    };

    const vm = createGameVm();
    vm.unsentVotes = [{ winner_id: 1, loser_id: 2 }];
    const saving = vm.performBatchVote();

    vm.unsentVotes.push({ winner_id: 1, loser_id: 3 });
    request.resolve({ data: { status: 'processing' } });
    await saving;

    assert.deepEqual(requestData.votes, [{ winner_id: 1, loser_id: 2 }]);
    assert.deepEqual(vm.unsentVotes, [{ winner_id: 1, loser_id: 3 }]);
  });

  test('partial and final saves are serialized without losing the final batch', async () => {
    const firstRequest = deferred();
    const requests = [];
    global.axios = {
      post(_url, data) {
        requests.push(data.votes);
        if (requests.length === 1) {
          return firstRequest.promise;
        }
        return Promise.resolve({ data: { status: 'end_game', data: null } });
      },
    };

    const vm = createGameVm();
    let handledFinalResponse = 0;
    vm.handleSendVote = response => {
      assert.equal(response.data.status, 'end_game');
      handledFinalResponse++;
    };
    vm.unsentVotes = [{ winner_id: 1, loser_id: 2 }];

    const partialSave = vm.sendPartialBatchVotes();
    vm.unsentVotes.push({ winner_id: 1, loser_id: 3 });
    vm.sendBatchVotes();

    assert.equal(requests.length, 1, 'Final batch must wait for the partial request');
    firstRequest.resolve({ data: { status: 'processing', data: {} } });
    await partialSave;
    await nextEventLoop();

    assert.deepEqual(requests, [
      [{ winner_id: 1, loser_id: 2 }],
      [{ winner_id: 1, loser_id: 3 }],
    ]);
    assert.equal(handledFinalResponse, 1);
    assert.deepEqual(vm.unsentVotes, []);
    assert.equal(vm.isBatchVoting, false);
    assert.equal(vm.isCloudSaving, false);
  });

  test('422 switches to persisted local-only mode and stops all later batch requests', async t => {
    t.mock.method(console, 'warn', () => {});
    let postCount = 0;
    global.axios = {
      post() {
        postCount++;
        return Promise.reject({ response: { status: 422, data: { message: 'conflict' } } });
      },
    };

    const vm = createGameVm();
    vm.unsentVotes = [{ winner_id: 1, loser_id: 2 }];
    await vm.sendPartialBatchVotes();

    assert.equal(vm.isLocalOnlyAfterBatchConflict, true);
    assert.equal(vm.isClientMode, true);
    assert.deepEqual(vm.unsentVotes, []);
    assert.equal(swalCalls.length, 0, '422 must not interrupt local voting with a retry dialog');

    const saved = JSON.parse(localStorage.getItem('gamestate_post-serial'));
    assert.equal(saved.localOnlyAfterBatchConflict, true);
    assert.deepEqual(saved.unsentVotes, []);

    vm.unsentVotes.push({ winner_id: 1, loser_id: 2 });
    vm.sendPartialBatchVotes();
    vm.sendBatchVotes();
    await nextEventLoop();
    assert.equal(postCount, 1, 'No batch request may be made after the first 422');
  });

  test('votes after a 422 update local results but never enter the upload queue', () => {
    const vm = createGameVm({ isLocalOnlyAfterBatchConflict: true });
    let animationResponse;
    vm.handleAnimationAfterVoted = response => {
      animationResponse = response;
    };

    vm.handleClientVote(vm.localElements[0], vm.localElements[1], 'left');

    assert.equal(vm.localElements[0].local_win_count, 1);
    assert.equal(vm.localElements[1].local_eliminated, true);
    assert.deepEqual(vm.localVotes, [{ winner_id: 1, loser_id: 2 }]);
    assert.deepEqual(vm.unsentVotes, []);
    assert.equal(animationResponse.data.status, 'end_game');
  });

  test('a final-batch 422 finishes locally and opens the ranking page', async t => {
    t.mock.method(console, 'warn', () => {});
    global.axios = {
      post() {
        return Promise.reject({ response: { status: 422 } });
      },
    };

    let cookieRemoved = 0;
    let localStateCleared = 0;
    let historyCleared = 0;
    let rankOpened = 0;
    const vm = createGameVm({
      localElements: [
        { id: 1, local_eliminated: false },
        { id: 2, local_eliminated: true },
      ],
      $cookies: {
        remove() {
          cookieRemoved++;
        },
      },
    });
    vm.recordMatchFromLastVote = () => {};
    vm.clearLocalStorage = () => {
      localStateCleared++;
    };
    vm.clearMatchHistory = () => {
      historyCleared++;
    };
    vm.showGameResult = () => {
      rankOpened++;
    };
    vm.unsentVotes = [{ winner_id: 1, loser_id: 2 }];

    await vm.sendBatchVotes();

    assert.equal(vm.isLocalOnlyAfterBatchConflict, true);
    assert.equal(vm.status, 'end_game');
    assert.equal(cookieRemoved, 1);
    assert.equal(localStateCleared, 1);
    assert.equal(historyCleared, 1);
    assert.equal(rankOpened, 1);
    assert.equal(swalCalls.length, 0);
  });

  test('a game already in local-only mode opens ranking without another request', () => {
    let postCount = 0;
    let rankOpened = 0;
    global.axios = {
      post() {
        postCount++;
        throw new Error('No request should be made in local-only mode');
      },
    };

    const vm = createGameVm({
      isLocalOnlyAfterBatchConflict: true,
      localElements: [
        { id: 1, local_eliminated: false },
        { id: 2, local_eliminated: true },
      ],
    });
    vm.recordMatchFromLastVote = () => {};
    vm.clearLocalStorage = () => {};
    vm.clearMatchHistory = () => {};
    vm.showGameResult = () => {
      rankOpened++;
    };

    vm.sendBatchVotes();

    assert.equal(postCount, 0);
    assert.equal(rankOpened, 1);
    assert.equal(vm.status, 'end_game');
  });

  test('local-only mode survives a page reload', () => {
    const writer = createGameVm();
    writer.switchToLocalOnlyAfterBatchConflict();

    const reader = createGameVm({
      gameSerial: null,
      isLocalOnlyAfterBatchConflict: false,
    });
    let nextLocalRoundCalls = 0;
    reader.nextLocalRound = () => {
      nextLocalRoundCalls++;
    };

    assert.equal(reader.loadFromLocalStorage(), true);
    assert.equal(reader.gameSerial, 'game-serial');
    assert.equal(reader.isLocalOnlyAfterBatchConflict, true);
    assert.equal(reader.isClientMode, true);
    assert.equal(nextLocalRoundCalls, 1);
  });

  test('continuing the same game keeps local-only progress even when cloud progress differs', () => {
    const writer = createGameVm();
    writer.switchToLocalOnlyAfterBatchConflict();

    let restoredLocal = 0;
    let fetchedRemoteElements = 0;
    const reader = createGameVm({
      gameSerial: null,
      userLastGame: {
        serial: 'game-serial',
        element_count: 2,
        vote_count: 1,
      },
    });
    reader.loadFromLocalStorage = () => {
      restoredLocal++;
      return true;
    };
    reader.fetchRemainingElements = () => {
      fetchedRemoteElements++;
    };
    reader.nextLocalRound = () => {};
    reader.loadMatchHistory = () => {};
    reader.resetTimer = () => {};
    reader.startTimer = () => {};

    reader.continueGame();

    assert.equal(restoredLocal, 1);
    assert.equal(fetchedRemoteElements, 1);
  });

  test('non-422 network errors retain votes and do not enable local-only mode', async t => {
    t.mock.method(console, 'error', () => {});
    global.axios = {
      post() {
        return Promise.reject(new Error('network unavailable'));
      },
    };

    const vm = createGameVm();
    vm.unsentVotes = [{ winner_id: 1, loser_id: 2 }];
    await vm.sendPartialBatchVotes();

    assert.equal(vm.isLocalOnlyAfterBatchConflict, false);
    assert.deepEqual(vm.unsentVotes, [{ winner_id: 1, loser_id: 2 }]);
    assert.equal(vm.batchVoteInterval, 21);
    assert.equal(vm.isBatchVoting, false);
    assert.equal(vm.isCloudSaving, false);
  });

  test('beforeunload persists pending votes without starting an unreliable request', () => {
    let postCount = 0;
    let saveCount = 0;
    global.axios = {
      post() {
        postCount++;
      },
    };

    const vm = createGameVm();
    vm.unsentVotes = [{ winner_id: 1, loser_id: 2 }];
    vm.saveToLocalStorage = () => {
      saveCount++;
    };
    vm.handleBeforeUnload();

    assert.equal(saveCount, 1);
    assert.equal(postCount, 0);
  });
});

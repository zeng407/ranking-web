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

function pendingVote(sequence, winnerId, loserId) {
  return {
    local_vote_id: `game-serial:${sequence}`,
    winner_id: winnerId,
    loser_id: loserId,
  };
}

function jsonClone(value) {
  return JSON.parse(JSON.stringify(value));
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
      getGameElementsEndpoint: '/api/game/_serial/elements',
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
        matchIndex: 0,
        stageStartCount: 2,
        matchesInStage: 0,
        targetMatches: 1,
      },
      $cookies: {
        get() {
          return null;
        },
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
    global.sessionStorage = createMemoryStorage();
    global.axios = undefined;
    global.$ = () => ({ modal() {} });
    delete global.window;
    swalCalls.length = 0;
  });

  test('requests the deferred custom ad after Vue renders an active game', () => {
    const dispatchedEvents = [];
    let nextTickCallback = null;
    global.window = {
      dispatchEvent(event) {
        dispatchedEvents.push(event.type);
      },
    };

    const vm = createGameVm({
      $nextTick(callback) {
        nextTickCallback = callback;
      },
    });

    vm.updateGame({
      current_round: 1,
      of_round: 1,
      remain_elements: 2,
      total_elements: 2,
      elements: vm.localElements,
    });

    assert.deepEqual(dispatchedEvents, []);
    assert.equal(typeof nextTickCallback, 'function');
    nextTickCallback();
    assert.deepEqual(dispatchedEvents, ['ranking:display-togawa-ad']);
  });

  test('persists an in-flight batch before HTTP and acknowledges only that snapshot', async () => {
    const request = deferred();
    let requestData;
    let requestConfig;
    global.axios = {
      post(_url, data, config) {
        requestData = data;
        requestConfig = config;

        const durableState = JSON.parse(localStorage.getItem('gamestate_post-serial'));
        assert.equal(durableState.inFlightBatch.votes.length, 1);
        assert.deepEqual(durableState.unsentVotes, [pendingVote(1, 1, 2)]);
        return request.promise;
      },
    };

    const vm = createGameVm();
    vm.localVoteSequence = 1;
    vm.unsentVotes = [pendingVote(1, 1, 2)];
    const saving = vm.performBatchVote();

    vm.localVoteSequence = 2;
    vm.unsentVotes.push(pendingVote(2, 1, 3));
    vm.saveToLocalStorage();
    request.resolve({ data: { status: 'processing', server_vote_count: 1 } });
    await saving;

    assert.deepEqual(requestData.votes, [{ winner_id: 1, loser_id: 2 }]);
    assert.equal(requestData.expected_vote_count, 0);
    assert.equal(requestConfig.timeout, 15000);
    assert.equal(vm.serverVoteCount, 1);
    assert.deepEqual(vm.unsentVotes, [pendingVote(2, 1, 3)]);
    assert.equal(vm.inFlightBatch, null);
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
        return Promise.resolve({
          data: { status: 'end_game', data: null, server_vote_count: 2 },
        });
      },
    };

    const vm = createGameVm();
    let handledFinalResponse = 0;
    vm.handleSendVote = response => {
      assert.equal(response.data.status, 'end_game');
      handledFinalResponse++;
    };
    vm.localVoteSequence = 1;
    vm.unsentVotes = [pendingVote(1, 1, 2)];

    const partialSave = vm.sendPartialBatchVotes();
    vm.localVoteSequence = 2;
    vm.unsentVotes.push(pendingVote(2, 1, 3));
    vm.sendBatchVotes();

    assert.equal(requests.length, 1, 'Final batch must wait for the partial request');
    firstRequest.resolve({ data: { status: 'processing', data: {}, server_vote_count: 1 } });
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

  test('422 disables cloud sync without deleting any local progress or outbox votes', async t => {
    t.mock.method(console, 'warn', () => {});
    let postCount = 0;
    global.axios = {
      post() {
        postCount++;
        return Promise.reject({ response: { status: 422, data: { message: 'conflict' } } });
      },
    };

    const vm = createGameVm();
    vm.localVoteSequence = 1;
    vm.localVotes = [pendingVote(1, 1, 2)];
    vm.unsentVotes = [pendingVote(1, 1, 2)];
    vm.currentLocalMatch = {
      left_id: 1,
      right_id: 2,
      current_round: 1,
      of_round: 1,
      remain_elements: 2,
      total_elements: 2,
      stage_start_count: 2,
    };
    const localElementsBefore = jsonClone(vm.localElements);
    const clientStateBefore = jsonClone(vm.clientState);
    await vm.sendPartialBatchVotes();

    assert.equal(vm.isLocalOnlyAfterBatchConflict, true);
    assert.equal(vm.isClientMode, true);
    assert.equal(vm.cloudSyncDisabledReason, 'http_422');
    assert.deepEqual(vm.localElements, localElementsBefore);
    assert.deepEqual(vm.clientState, clientStateBefore);
    assert.deepEqual(vm.localVotes, [pendingVote(1, 1, 2)]);
    assert.deepEqual(vm.unsentVotes, [pendingVote(1, 1, 2)]);
    assert.equal(vm.currentLocalMatch.left_id, 1);
    assert.equal(swalCalls.length, 0, '422 must not interrupt local voting with a retry dialog');

    const saved = JSON.parse(localStorage.getItem('gamestate_post-serial'));
    assert.equal(saved.localOnlyAfterBatchConflict, true);
    assert.deepEqual(saved.localVotes, [pendingVote(1, 1, 2)]);
    assert.deepEqual(saved.unsentVotes, [pendingVote(1, 1, 2)]);
    assert.deepEqual(saved.localElements, localElementsBefore);

    vm.sendPartialBatchVotes();
    vm.sendBatchVotes();
    await nextEventLoop();
    assert.equal(postCount, 1, 'No batch request may be made after the first 422');
  });

  test('409 never fetches or applies server state and preserves the local branch exactly', async t => {
    t.mock.method(console, 'warn', () => {});
    let postCount = 0;
    let getCount = 0;
    global.axios = {
      post() {
        postCount++;
        return Promise.reject({
          response: {
            status: 409,
            data: {
              code: 'game_state_conflict',
              reason: 'winner_eliminated',
              server_vote_count: 2,
            },
          },
        });
      },
      get() {
        getCount++;
        throw new Error('A cloud conflict must never trigger a server-state reload');
      },
    };

    const vm = createGameVm({
      elementsCount: 4,
      localElements: [
        { id: 1, local_eliminated: false, local_is_ready: false, local_win_count: 1 },
        { id: 2, local_eliminated: true, local_is_ready: false, local_win_count: 0 },
        { id: 3, local_eliminated: false, local_is_ready: true, local_win_count: 0 },
        { id: 4, local_eliminated: false, local_is_ready: true, local_win_count: 0 },
      ],
    });
    let nextRoundCount = 0;
    vm.nextLocalRound = () => {
      nextRoundCount++;
    };
    vm.localVoteSequence = 2;
    vm.localVotes = [pendingVote(1, 1, 2), pendingVote(2, 3, 4)];
    vm.unsentVotes = [pendingVote(2, 3, 4)];
    vm.matchHistory = [{ id: 'local-history' }];
    vm.currentLocalMatch = {
      left_id: 1,
      right_id: 3,
      current_round: 2,
      of_round: 2,
      remain_elements: 3,
      total_elements: 4,
      stage_start_count: 4,
    };
    const localElementsBefore = jsonClone(vm.localElements);
    const clientStateBefore = jsonClone(vm.clientState);

    await vm.sendPartialBatchVotes();

    assert.equal(postCount, 1);
    assert.equal(getCount, 0);
    assert.equal(vm.serverVoteCount, 0, 'Conflict metadata must not become local progress');
    assert.equal(vm.isLocalOnlyAfterBatchConflict, true);
    assert.equal(vm.cloudSyncDisabledReason, 'winner_eliminated');
    assert.deepEqual(vm.localElements, localElementsBefore);
    assert.deepEqual(vm.clientState, clientStateBefore);
    assert.deepEqual(vm.localVotes, [pendingVote(1, 1, 2), pendingVote(2, 3, 4)]);
    assert.deepEqual(vm.unsentVotes, [pendingVote(2, 3, 4)]);
    assert.deepEqual(vm.matchHistory, [{ id: 'local-history' }]);
    assert.equal(vm.currentLocalMatch.left_id, 1);
    assert.equal(nextRoundCount, 0);

    const saved = JSON.parse(localStorage.getItem('gamestate_post-serial'));
    assert.deepEqual(saved.localElements, localElementsBefore);
    assert.deepEqual(saved.unsentVotes, [pendingVote(2, 3, 4)]);
  });

  test('server-mode reload restores the host match history and keeps the history panel visible', () => {
    const history = [{ id: 'server-history' }];
    localStorage.setItem('gamestate_post-serial', JSON.stringify({
      schemaVersion: 3,
      localStateRevision: 1,
      gameSerial: 'game-serial',
      clientMode: false,
    }));
    localStorage.setItem('matchHistory_post-serial', JSON.stringify({
      gameSerial: 'game-serial',
      matches: history,
    }));

    const vm = createGameVm({
      gameSerial: null,
      showMatchHistory: false,
      matchHistory: [],
    });

    assert.equal(vm.loadFromLocalStorage(), true);
    assert.equal(vm.isClientMode, false);
    assert.equal(vm.showMatchHistory, true);
    assert.deepEqual(vm.matchHistory, history);
  });

  test('uses one 30-second ad refresh timer across game rounds', t => {
    let intervalCallback;
    let intervalDelay;
    let clearIntervalValue;
    let loadCount = 0;
    let reloadCount = 0;

    t.mock.method(global, 'setInterval', (callback, delay) => {
      intervalCallback = callback;
      intervalDelay = delay;
      return 123;
    });
    t.mock.method(global, 'clearInterval', value => {
      clearIntervalValue = value;
    });

    const vm = createGameVm({
      game: { current_round: 1 },
      $nextTick(callback) {
        callback();
      },
    });
    vm.loadGoogleAds = () => {
      loadCount++;
    };
    vm.reloadGoogleAds = () => {
      reloadCount++;
    };
    vm.needReloadAD = () => true;

    vm.startAdRefreshTimer();
    vm.startAdRefreshTimer();

    assert.equal(intervalDelay, 30000);
    assert.equal(loadCount, 1, 'The initial ad is loaded only once');
    assert.equal(reloadCount, 0, 'Starting another round must not refresh the ad');

    intervalCallback();
    assert.equal(reloadCount, 1);
    assert.equal(loadCount, 2);

    vm.stopAdRefreshTimer();
    assert.equal(clearIntervalValue, 123);
    assert.equal(vm.adRefreshInterval, null);
  });

  test('submitting a room bet does not refresh ads', async () => {
    global.axios = {
      post() {
        return Promise.resolve({});
      },
    };

    let loadCount = 0;
    let reloadCount = 0;
    const vm = createGameVm({
      gameRoomSerial: 'room-serial',
      betEndpoint: '/api/game/room/_serial/bet',
      game: {
        current_round: 1,
        of_round: 1,
        remain_elements: 2,
      },
    });
    vm.loadGoogleAds = () => {
      loadCount++;
    };
    vm.reloadGoogleAds = () => {
      reloadCount++;
    };

    vm.bet({ id: 1 }, { id: 2 });
    await nextEventLoop();

    assert.equal(loadCount, 0);
    assert.equal(reloadCount, 0);
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
    assert.deepEqual(vm.localVotes, [pendingVote(1, 1, 2)]);
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
    vm.localVoteSequence = 1;
    vm.localVotes = [pendingVote(1, 1, 2)];
    vm.unsentVotes = [pendingVote(1, 1, 2)];

    await vm.sendBatchVotes();

    assert.equal(vm.isLocalOnlyAfterBatchConflict, true);
    assert.equal(vm.status, 'end_game');
    assert.equal(cookieRemoved, 1);
    assert.equal(localStateCleared, 1);
    assert.equal(historyCleared, 1);
    assert.equal(rankOpened, 1);
    assert.equal(swalCalls.length, 0);

    const savedResult = JSON.parse(localStorage.getItem('gameresult_post-serial'));
    assert.equal(savedResult.gameSerial, 'game-serial');
    assert.equal(savedResult.localOnlyAfterBatchConflict, true);
    assert.deepEqual(savedResult.localElements, vm.localElements);
    assert.deepEqual(savedResult.localVotes, [pendingVote(1, 1, 2)]);
    assert.deepEqual(savedResult.unsentVotes, [pendingVote(1, 1, 2)]);
  });

  test('local game progress is kept when the dedicated rank result cannot be stored', t => {
    t.mock.method(console, 'error', () => {});
    const storage = createMemoryStorage();
    const setItem = storage.setItem.bind(storage);
    storage.setItem = (key, value) => {
      if (key === 'gameresult_post-serial') {
        throw new Error('storage quota exceeded');
      }
      setItem(key, value);
    };
    global.localStorage = storage;

    let localStateCleared = 0;
    const vm = createGameVm({
      isLocalOnlyAfterBatchConflict: true,
      localElements: [
        { id: 1, local_eliminated: false, local_win_count: 1 },
        { id: 2, local_eliminated: true, local_win_count: 0 },
      ],
    });
    vm.recordMatchFromLastVote = () => {};
    vm.clearLocalStorage = () => {
      localStateCleared++;
    };
    vm.clearMatchHistory = () => {};
    vm.showGameResult = () => {};

    vm.saveToLocalStorage();
    vm.finishLocalOnlyGame();

    assert.equal(localStateCleared, 0);
    assert.notEqual(localStorage.getItem('gamestate_post-serial'), null);
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

  test('reload refreshes the pairing without restoring the previous display', () => {
    const writer = createGameVm();
    writer.handleAnimationAfterNextRound = () => {};
    writer.nextLocalRound();
    const matchIndexBeforeReload = writer.clientState.matchIndex;
    writer.disableCloudSync('winner_eliminated');
    writer.releaseGameTabLease();

    const reader = createGameVm({
      gameSerial: null,
      isLocalOnlyAfterBatchConflict: false,
    });
    let nextLocalRoundCalls = 0;
    let restoreCalls = 0;
    reader.nextLocalRound = () => {
      nextLocalRoundCalls++;
    };
    reader.restoreCurrentLocalMatch = () => {
      restoreCalls++;
      return true;
    };

    assert.equal(reader.loadFromLocalStorage(), true);
    assert.equal(reader.gameSerial, 'game-serial');
    assert.equal(reader.isLocalOnlyAfterBatchConflict, true);
    assert.equal(reader.isClientMode, true);
    assert.equal(nextLocalRoundCalls, 0, 'Loading data must not advance the bracket');

    reader.resumeLocalGame(false, true);

    assert.equal(restoreCalls, 0, 'A page refresh must not restore the saved pairing');
    assert.equal(nextLocalRoundCalls, 1, 'A page refresh must draw a new pairing');
    assert.equal(reader.clientState.matchIndex, matchIndexBeforeReload);
  });

  test('reload can redraw from every unplayed candidate without advancing progress', t => {
    const localElements = [
      { id: 1, local_eliminated: false, local_is_ready: true, local_played: 0, local_win_count: 0 },
      { id: 2, local_eliminated: false, local_is_ready: true, local_played: 0, local_win_count: 0 },
      { id: 3, local_eliminated: false, local_is_ready: true, local_played: 5, local_win_count: 5 },
      { id: 4, local_eliminated: false, local_is_ready: true, local_played: 5, local_win_count: 5 },
      { id: 5, local_eliminated: false, local_is_ready: false, local_played: 1, local_win_count: 1 },
      { id: 6, local_eliminated: true, local_is_ready: false, local_played: 1, local_win_count: 0 },
    ];
    const clientState = {
      stage: 2,
      matchIndex: 2,
      stageStartCount: 6,
      matchesInStage: 1,
      targetMatches: 3,
    };
    const writer = createGameVm({
      elementsCount: 6,
      localElements: jsonClone(localElements),
      clientState: { ...clientState },
      currentLocalMatch: {
        left_id: 1,
        right_id: 2,
        current_round: 2,
        of_round: 3,
        remain_elements: 5,
        total_elements: 6,
        stage_start_count: 6,
      },
    });
    writer.existingElementIds = new Set(localElements.map(element => element.id));
    assert.equal(writer.saveToLocalStorage(), true);
    writer.releaseGameTabLease();

    const reader = createGameVm({ gameSerial: null });
    let displayedPair = null;
    reader.handleAnimationAfterNextRound = game => {
      displayedPair = game.elements.map(element => element.id);
    };
    reader.fetchRemainingElements = () => {};

    assert.equal(reader.loadFromLocalStorage(), true);
    t.mock.method(Math, 'random', () => 0);
    reader.resumeLocalGame(false, true);

    assert.deepEqual(displayedPair, [2, 3]);
    assert.equal(reader.clientState.matchIndex, clientState.matchIndex);
    assert.equal(reader.clientState.matchesInStage, clientState.matchesInStage);
    assert.deepEqual(reader.localVotes, []);
    assert.deepEqual(
      reader.localElements.map(element => element.local_is_ready),
      localElements.map(element => element.local_is_ready)
    );

    const saved = JSON.parse(localStorage.getItem('gamestate_post-serial'));
    assert.deepEqual(
      [saved.currentLocalMatch.left_id, saved.currentLocalMatch.right_id],
      displayedPair
    );
    assert.equal(saved.clientState.matchIndex, clientState.matchIndex);
    assert.equal(saved.clientState.matchesInStage, clientState.matchesInStage);
  });

  test('continueGame always chooses a valid local snapshot over different cloud progress', () => {
    const writer = createGameVm({
      localElements: [
        {
          id: 1,
          local_eliminated: false,
          local_is_ready: true,
          local_played: 7,
          local_win_count: 7,
        },
        {
          id: 2,
          local_eliminated: false,
          local_is_ready: true,
          local_played: 0,
          local_win_count: 0,
        },
      ],
    });
    writer.nextLocalRound = () => {};
    writer.currentLocalMatch = {
      left_id: 1,
      right_id: 2,
      current_round: 1,
      of_round: 1,
      remain_elements: 2,
      total_elements: 2,
      stage_start_count: 2,
    };
    writer.disableCloudSync('revision_mismatch');
    writer.releaseGameTabLease();

    let remoteGetCount = 0;
    global.axios = {
      get() {
        remoteGetCount++;
        throw new Error('Remote state must not be requested when local state exists');
      },
    };
    const reader = createGameVm({
      gameSerial: null,
      userLastGame: {
        serial: 'game-serial',
        element_count: 1024,
        vote_count: 999,
      },
    });
    let nextLocalRoundCalls = 0;
    reader.nextLocalRound = () => {
      nextLocalRoundCalls++;
    };
    reader.resetTimer = () => {};
    reader.startTimer = () => {};

    reader.continueGame();

    assert.equal(remoteGetCount, 0);
    assert.equal(reader.localElements[0].local_win_count, 7);
    assert.equal(nextLocalRoundCalls, 1);
    assert.equal(reader.cloudSyncDisabledReason, 'revision_mismatch');
  });

  test('an unresolved request survives reload and retries the same durable outbox', async () => {
    const abandonedRequest = deferred();
    global.axios = {
      post() {
        return abandonedRequest.promise;
      },
    };

    const writer = createGameVm({
      elementsCount: 4,
      localElements: [
        { id: 1, local_eliminated: false, local_is_ready: false, local_win_count: 1 },
        { id: 2, local_eliminated: true, local_is_ready: false, local_win_count: 0 },
        { id: 3, local_eliminated: false, local_is_ready: true, local_win_count: 0 },
        { id: 4, local_eliminated: false, local_is_ready: true, local_win_count: 0 },
      ],
      clientState: {
        stage: 1,
        matchIndex: 1,
        stageStartCount: 4,
        matchesInStage: 1,
        targetMatches: 2,
      },
    });
    writer.existingElementIds = new Set([1, 2, 3, 4]);
    writer.localVoteSequence = 1;
    writer.localVotes = [pendingVote(1, 1, 2)];
    writer.unsentVotes = [pendingVote(1, 1, 2)];
    writer.currentLocalMatch = {
      left_id: 3,
      right_id: 4,
      current_round: 2,
      of_round: 2,
      remain_elements: 3,
      total_elements: 4,
      stage_start_count: 4,
    };

    void writer.performBatchVote(false);

    const interruptedState = JSON.parse(localStorage.getItem('gamestate_post-serial'));
    assert.deepEqual(interruptedState.unsentVotes, [pendingVote(1, 1, 2)]);
    assert.deepEqual(interruptedState.inFlightBatch.votes, [pendingVote(1, 1, 2)]);
    writer.releaseGameTabLease();

    const retryRequests = [];
    global.axios = {
      post(_url, data) {
        retryRequests.push(data);
        return Promise.resolve({
          data: { status: 'processing', data: {}, server_vote_count: 1 },
        });
      },
    };

    const reader = createGameVm({ gameSerial: null });
    let restoredPair;
    reader.handleAnimationAfterNextRound = game => {
      restoredPair = game.elements.map(element => element.id);
    };
    reader.fetchRemainingElements = () => {};

    assert.equal(reader.loadFromLocalStorage(), true);
    assert.deepEqual(reader.localVotes, [pendingVote(1, 1, 2)]);
    assert.deepEqual(reader.unsentVotes, [pendingVote(1, 1, 2)]);
    assert.notEqual(reader.recoveredInterruptedBatch, null);
    assert.equal(reader.inFlightBatch, null);

    reader.resumeLocalGame();
    assert.deepEqual(restoredPair, [3, 4]);
    assert.equal(reader.clientState.matchIndex, 1);

    await new Promise(resolve => setTimeout(resolve, 0));
    await nextEventLoop();

    assert.equal(retryRequests.length, 1);
    assert.equal(retryRequests[0].expected_vote_count, 0);
    assert.deepEqual(retryRequests[0].votes, [{ winner_id: 1, loser_id: 2 }]);
    assert.deepEqual(reader.unsentVotes, []);
    assert.equal(reader.inFlightBatch, null);
    assert.equal(reader.serverVoteCount, 1);
  });

  test('a late callback from the old page cannot overwrite or delete the resumed snapshot', async t => {
    t.mock.method(console, 'warn', () => {});
    const oldRequest = deferred();
    global.axios = {
      post() {
        return oldRequest.promise;
      },
    };

    const oldPage = createGameVm();
    oldPage.localVoteSequence = 1;
    oldPage.localVotes = [pendingVote(1, 1, 2)];
    oldPage.unsentVotes = [pendingVote(1, 1, 2)];
    const oldSave = oldPage.performBatchVote(false);

    const resumedPage = createGameVm({ gameSerial: null });
    assert.equal(resumedPage.loadFromLocalStorage(), true);
    assert.equal(resumedPage.saveToLocalStorage(true), true);
    resumedPage.localElements[0].local_win_count = 99;
    assert.equal(resumedPage.saveToLocalStorage(), true);

    const ownedState = JSON.parse(localStorage.getItem('gamestate_post-serial'));
    assert.equal(ownedState.writerId, resumedPage.localWriterId);

    oldRequest.resolve({
      data: { status: 'processing', data: {}, server_vote_count: 1 },
    });
    await oldSave;

    let savedAfterLateResponse = JSON.parse(localStorage.getItem('gamestate_post-serial'));
    assert.equal(savedAfterLateResponse.writerId, resumedPage.localWriterId);
    assert.equal(savedAfterLateResponse.localElements[0].local_win_count, 99);
    assert.deepEqual(savedAfterLateResponse.unsentVotes, [pendingVote(1, 1, 2)]);

    oldPage.saveLocalRankResult = () => true;
    oldPage.clearMatchHistory = () => {};
    oldPage.showGameResult = () => {};
    oldPage.handleSendVote({ data: { status: 'end_game' } });

    savedAfterLateResponse = JSON.parse(localStorage.getItem('gamestate_post-serial'));
    assert.equal(savedAfterLateResponse.writerId, resumedPage.localWriterId);
    assert.equal(savedAfterLateResponse.localElements[0].local_win_count, 99);
  });

  test('the final local vote is recoverable before its batch request starts or returns', () => {
    const writer = createGameVm();
    writer.currentLocalMatch = {
      left_id: 1,
      right_id: 2,
      current_round: 1,
      of_round: 1,
      remain_elements: 2,
      total_elements: 2,
      stage_start_count: 2,
    };
    writer.handleAnimationAfterVoted = () => {};

    writer.handleClientVote(writer.localElements[0], writer.localElements[1], 'left');

    const durableGame = JSON.parse(localStorage.getItem('gamestate_post-serial'));
    const durableResult = JSON.parse(localStorage.getItem('gameresult_post-serial'));
    assert.deepEqual(durableGame.localVotes, [pendingVote(1, 1, 2)]);
    assert.deepEqual(durableGame.unsentVotes, [pendingVote(1, 1, 2)]);
    assert.equal(durableGame.currentLocalMatch, null);
    assert.equal(durableResult.cloudSyncPending, true);

    const reader = createGameVm({ gameSerial: null });
    assert.equal(reader.loadFromLocalStorage(), true);
    assert.deepEqual(reader.localVotes, [pendingVote(1, 1, 2)]);
    assert.deepEqual(reader.unsentVotes, [pendingVote(1, 1, 2)]);
    assert.equal(
      reader.localElements.filter(element => !element.local_eliminated).length,
      1
    );
  });

  test('never sends user votes when the durable pre-request write fails', async t => {
    t.mock.method(console, 'error', () => {});
    t.mock.method(console, 'warn', () => {});
    const storage = createMemoryStorage();
    storage.setItem = () => {
      throw new Error('local storage unavailable');
    };
    global.localStorage = storage;

    let postCount = 0;
    global.axios = {
      post() {
        postCount++;
        return Promise.resolve({ data: { server_vote_count: 1 } });
      },
    };

    const vm = createGameVm();
    vm.localVoteSequence = 1;
    vm.localVotes = [pendingVote(1, 1, 2)];
    vm.unsentVotes = [pendingVote(1, 1, 2)];

    await vm.sendPartialBatchVotes();

    assert.equal(postCount, 0);
    assert.equal(vm.isLocalOnlyAfterBatchConflict, true);
    assert.equal(vm.cloudSyncDisabledReason, 'local_state_save_failed');
    assert.deepEqual(vm.localVotes, [pendingVote(1, 1, 2)]);
    assert.deepEqual(vm.unsentVotes, [pendingVote(1, 1, 2)]);
  });

  test('non-422 network errors retain votes and do not enable local-only mode', async t => {
    t.mock.method(console, 'error', () => {});
    global.axios = {
      post() {
        return Promise.reject(new Error('network unavailable'));
      },
    };

    const vm = createGameVm();
    vm.localVoteSequence = 1;
    vm.unsentVotes = [pendingVote(1, 1, 2)];
    await vm.sendPartialBatchVotes();

    assert.equal(vm.isLocalOnlyAfterBatchConflict, false);
    assert.deepEqual(vm.unsentVotes, [pendingVote(1, 1, 2)]);
    assert.equal(vm.inFlightBatch, null);
    assert.equal(vm.batchVoteInterval, 21);
    assert.equal(vm.isBatchVoting, false);
    assert.equal(vm.isCloudSaving, false);
  });

  test('beforeunload persists the whole client snapshot without starting a request', () => {
    let postCount = 0;
    let saveCount = 0;
    global.axios = {
      post() {
        postCount++;
      },
    };

    const vm = createGameVm();
    vm.unsentVotes = [];
    vm.currentLocalMatch = { left_id: 1, right_id: 2 };
    vm.saveToLocalStorage = () => {
      saveCount++;
    };
    vm.handleBeforeUnload();

    assert.equal(saveCount, 1);
    assert.equal(postCount, 0);
  });

  test('the first active tab remains the only writer and a second tab is read-only', () => {
    const owner = createGameVm();
    owner.currentLocalMatch = {
      left_id: 1,
      right_id: 2,
      current_round: 1,
      of_round: 1,
      remain_elements: 2,
      total_elements: 2,
      stage_start_count: 2,
    };
    assert.equal(owner.saveToLocalStorage(), true);

    const follower = createGameVm({ gameSerial: null });
    follower.handleAnimationAfterNextRound = () => {};
    assert.equal(follower.loadFromLocalStorage(), true);
    follower.resumeLocalGame();

    const lease = JSON.parse(localStorage.getItem('gamelease_post-serial_game-serial'));
    assert.equal(lease.ownerId, owner.localWriterId);
    assert.equal(follower.isGameTabReadOnly, true);

    const voteAccepted = follower.handleClientVote(
      follower.localElements[0],
      follower.localElements[1],
      'left'
    );
    assert.equal(voteAccepted, false);
    assert.deepEqual(follower.localVotes, []);

    const canonical = JSON.parse(localStorage.getItem('gamestate_post-serial'));
    assert.equal(canonical.writerId, owner.localWriterId);
    assert.deepEqual(canonical.localVotes, []);
  });

  test('a read-only tab never retries the canonical outbox', async () => {
    let postCount = 0;
    global.axios = {
      post() {
        postCount++;
        return Promise.resolve({ data: { status: 'processing', server_vote_count: 1 } });
      },
    };

    const owner = createGameVm();
    owner.localVoteSequence = 1;
    owner.localVotes = [pendingVote(1, 1, 2)];
    owner.unsentVotes = [pendingVote(1, 1, 2)];
    owner.currentLocalMatch = {
      left_id: 1,
      right_id: 2,
      current_round: 1,
      of_round: 1,
      remain_elements: 2,
      total_elements: 2,
      stage_start_count: 2,
    };
    assert.equal(owner.saveToLocalStorage(), true);

    const follower = createGameVm({ gameSerial: null });
    follower.handleAnimationAfterNextRound = () => {};
    assert.equal(follower.loadFromLocalStorage(), true);
    follower.resumeLocalGame();
    await nextEventLoop();

    assert.equal(follower.isGameTabReadOnly, true);
    assert.equal(postCount, 0);
    assert.deepEqual(follower.unsentVotes, [pendingVote(1, 1, 2)]);
  });

  test('a second tab cannot start a new game by deleting an active canonical game', () => {
    let createRequests = 0;
    global.axios = {
      post() {
        createRequests++;
        return Promise.resolve({ data: { game_serial: 'new-game' } });
      },
    };

    const owner = createGameVm();
    owner.currentLocalMatch = {
      left_id: 1,
      right_id: 2,
      current_round: 1,
      of_round: 1,
      remain_elements: 2,
      total_elements: 2,
      stage_start_count: 2,
    };
    assert.equal(owner.saveToLocalStorage(), true);

    const follower = createGameVm({ createGameEndpoint: '/api/game' });
    follower.createGame();

    assert.equal(createRequests, 0);
    assert.equal(follower.isGameTabReadOnly, true);
    const canonical = JSON.parse(localStorage.getItem('gamestate_post-serial'));
    assert.equal(canonical.writerId, owner.localWriterId);
  });

  test('explicit takeover fences the old tab and resumes the latest canonical snapshot', () => {
    const owner = createGameVm();
    owner.currentLocalMatch = {
      left_id: 1,
      right_id: 2,
      current_round: 1,
      of_round: 1,
      remain_elements: 2,
      total_elements: 2,
      stage_start_count: 2,
    };
    assert.equal(owner.saveToLocalStorage(), true);
    const ownerToken = owner.gameLeaseToken;

    const follower = createGameVm({ gameSerial: null });
    follower.handleAnimationAfterNextRound = () => {};
    follower.resetTimer = () => {};
    follower.startTimer = () => {};
    assert.equal(follower.loadFromLocalStorage(), true);
    follower.resumeLocalGame();
    assert.equal(follower.isGameTabReadOnly, true);

    assert.equal(follower.takeOverGameTab(true), true);
    const lease = JSON.parse(localStorage.getItem('gamelease_post-serial_game-serial'));
    assert.equal(lease.ownerId, follower.localWriterId);
    assert.ok(lease.fencingToken > ownerToken);

    owner.monitorGameTabLease();
    assert.equal(owner.isGameTabReadOnly, true);
    assert.equal(owner.handleClientVote(owner.localElements[0], owner.localElements[1]), false);

    const canonical = JSON.parse(localStorage.getItem('gamestate_post-serial'));
    assert.equal(canonical.writerId, follower.localWriterId);
    assert.equal(canonical.writerLeaseToken, lease.fencingToken);
  });

  test('a stale heartbeat cannot reclaim the lease after a higher fencing token took over', () => {
    const owner = createGameVm();
    owner.currentLocalMatch = {
      left_id: 1,
      right_id: 2,
      current_round: 1,
      of_round: 1,
      remain_elements: 2,
      total_elements: 2,
      stage_start_count: 2,
    };
    assert.equal(owner.saveToLocalStorage(), true);
    const staleToken = owner.gameLeaseToken;

    const follower = createGameVm({ gameSerial: null });
    follower.handleAnimationAfterNextRound = () => {};
    follower.resetTimer = () => {};
    follower.startTimer = () => {};
    assert.equal(follower.loadFromLocalStorage(), true);
    assert.equal(follower.takeOverGameTab(true), true);
    assert.ok(follower.gameLeaseToken > staleToken);

    localStorage.setItem('gamelease_post-serial_game-serial', JSON.stringify({
      schemaVersion: 1,
      gameSerial: 'game-serial',
      ownerId: owner.localWriterId,
      fencingToken: staleToken,
      heartbeatAt: Date.now(),
      expiresAt: Date.now() + 120000,
    }));
    follower.handleGameTabStorageEvent({ key: 'gamelease_post-serial_game-serial' });

    const restoredLease = JSON.parse(localStorage.getItem('gamelease_post-serial_game-serial'));
    assert.equal(restoredLease.ownerId, follower.localWriterId);
    assert.equal(restoredLease.fencingToken, follower.gameLeaseToken);
    assert.equal(follower.isGameTabReadOnly, false);
  });

  test('an already-diverged old tab is preserved as a separate local-only branch', () => {
    const owner = createGameVm();
    owner.currentLocalMatch = {
      left_id: 1,
      right_id: 2,
      current_round: 1,
      of_round: 1,
      remain_elements: 2,
      total_elements: 2,
      stage_start_count: 2,
    };
    assert.equal(owner.saveToLocalStorage(), true);

    const follower = createGameVm({ gameSerial: null });
    follower.handleAnimationAfterNextRound = () => {};
    follower.resetTimer = () => {};
    follower.startTimer = () => {};
    assert.equal(follower.loadFromLocalStorage(), true);
    assert.equal(follower.takeOverGameTab(true), true);

    owner.localVoteSequence = 1;
    owner.localVotes.push(pendingVote(1, 1, 2));
    owner.unsentVotes.push(pendingVote(1, 1, 2));
    owner.localElements[0].local_win_count = 1;
    owner.localElements[1].local_eliminated = true;
    owner.hasUnpersistedLocalProgress = true;
    owner.handleGameTabLeaseLost(owner.readGameTabLease());

    assert.notEqual(owner.localBranchId, null);
    assert.equal(owner.isLocalOnlyAfterBatchConflict, true);
    assert.equal(owner.isGameTabReadOnly, false);
    assert.equal(owner.cloudSyncDisabledReason, 'multi_tab_divergence');

    const branchKey = `gamebranch_post-serial_${owner.localBranchId}`;
    const branch = JSON.parse(localStorage.getItem(branchKey));
    assert.deepEqual(branch.localVotes, [pendingVote(1, 1, 2)]);
    assert.deepEqual(branch.unsentVotes, [pendingVote(1, 1, 2)]);

    const canonical = JSON.parse(localStorage.getItem('gamestate_post-serial'));
    assert.equal(canonical.writerId, follower.localWriterId);
    assert.deepEqual(canonical.localVotes, []);

    localStorage.setItem('gameresult_post-serial', JSON.stringify({ marker: 'canonical-result' }));
    assert.equal(owner.saveLocalRankResult(false), true);
    const branchResultKey = `gameresult_post-serial_branch_${owner.localBranchId}`;
    const branchResult = JSON.parse(localStorage.getItem(branchResultKey));
    assert.deepEqual(branchResult.localVotes, [pendingVote(1, 1, 2)]);
    assert.equal(
      JSON.parse(localStorage.getItem('gameresult_post-serial')).marker,
      'canonical-result'
    );
    assert.equal(
      sessionStorage.getItem('gameresult_selection_post-serial'),
      branchResultKey
    );
  });

  test('a released lease can be acquired by a waiting tab without overwriting progress', () => {
    const owner = createGameVm();
    owner.localVoteSequence = 1;
    owner.localVotes = [pendingVote(1, 1, 2)];
    owner.currentLocalMatch = {
      left_id: 1,
      right_id: 2,
      current_round: 1,
      of_round: 1,
      remain_elements: 2,
      total_elements: 2,
      stage_start_count: 2,
    };
    assert.equal(owner.saveToLocalStorage(), true);

    const follower = createGameVm({ gameSerial: null });
    follower.handleAnimationAfterNextRound = () => {};
    follower.resetTimer = () => {};
    follower.startTimer = () => {};
    assert.equal(follower.loadFromLocalStorage(), true);
    follower.resumeLocalGame();
    assert.equal(follower.isGameTabReadOnly, true);

    assert.equal(owner.releaseGameTabLease(), true);
    assert.equal(follower.takeOverGameTab(false), true);
    assert.deepEqual(follower.localVotes, [pendingVote(1, 1, 2)]);
    assert.equal(follower.isGameTabReadOnly, false);
  });
});

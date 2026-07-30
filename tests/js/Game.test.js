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
    global.axios = undefined;
    global.$ = () => ({ modal() {} });
    swalCalls.length = 0;
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

  test('reload restores the exact displayed pairing without advancing the bracket twice', () => {
    const writer = createGameVm();
    let originallyDisplayedPair;
    writer.handleAnimationAfterNextRound = game => {
      originallyDisplayedPair = game.elements.map(element => element.id);
    };
    writer.nextLocalRound();
    const matchIndexBeforeReload = writer.clientState.matchIndex;
    writer.disableCloudSync('winner_eliminated');

    const reader = createGameVm({
      gameSerial: null,
      isLocalOnlyAfterBatchConflict: false,
    });
    let nextLocalRoundCalls = 0;
    let restoredPair;
    reader.nextLocalRound = () => {
      nextLocalRoundCalls++;
    };
    reader.handleAnimationAfterNextRound = game => {
      restoredPair = game.elements.map(element => element.id);
    };

    assert.equal(reader.loadFromLocalStorage(), true);
    assert.equal(reader.gameSerial, 'game-serial');
    assert.equal(reader.isLocalOnlyAfterBatchConflict, true);
    assert.equal(reader.isClientMode, true);
    assert.equal(nextLocalRoundCalls, 0, 'Loading data must not advance the bracket');

    reader.resumeLocalGame();

    assert.deepEqual(restoredPair, originallyDisplayedPair);
    assert.equal(reader.clientState.matchIndex, matchIndexBeforeReload);
    assert.equal(nextLocalRoundCalls, 0, 'A saved pairing must be rendered, not redrawn');
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
    let restoredPair;
    reader.handleAnimationAfterNextRound = game => {
      restoredPair = game.elements.map(element => element.id);
    };
    reader.resetTimer = () => {};
    reader.startTimer = () => {};

    reader.continueGame();

    assert.equal(remoteGetCount, 0);
    assert.equal(reader.localElements[0].local_win_count, 7);
    assert.deepEqual(restoredPair, [1, 2]);
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
});

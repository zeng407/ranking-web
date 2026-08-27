<script>
import Swal from "sweetalert2";
import ICountUp from 'vue-countup-v2';
import QRCode from 'qrcode';


const MD_WIDTH_SIZE = 576;
const MOBILE_HEIGHT = 700;
const BATCH_VOTE_SAVE_INTERVAL = 10; // 每 10 票存一次
const KEEP_VOTE_RECORD_COUNT = 128 // 保留最近 128 筆投票紀錄
const LOCAL_GAME_STATE_VERSION = 3;
const BATCH_REQUEST_TIMEOUT_MS = 15000;
const GAME_TAB_LEASE_VERSION = 1;
const GAME_TAB_HEARTBEAT_MS = 5000;
const GAME_TAB_LEASE_TTL_MS = 120000; // 容忍背景分頁 timer throttling；正常關閉會立即 release
const GAME_TAB_MONITOR_MS = 5000;
const AD_REFRESH_INTERVAL_MS = 30000;

function createLocalWriterId() {
  return `${Date.now().toString(36)}-${Math.random().toString(36).slice(2)}`;
}

export default {
  beforeMount() {
    this.loadGameSerialFromCookie();
    this.bootScreenSize();
  },
  mounted() {
    if (this.gameRoomSerial) {
      this.showGameRoomJoinSetting();
    } else {
      if (!this.requirePassword) {
        this.loadGameSetting();
      }
      this.showGameSettingPanel();
    }
    this.origin = window.location.origin;
    this.host = window.location.host;
    // update elementHeight to 50% of current window height
    this.enableTooltip();
    this.registerResizeEvent();
    this.resizeElementHeight();
    this.registerScrollEvent();

    // 發動儲存 BatchVotes (離線模式)
    window.addEventListener('beforeunload', this.handleBeforeUnload);
    this.setupGameTabCoordination();
  },

  beforeDestroy() {
    this.stopTimer();
    this.stopAdRefreshTimer();
    window.removeEventListener('beforeunload', this.handleBeforeUnload);
    this.teardownGameTabCoordination(true);
  },
  components: {
    ICountUp
  },
  props: {
    postSerial: String,
    userLastGame: Object | null,
    getRankRoute: String,
    getGameSettingEndpoint: String,
    nextRoundEndpoint: String,
    createGameEndpoint: String,
    voteGameEndpoint: String,
    requirePassword: Boolean,
    accessEndpoint: String,
    propsGameRoomSerial: {
      type: String | null,
    },
    getRoomEndpoint: String,
    betEndpoint: String,
    updateRoomProfileEndpoint: String,
    getRoomVotesEndpoint: String,
    getRoomUserEndpoint: String,
    getGameElementsEndpoint: String,
    batchVoteEndpoint: String,
    propsEnableClientMode: {
      type: Boolean,
      default: true,
    },
  },
  data: function () {
    return {
      isMobileScreen: true,
      origin: "",
      host: "",
      elementHeight: 360,
      gameBodyHeight: 700,
      gameSerial: null,
      game: null,
      le: null,
      re: null,
      status: null,
      post: null,
      elementsCount: "",
      isVoting: false,
      isDataLoading: false,
      isLeftPlaying: false,
      isRightPlaying: false,
      rememberedScrollPosition: null,
      error403WhenLoad: false,
      invalidPasswordWhenLoad: false,
      errorImages: [],
      currentRemainElement: false,
      mousePosition: 1, // 1:left , right:0
      isHoverIn: false,
      showPopover: false,
      refreshAD: false,
      adRefreshInterval: null,
      leftImageLoaded: false,
      rightImageLoaded: false,
      creatingGame: false,
      finishingGame: false,
      gameResultUrl: "",
      inputPassword: "",
      leftReady: true,
      rightReady: true,
      knownIncorrectPassword: false,
      viewerOptions: {
        inline: false,
        button: true,
        movable: true,
        navbar: 0,
        title: false,
        toolbar: {
          zoomIn: 1,
          zoomOut: 1,
          reset: 1,
          rotateRight: 1,
        },
        rotatable: true,
      },
      animationShowLeftPlayer: true,
      animationShowRightPlayer: true,
      animationShowRoundSession: true,
      readyAds: false,
      // game room
      gameRoomSerial: this.propsGameRoomSerial,
      gameRoom: null,
      currentBetRecord: null,
      showFirework: false,
      showBetFailed: false,
      gameBetRanks: [],
      runInBackGameRoom: false,
      isEditingNickname: false,
      newNickname: "",
      qrUrl: "",
      gameRoomUrl: "",
      isHostingGameRank: false,
      autoRefreshRoomInterval: null,
      autoRefreshRoomCounter: 0,
      showRoomInvitation: true,
      gameRoomVotes: [],
      isListeningGameBet: false,
      showGameRoomVotes: false,
      sortByTop: true,
      showCreateRoomButton: false,

      isClientMode: this.propsEnableClientMode,
      localElements: [], // 儲存所有參賽者物件 { ...element, local_win_count: 0, local_eliminated: false }
      localVotes: [], // 儲存投票紀錄 [{winner_id, loser_id}, ...]
      existingElementIds: new Set(), // 已存在的元素 ID 集合 (用來避免重複加入)
      localWriterId: createLocalWriterId(),
      localStateSchemaVersion: LOCAL_GAME_STATE_VERSION,
      localStateRevision: 0,
      gameLeaseToken: 0,
      gameLeaseOwnerId: null,
      gameLeaseExpiresAt: 0,
      gameLeaseHeartbeatTimer: null,
      gameLeaseMonitorTimer: null,
      gameTabChannel: null,
      gameTabTakeoverTimer: null,
      isGameTabReadOnly: false,
      lastGameTabNoticeAt: 0,
      hasUnpersistedLocalProgress: false,
      localBranchId: null,
      localBranchReason: null,
      localVoteSequence: 0,
      currentLocalMatch: null,
      clientState: {
          currentRound: 1,
          matchesPlayedInRound: 0,
          ofRound: 0, // 這一輪總共要打幾場
          roundStartRemain: 0 // 這一輪開始時有多少人
      },

      // Cloud Save 相關變數
      batchVoteInterval: BATCH_VOTE_SAVE_INTERVAL,
      unsentVotes: [],       // 尚未同步到雲端的投票
      isCloudSaving: false,  // 是否正在儲存中
      isBatchVoting: false,  // 避免重複送出 batch vote
      serverVoteCount: 0,   // 最後一次由 server 確認的 game round revision
      inFlightBatch: null,  // 已開始傳送但尚未收到回應的 durable batch
      recoveredInterruptedBatch: null,
      pendingFinalBatchVote: false, // Partial batch 完成後是否還需要送出最終結果
      isLocalOnlyAfterBatchConflict: false, // 雲端拒絕本地分支後，改為純本地完成
      cloudSyncDisabledReason: null,

      // 計時器
      timerSeconds: 0,
      timerInterval: null,

      // 對戰紀錄時間軸
      matchHistory: [],
      lastVotePair: null,
      showMatchHistory: false,

      // 時間軸拖曳
      isDragging: false,
      startX: 0,
      scrollLeft: 0,
    };
  },
  computed: {
    // UI 顯示：目前場次 (改讀 game)
    displayCurrentRound() {
        return (this.game && this.game.current_round) ? this.game.current_round : 1;
    },

    // UI 顯示：本輪總場次 (改讀 game)
    displayTotalRound() {
        return (this.game && this.game.of_round) ? this.game.of_round : 1;
    },

    // UI 顯示：總參賽人數 (灰階旗標)
    displayTotalElements() {
        // 優先使用設定值，若無則回退到 game 物件
        return this.elementsCount || (this.game ? this.game.total_elements : 0);
    },

    // UI 顯示：剩餘人數 (改讀 game)
    displayRemainElements() {
        return (this.game && this.game.remain_elements) ? this.game.remain_elements : 0;
    },

    roundTitleCount() {
        // 優先讀取我們剛剛塞進去的 stage_start_count
        if (this.game && this.game.stage_start_count) {
            return this.game.stage_start_count;
        }
        // Fallback
        if (this.clientState && this.clientState.stageStartCount) {
             return this.clientState.stageStartCount;
        }
        return this.displayRemainElements;
    },

    displayTimer() {
      const total = this.timerSeconds;
      if (total >= 3600) {
        const h = Math.floor(total / 3600);
        const m = Math.floor((total % 3600) / 60);
        const s = total % 60;
        return `${h}h${m}m${s}s`;
      }
      if (total >= 60) {
        const m = Math.floor(total / 60);
        const s = total % 60;
        return `${m}m${s}s`;
      }
      return `${total}s`;
    },

    gameRankUrl: function () {
      return this.getRankRoute.replace("_serial", this.postSerial);
    },
    gameOnlineUsers() {
      if (this.gameRoom && (this.gameRoom.online_users - 1) > 0) {
        return this.gameRoom.online_users - 1;
      }
      return 0;
    },
    isElementsPowerOfTwo: function () {
      if (!this.post || !this.post.elements_count) {
        return false;
      }

      return Number.isInteger(Math.log2(this.post.elements_count));
    },
    isBetGameHost() {
      if (!this.gameRoom) {
        return false;
      }
      return !this.gameRoom.user;
    },
    isBetGameClient() {
      if (!this.gameRoom) {
        return false;
      }
      // to boolean
      return !!this.gameRoom.user;
    },
    leftVotes() {
      if (!this.gameRoomVotes || this.gameRoomVotes.length === 0) {
        return 0;
      }

      if (this.gameRoomVotes.remain_elements !== this.game.remain_elements) {
        return 0;
      }

      if (parseInt(this.gameRoomVotes.first_candidate) !== this.le.id || parseInt(this.gameRoomVotes.second_candidate) !== this.re.id) {
        return 0;
      }

      return this.gameRoomVotes.first_candidate_votes;
    },
    rightVotes() {
      if (!this.gameRoomVotes || this.gameRoomVotes.length === 0) {
        return 0;
      }

      if (this.gameRoomVotes.remain_elements !== this.game.remain_elements) {
        return 0;
      }

      if (parseInt(this.gameRoomVotes.first_candidate) !== this.le.id || parseInt(this.gameRoomVotes.second_candidate) !== this.re.id) {
        return 0;
      }

      return this.gameRoomVotes.second_candidate_votes;
    },
    leftVotesPercentage() {
      if (this.leftVotes === 0 && this.rightVotes === 0) {
        return 0;
      }

      return Math.round(this.leftVotes / (this.leftVotes + this.rightVotes) * 100);
    },
    rightVotesPercentage() {
      if (this.leftVotes === 0 && this.rightVotes === 0) {
        return 0;
      }

      return Math.round(this.rightVotes / (this.leftVotes + this.rightVotes) * 100);
    },
    getSortedRanks() {
      if (!this.gameBetRanks) {
        return [];
      }
      if (this.sortByTop) {
        return this.gameBetRanks.top_10;
      } else {
        return this.gameBetRanks.bottom_10;
      }
    },
    isGameRoomFinished() {
      return this.gameRoom && this.gameRoom.is_game_completed
    },
    isFixedGameHeight() {
      return this.isMobileScreen && !this.isBetGameClient;
    },
    isGameVoteReadOnly() {
      return this.isGameTabReadOnly
        && !this.localBranchId
        && !this.isHostingGameRank
        && !this.isBetGameClient;
    },
  },
  methods: {
    formatThinkingTime(seconds) {
      if (!seconds) return '0s';
      if (seconds >= 3600) {
        const h = Math.floor(seconds / 3600);
        const m = Math.floor((seconds % 3600) / 60);
        const s = seconds % 60;
        return `${h}h${m}m${s}s`;
      }
      if (seconds >= 60) {
        const m = Math.floor(seconds / 60);
        const s = seconds % 60;
        return `${m}m${s}s`;
      }
      return `${seconds}s`;
    },
    // game room
    showGameRoomJoinSetting() {
      $("#gameRoomJoin").modal("show");
    },
    joinRoom() {
      $("#gameRoomJoin").modal("hide");
      this.listenNotifyVoted();
      this.listenGameBetRank();
      this.listenGameRoomRefresh();
      this.getGameRoom();
    },
    minimizeGameRoom() {

      clearInterval(this.isListeningGameBet);
      this.leaveGameRoom();
      this.isHostingGameRank = false;
      this.runInBackGameRoom = true;
      this.gameBetRanks = [];
      this.gameRoomVotes = [];
      this.showGameRoomVotes = false;
      this.isListeningGameBet = false;
      $("#close-game-room").tooltip("dispose");
      $("#minimize-game-room").tooltip("dispose");
      this.$bus.$emit("closeGameRoom");
    },
    closeGameRoom() {
      clearInterval(this.autoRefreshRoomInterval);
      this.minimizeGameRoom();
      this.gameRoom = null;
      this.gameRoomSerial = null;
      this.autoRefreshRoomInterval = null;
      this.runInBackGameRoom = false;
    },
    changeSortRanks() {
      this.sortByTop = !this.sortByTop;
    },
    shouldCoordinateGameTabs() {
      return !!this.gameSerial
        && !this.localBranchId
        && !this.isHostingGameRank
        && !this.isBetGameClient;
    },
    getCanonicalLocalGameStateKey() {
      return `gamestate_${this.postSerial}`;
    },
    getLocalBranchStateKey(branchId = this.localBranchId) {
      return branchId ? `gamebranch_${this.postSerial}_${branchId}` : null;
    },
    getLocalBranchSelectionKey() {
      return `gamebranch_selection_${this.postSerial}`;
    },
    getLocalRankResultSelectionKey() {
      return `gameresult_selection_${this.postSerial}`;
    },
    getSelectedLocalBranchId() {
      try {
        if (typeof sessionStorage === 'undefined') return null;
        return sessionStorage.getItem(this.getLocalBranchSelectionKey());
      } catch (error) {
        return null;
      }
    },
    selectLocalBranch(branchId) {
      try {
        if (typeof sessionStorage === 'undefined') return;
        if (branchId) {
          sessionStorage.setItem(this.getLocalBranchSelectionKey(), branchId);
        } else {
          sessionStorage.removeItem(this.getLocalBranchSelectionKey());
        }
      } catch (error) {
        console.warn("Unable to select a local game branch.", error);
      }
    },
    selectLocalRankResult(key) {
      try {
        if (typeof sessionStorage === 'undefined') return;
        if (key) {
          sessionStorage.setItem(this.getLocalRankResultSelectionKey(), key);
        } else {
          sessionStorage.removeItem(this.getLocalRankResultSelectionKey());
        }
      } catch (error) {
        console.warn("Unable to select a local rank result.", error);
      }
    },
    getGameTabLeaseKey(gameSerial = this.gameSerial) {
      return gameSerial ? `gamelease_${this.postSerial}_${gameSerial}` : null;
    },
    parseLocalJson(rawValue) {
      if (!rawValue) return null;
      try {
        return JSON.parse(rawValue);
      } catch (error) {
        return null;
      }
    },
    readGameTabLease(gameSerial = this.gameSerial) {
      const key = this.getGameTabLeaseKey(gameSerial);
      if (!key) return null;
      try {
        return this.parseLocalJson(localStorage.getItem(key));
      } catch (error) {
        return null;
      }
    },
    isGameTabLeaseActive(lease, now = Date.now()) {
      return !!lease
        && String(lease.gameSerial) === String(this.gameSerial)
        && Number(lease.expiresAt || 0) > now;
    },
    getCanonicalSnapshotLeaseToken() {
      try {
        const snapshot = this.parseLocalJson(
          localStorage.getItem(this.getCanonicalLocalGameStateKey())
        );
        if (!snapshot || String(snapshot.gameSerial) !== String(this.gameSerial)) {
          return 0;
        }
        return Number(snapshot.writerLeaseToken || 0);
      } catch (error) {
        return 0;
      }
    },
    getActiveForeignCanonicalGameLease() {
      try {
        const snapshot = this.parseLocalJson(
          localStorage.getItem(this.getCanonicalLocalGameStateKey())
        );
        if (!snapshot || !snapshot.gameSerial) return null;

        const lease = this.readGameTabLease(snapshot.gameSerial);
        if (lease
          && String(lease.gameSerial) === String(snapshot.gameSerial)
          && Number(lease.expiresAt || 0) > Date.now()
          && lease.ownerId !== this.localWriterId) {
          return lease;
        }
      } catch (error) {
        return null;
      }
      return null;
    },
    ownsGameTabLease() {
      if (!this.shouldCoordinateGameTabs()) return true;
      const lease = this.readGameTabLease();
      return this.isGameTabLeaseActive(lease)
        && lease.ownerId === this.localWriterId
        && Number(lease.fencingToken || 0) === Number(this.gameLeaseToken || 0);
    },
    acquireGameTabLease(force = false) {
      if (!this.shouldCoordinateGameTabs()) return true;

      const now = Date.now();
      const currentLease = this.readGameTabLease();
      const activeForeignLease = this.isGameTabLeaseActive(currentLease, now)
        && currentLease.ownerId !== this.localWriterId;

      if (activeForeignLease && !force) {
        this.handleGameTabLeaseLost(currentLease);
        return false;
      }

      const sameLease = this.isGameTabLeaseActive(currentLease, now)
        && currentLease.ownerId === this.localWriterId
        && Number(currentLease.fencingToken || 0) === Number(this.gameLeaseToken || 0);
      const previousToken = Math.max(
        Number(this.gameLeaseToken || 0),
        Number(currentLease && currentLease.fencingToken || 0),
        this.getCanonicalSnapshotLeaseToken()
      );
      const fencingToken = sameLease ? previousToken : previousToken + 1;
      const lease = {
        schemaVersion: GAME_TAB_LEASE_VERSION,
        gameSerial: this.gameSerial,
        ownerId: this.localWriterId,
        fencingToken,
        heartbeatAt: now,
        expiresAt: now + GAME_TAB_LEASE_TTL_MS,
      };

      try {
        localStorage.setItem(this.getGameTabLeaseKey(), JSON.stringify(lease));
        const confirmedLease = this.readGameTabLease();
        const acquired = !!confirmedLease
          && confirmedLease.ownerId === this.localWriterId
          && Number(confirmedLease.fencingToken || 0) === fencingToken;

        if (!acquired) {
          this.handleGameTabLeaseLost(confirmedLease);
          return false;
        }

        this.gameLeaseToken = fencingToken;
        this.gameLeaseOwnerId = this.localWriterId;
        this.gameLeaseExpiresAt = lease.expiresAt;
        this.isGameTabReadOnly = false;
        this.startGameTabHeartbeat();
        this.postGameTabMessage({
          type: 'lease-acquired',
          gameSerial: this.gameSerial,
          ownerId: this.localWriterId,
          fencingToken,
        });
        return true;
      } catch (error) {
        console.error("Unable to acquire the local game lease.", error);
        return false;
      }
    },
    refreshGameTabLease() {
      if (!this.shouldCoordinateGameTabs() || !this.ownsGameTabLease()) {
        this.stopGameTabHeartbeat();
        return false;
      }

      const lease = this.readGameTabLease();
      if (!lease
        || lease.ownerId !== this.localWriterId
        || Number(lease.fencingToken || 0) !== Number(this.gameLeaseToken || 0)) {
        this.handleGameTabLeaseLost(lease);
        return false;
      }
      const now = Date.now();
      const refreshedLease = {
        ...lease,
        heartbeatAt: now,
        expiresAt: now + GAME_TAB_LEASE_TTL_MS,
      };

      try {
        localStorage.setItem(this.getGameTabLeaseKey(), JSON.stringify(refreshedLease));
        const confirmedLease = this.readGameTabLease();
        if (!confirmedLease
          || confirmedLease.ownerId !== this.localWriterId
          || Number(confirmedLease.fencingToken || 0) !== Number(this.gameLeaseToken || 0)) {
          this.handleGameTabLeaseLost(confirmedLease);
          return false;
        }
        this.gameLeaseExpiresAt = refreshedLease.expiresAt;
        return true;
      } catch (error) {
        console.error("Unable to refresh the local game lease.", error);
        return false;
      }
    },
    startGameTabHeartbeat() {
      if (this.gameLeaseHeartbeatTimer || typeof window === 'undefined') return;
      this.gameLeaseHeartbeatTimer = window.setInterval(
        () => this.refreshGameTabLease(),
        GAME_TAB_HEARTBEAT_MS
      );
    },
    stopGameTabHeartbeat() {
      if (!this.gameLeaseHeartbeatTimer) return;
      if (typeof window !== 'undefined') {
        window.clearInterval(this.gameLeaseHeartbeatTimer);
      } else {
        clearInterval(this.gameLeaseHeartbeatTimer);
      }
      this.gameLeaseHeartbeatTimer = null;
    },
    releaseGameTabLease() {
      this.stopGameTabHeartbeat();
      if (!this.gameSerial || this.localBranchId) return false;

      const lease = this.readGameTabLease();
      const ownsLease = lease
        && lease.ownerId === this.localWriterId
        && Number(lease.fencingToken || 0) === Number(this.gameLeaseToken || 0);
      if (!ownsLease) return false;

      try {
        localStorage.removeItem(this.getGameTabLeaseKey());
        this.gameLeaseOwnerId = null;
        this.gameLeaseExpiresAt = 0;
        this.postGameTabMessage({
          type: 'lease-released',
          gameSerial: this.gameSerial,
          ownerId: this.localWriterId,
          fencingToken: this.gameLeaseToken,
        });
        return true;
      } catch (error) {
        console.error("Unable to release the local game lease.", error);
        return false;
      }
    },
    setupGameTabCoordination() {
      if (typeof window === 'undefined') return;

      window.addEventListener('storage', this.handleGameTabStorageEvent);
      window.addEventListener('pagehide', this.handlePageHide);

      if (typeof window.BroadcastChannel === 'function') {
        this.gameTabChannel = new window.BroadcastChannel(`2pick-game-tabs:${this.postSerial}`);
        this.gameTabChannel.addEventListener('message', this.handleGameTabBroadcast);
      }

      if (!this.gameLeaseMonitorTimer) {
        this.gameLeaseMonitorTimer = window.setInterval(
          () => this.monitorGameTabLease(),
          GAME_TAB_MONITOR_MS
        );
      }
    },
    teardownGameTabCoordination(releaseLease = false) {
      if (releaseLease) {
        this.releaseGameTabLease();
      } else {
        this.stopGameTabHeartbeat();
      }

      if (this.gameTabTakeoverTimer) {
        clearTimeout(this.gameTabTakeoverTimer);
        this.gameTabTakeoverTimer = null;
      }
      if (this.gameLeaseMonitorTimer) {
        clearInterval(this.gameLeaseMonitorTimer);
        this.gameLeaseMonitorTimer = null;
      }
      if (typeof window !== 'undefined') {
        window.removeEventListener('storage', this.handleGameTabStorageEvent);
        window.removeEventListener('pagehide', this.handlePageHide);
      }
      if (this.gameTabChannel) {
        this.gameTabChannel.removeEventListener('message', this.handleGameTabBroadcast);
        this.gameTabChannel.close();
        this.gameTabChannel = null;
      }
    },
    postGameTabMessage(message) {
      if (!this.gameTabChannel) return;
      try {
        this.gameTabChannel.postMessage(message);
      } catch (error) {
        console.warn("Unable to notify another game tab.", error);
      }
    },
    handleGameTabBroadcast(event) {
      const message = event && event.data ? event.data : null;
      if (!message || String(message.gameSerial) !== String(this.gameSerial)) return;
      if (message.ownerId === this.localWriterId) return;

      if (message.type === 'lease-acquired') {
        this.handleGameTabStorageEvent({ key: this.getGameTabLeaseKey() });
      } else if (message.type === 'lease-released') {
        const currentLease = this.readGameTabLease();
        if (!this.isGameTabLeaseActive(currentLease)) {
          this.queueAutomaticGameTabTakeover();
        }
      }
    },
    handleGameTabStorageEvent(event) {
      if (!this.gameSerial || !event) return;
      if (event.key !== this.getGameTabLeaseKey()) return;

      // Storage/Broadcast events may be queued. Always inspect the current lease
      // instead of trusting an older event payload that arrived after a takeover.
      const lease = this.readGameTabLease();
      if (this.isGameTabLeaseActive(lease) && lease.ownerId !== this.localWriterId) {
        const incomingToken = Number(lease.fencingToken || 0);
        const canonicalState = this.parseLocalJson(
          localStorage.getItem(this.getCanonicalLocalGameStateKey())
        );
        const canonicalToken = Number(canonicalState && canonicalState.writerLeaseToken || 0);
        const canonicalOwnedByThisTab = canonicalState
          && canonicalState.writerId === this.localWriterId
          && canonicalToken === Number(this.gameLeaseToken || 0);

        // A heartbeat that started before takeover must not restore an older
        // fencing token. Reassert the newer owner instead of stepping down.
        if (incomingToken < Number(this.gameLeaseToken || 0)
          || (incomingToken === Number(this.gameLeaseToken || 0) && canonicalOwnedByThisTab)) {
          this.reassertGameTabLease();
          return;
        }
        this.handleGameTabLeaseLost(lease);
      } else if (!lease && this.isGameTabReadOnly) {
        this.queueAutomaticGameTabTakeover();
      }
    },
    reassertGameTabLease() {
      if (!this.gameSerial || !this.gameLeaseToken || this.localBranchId) return false;
      const now = Date.now();
      const lease = {
        schemaVersion: GAME_TAB_LEASE_VERSION,
        gameSerial: this.gameSerial,
        ownerId: this.localWriterId,
        fencingToken: this.gameLeaseToken,
        heartbeatAt: now,
        expiresAt: now + GAME_TAB_LEASE_TTL_MS,
      };
      try {
        localStorage.setItem(this.getGameTabLeaseKey(), JSON.stringify(lease));
        this.gameLeaseOwnerId = this.localWriterId;
        this.gameLeaseExpiresAt = lease.expiresAt;
        this.isGameTabReadOnly = false;
        this.startGameTabHeartbeat();
        return true;
      } catch (error) {
        console.error("Unable to reassert the local game lease.", error);
        return false;
      }
    },
    handleGameTabLeaseLost(lease = null) {
      if (this.localBranchId) return;

      this.stopGameTabHeartbeat();
      this.gameLeaseOwnerId = lease ? lease.ownerId : null;
      this.gameLeaseExpiresAt = lease ? Number(lease.expiresAt || 0) : 0;
      this.isGameTabReadOnly = true;
      this.isVoting = false;

      if (this.hasUnpersistedLocalProgress) {
        this.forkCurrentLocalProgress('multi_tab_divergence');
      }
    },
    monitorGameTabLease() {
      if (!this.gameSerial || this.localBranchId || this.isHostingGameRank || this.isBetGameClient) {
        return;
      }

      if (this.ownsGameTabLease()) {
        if (this.gameLeaseExpiresAt - Date.now() < GAME_TAB_LEASE_TTL_MS / 2) {
          this.refreshGameTabLease();
        }
        return;
      }

      const lease = this.readGameTabLease();
      if (this.isGameTabLeaseActive(lease)) {
        this.handleGameTabLeaseLost(lease);
        return;
      }

      if (this.isGameTabReadOnly) {
        this.queueAutomaticGameTabTakeover();
      }
    },
    queueAutomaticGameTabTakeover() {
      if (this.gameTabTakeoverTimer || !this.isGameTabReadOnly || this.localBranchId) return;
      const delay = 50 + Math.floor(Math.random() * 200);
      this.gameTabTakeoverTimer = setTimeout(() => {
        this.gameTabTakeoverTimer = null;
        this.takeOverGameTab(false);
      }, delay);
    },
    takeOverGameTab(force = true) {
      if (!this.gameSerial || this.localBranchId) return false;
      if (!this.loadFromLocalStorage(true)) return false;
      if (!this.acquireGameTabLease(force)) return false;

      if (!this.claimLatestCanonicalGameState()) {
        this.releaseGameTabLease();
        this.isGameTabReadOnly = true;
        return false;
      }

      this.isGameTabReadOnly = false;
      this.hasUnpersistedLocalProgress = false;
      if (this.isClientMode) {
        this.resumeLocalGame(true);
      } else {
        this.nextRound(null);
      }
      this.resetTimer();
      this.startTimer();
      return true;
    },
    claimLatestCanonicalGameState(maxAttempts = 3) {
      for (let attempt = 0; attempt < maxAttempts; attempt++) {
        // 取得 fencing token 後重讀，涵蓋接管同時舊分頁剛完成的最後一次寫入。
        if (!this.loadFromLocalStorage(true) || !this.ownsGameTabLease()) {
          return false;
        }
        if (this.saveToLocalStorage()) {
          return true;
        }
      }
      return false;
    },
    ensureGameTabWriteAccess(showNotice = true) {
      if (!this.shouldCoordinateGameTabs()) return true;
      if (this.ownsGameTabLease() || this.acquireGameTabLease(false)) return true;
      if (showNotice) this.notifyReadOnlyGameTab();
      return false;
    },
    notifyReadOnlyGameTab() {
      const now = Date.now();
      if (now - this.lastGameTabNoticeAt < 3000) return;
      this.lastGameTabNoticeAt = now;
      Swal.fire({
        icon: 'info',
        toast: true,
        position: 'top-end',
        timer: 3000,
        text: this.$t('game.multi_tab.read_only'),
      });
    },
    forkCurrentLocalProgress(reason = 'multi_tab_divergence') {
      if (this.localBranchId) return true;

      this.stopGameTabHeartbeat();
      this.localBranchId = `${this.gameSerial}-${this.localWriterId}`;
      this.localBranchReason = reason;
      this.isGameTabReadOnly = false;
      this.isClientMode = true;
      this.isLocalOnlyAfterBatchConflict = true;
      this.cloudSyncDisabledReason = reason;
      this.inFlightBatch = null;
      this.recoveredInterruptedBatch = null;
      this.pendingFinalBatchVote = false;
      this.selectLocalBranch(this.localBranchId);
      return this.saveToLocalStorage();
    },
    handlePageHide() {
      if (this.isClientMode) {
        this.saveToLocalStorage();
      }
      this.releaseGameTabLease();
    },
    handleBeforeUnload() {
        if (this.isClientMode) {
          // beforeunload 不送 HTTP；無論目前有沒有待送票，都再保存
          // 當前配對、本地賽程與 outbox，讓任意時點重整都可恢復。
          this.saveToLocalStorage();
        }
        this.releaseGameTabLease();
    },
    loadGameSerialFromCookie() {
      return this.game_serial = this.$cookies.get(this.postSerial);
    },
    getGameSerial() {
      return this.$cookies.get(this.postSerial);
    },
    loadGameSetting() {
      axios
        .get(this.getGameSettingEndpoint)
        .then((res) => {
          this.error403WhenLoad = false;
          this.post = res.data.data;
        })
        .catch((error) => {
          if (error.response.status === 403) {
            this.error403WhenLoad = true;
          }
        });
    },
    getRoomVotes() {
      const route = this.getRoomVotesEndpoint.replace("_serial", this.gameRoomSerial);
      const prams = {
        params: {
          game_serial: this.gameSerial
        }
      };
      axios.get(route, prams)
        .then((res) => {
          this.gameRoomVotes = res.data.data;
        });
    },
    toggleShowGameRoomVotes() {
      this.showGameRoomVotes = !this.showGameRoomVotes;
      if (this.showGameRoomVotes) {
        this.getRoomVotes();
        if (!this.isListeningGameBet) {
          this.isListeningGameBet = setInterval(() => {
            this.getRoomVotes();
          }, 5 * 1000); // 5 seconds
        }
      } else {
        if (this.isListeningGameBet) {
          clearInterval(this.isListeningGameBet);
          this.isListeningGameBet = false;
        }
      }
    },
    getGameRoom() {
      axios
        .get(this.getRoomEndpoint.replace("_serial", this.gameRoomSerial))
        .then((response) => {
          this.gameRoom = response.data.data;
          this.gameBetRanks = this.gameRoom.ranks;
          // clear ranks to prevent duplicate
          this.gameRoom.ranks = null;
          let promise = new Promise((resolve) => {
            resolve();
          })
          if (response.data.data.current_round) {
            promise = this.handleAnimationAfterNextRound(response.data.data.current_round);
          }

          this.enableTooltip();
        });
    },

    // 目前定時getRoomVotes抓取投票結果
    // listenGameBet() {
    //   if (this.gameRoomSerial) {
    //     const channel = "game-room." + this.gameRoomSerial + ".game-serial." + this.gameSerial;
    //     Echo.channel(channel).listen(".GameBet", (data) => {
    //       this.gameRoomVotes = data;
    //     });
    //   }
    // },
    listenNotifyVoted() {
      if (this.gameRoomSerial) {
        Echo.channel("game-room." + this.gameRoomSerial).listen(".NotifyVoted", (data) => {
          this.showBetResult(data)
            .then(() => {
              this.showNextBetRound(data);
            });
        });
      }
    },
    listenGameRoomRefresh() {
      if (this.gameRoomSerial) {
        Echo.channel("game-room." + this.gameRoomSerial).listen(".GameRoomRefresh", (data) => {
          this.handleAnimationAfterNextRound(data.next_round, true)
            .then(() => {
              this.enableTooltip();
            });
        });
      }
    },
    listenGameBetRank() {
      if (this.gameRoomSerial) {
        Echo.channel("game-room." + this.gameRoomSerial).listen(".GameBetRank", (data) => {
          this.gameBetRanks = data;
          // find the key of current user
          // merge top_10 and bottom_10
          let top10 = this.gameBetRanks.top_10;
          let bottom10 = this.gameBetRanks.bottom_10;
          let currentUserRank = null;
          let allRanks = top10.concat(bottom10);
          allRanks.forEach((rank, index) => {
            if (rank.user_id === this.gameRoom.user.user_id) {
              currentUserRank = rank;
            }
          });

          if (currentUserRank) {
            this.gameRoom.user = currentUserRank;
          } else {
            this.getRoomUser();
          }
        });
      }
    },
    getRoomUser() {
      axios
        .get(this.getRoomUserEndpoint.replace("_serial", this.gameRoomSerial))
        .then((response) => {
          this.gameRoom.user = response.data.data;
        });
    },
    leaveGameRoom() {
      if (this.gameRoomSerial) {
        Echo.leave("game-room." + this.gameRoomSerial);
        Echo.leave("game-room." + this.gameRoomSerial + ".game-serial." + this.gameSerial);
      }
    },
    showBetResult(notifyData) {
      return new Promise((resolve) => {
        if (!this.currentBetRecord) {
          resolve();
          return;
        }
        const isBetSuccess = notifyData.winner_id === this.currentBetRecord.winner_id;
        this.currentBetRecord = null;
        if (isBetSuccess) {
          this.showFirework = true;
          setTimeout(() => {
            this.showFirework = false;
            resolve();
          }, 2000);
        } else {
          this.showBetFailed = true;
          setTimeout(() => {
            this.showBetFailed = false;
            resolve();
          }, 2000);
        }
      })
    },
    showNextBetRound(notifyData) {
      if (notifyData.next_round) {
        this.handleAnimationAfterNextRound(notifyData.next_round, true);
      } else {
        this.gameRoom.is_game_completed = true;
        this.finishingGame = true;
        this.isVoting = false;
        this.clearMatchHistory();
      }
    },
    isBetBefore() {
      return (
        this.gameRoom &&
        this.gameRoom.bet &&
        this.gameRoom.current_round &&
        this.gameRoom.bet.hash === this.gameRoom.current_round.hash
      );
    },
    toggleEditNickname() {
      this.isEditingNickname = !this.isEditingNickname;
      if (this.isEditingNickname) {
        this.newNickname = this.gameRoom.user.nickname;
      } else {
        this.newNickname = "";
      }
    },
    saveNickname() {
      axios
        .put(this.updateRoomProfileEndpoint.replace("_serial", this.gameRoomSerial), {
          nickname: this.newNickname,
        })
        .then((response) => {
          this.isEditingNickname = false;
          this.gameRoom.user.name = this.newNickname;
          // update rank
          if (this.gameBetRanks) {
            this.gameBetRanks.top_10.forEach((rank, index) => {
              if (rank.user_id === this.gameRoom.user.user_id) {
                rank.name = this.newNickname;
              }
            });
            this.gameBetRanks.bottom_10.forEach((rank, index) => {
              if (rank.user_id === this.gameRoom.user.user_id) {
                rank.name = this.newNickname;
              }
            });
          }
        })
        .catch((error) => {
          if (error.response.status === 429) {
            Swal.fire({
              icon: "error",
              toast: true,
              text: this.$t("You can only change your nickname once per hour"),
            });
          } else {
            Swal.fire({
              icon: "error",
              toast: true,
              text: this.$t("An error occurred. Please try again later."),
            });
          }
        });
    },

    getCurrentCandidates() {
        // 確保 le (左邊) 和 re (右邊) 物件存在
        if (this.le && this.re) {
            return [this.le.id, this.re.id];
        }
        return null;
    },
    // Room server
    handleCreatedRoom(data, roomUrl) {
      this.gameRoomSerial = data.serial
      this.gameRoom = data;
      this.gameBetRanks = this.gameRoom.ranks;
      this.gameRoomUrl = roomUrl;
      this.runInBackGameRoom = false;
      this.enableTooltip();
      this.isClientMode = false;
      Vue.nextTick(() => {
        QRCode.toCanvas(document.getElementById('qrcode'), roomUrl, {
          width: 160,
        });
      });

      if (!this.isHostingGameRank) {
        this.isHostingGameRank = true;
        Echo.channel("game-room." + this.gameRoomSerial)
          .listen(".GameBetRank", (data) => {
            this.gameBetRanks = data;
            this.gameRoom.total_users = data.total_users;
            this.enableTooltip();
          });
      }

      if (!this.autoRefreshRoomInterval) {
        this.autoRefreshRoomCounter = 0;
        this.autoRefreshRoomInterval = setInterval(() => {
          if (this.autoRefreshRoomCounter >= 3) {
            return ;
          }
          this.autoRefreshRoomCounter++;
          const route = this.getRoomEndpoint.replace("_serial", this.gameRoomSerial);
          const params = {
            params: {
              q: 'rank'
            }
          };
          axios.get(route, params)
            .then((response) => {
              this.gameRoom = response.data.data;
              this.gameBetRanks = this.gameRoom.ranks;
            });
        }, 5 * 1000);
      }

      if (this.unsentVotes.length > 0) {
        this.sendBatchVotes();
      }
    },
    isSameUser(rank) {
      return this.gameRoom && this.gameRoom.user && rank.user_id === this.gameRoom.user.user_id;
    },
    toogleRoomInvitation() {
      this.showRoomInvitation = !this.showRoomInvitation;

    },
    tipMethod(rank) {
      return `勝率:${rank.accuracy}% (${rank.total_correct} / ${rank.total_played})`;
    },
    accessGame() {
      if (this.inputPassword) {
        axios.defaults.headers.common["Authorization"] = this.inputPassword;
      } else {
        this.isInvalidPassword = true;
        return;
      }

      axios
        .get(this.accessEndpoint)
        .then((response) => {
          if (response.status === 200) {
            this.hideInvalidPasswordHint();
            this.loadGameSetting();
          } else {
            this.showInvalidPasswordHint();
            this.knownIncorrectPassword = true;
          }
        })
        .catch((error) => {
          if (error.response.status === 403) {
            this.showInvalidPasswordHint();
            this.knownIncorrectPassword = true;
          } else if (error.response.status === 429) {
            Swal.fire({
              icon: "error",
              toast: true,
              text: this.$t("You have tried too many times. Please try again later."),
            });
          } else {
            Swal.fire({
              icon: "error",
              toast: true,
              text: this.$t("An error occurred. Please try again later."),
            });
          }
        });
    },
    // game
    showInvalidPasswordHint() {
      this.invalidPasswordWhenLoad = true;
      Swal.fire({
        icon: "error",
        position: "top-end",
        timer: 3000,
        toast: true,
        text: this.$t("game.invalid_password"),
      });
    },
    hideInvalidPasswordHint() {
      this.invalidPasswordWhenLoad = false;
    },

    createGame() {
      if (this.creatingGame) return;

      // 建立新遊戲前，移除該主題的舊進度與上一局本地排行榜。
      const activeForeignLease = this.getActiveForeignCanonicalGameLease();
      if (activeForeignLease) {
        this.gameLeaseOwnerId = activeForeignLease.ownerId;
        this.gameLeaseExpiresAt = Number(activeForeignLease.expiresAt || 0);
        this.isGameTabReadOnly = true;
        this.notifyReadOnlyGameTab();
        return;
      }
      this.releaseGameTabLease();
      const wasLocalBranch = !!this.localBranchId;
      if (!this.clearLocalStorage(true)) {
        this.notifyReadOnlyGameTab();
        return;
      }
      if (wasLocalBranch) {
        this.localBranchId = null;
        this.localBranchReason = null;
        this.selectLocalBranch(null);
        if (!this.clearLocalStorage(true)) {
          return;
        }
      }
      this.clearLocalRankResult();

      // 清空時間軸
      this.clearMatchHistory();

      const data = {
        post_serial: this.postSerial,
        element_count: this.elementsCount,
        password: this.inputPassword,
      };
      this.creatingGame = true;
      axios
        .post(this.createGameEndpoint, data)
        .then((res) => {
          this.gameSerial = res.data.game_serial;
          this.serverVoteCount = Number(res.data.server_vote_count || 0);
          this.showMatchHistory = true;
          this.resetTimer();
          this.startTimer();
          this.keepGameCookie();
          if (this.isHostingGameRank || this.isClientMode === false) {
              // --- 多人模式 (Server Mode) ---
              this.isClientMode = false;
              if (this.saveToLocalStorage()) {
                this.nextRound(null, false);
              }
          } else {
              // --- 單人模式 (Client Mode) ---
              this.initClientSideGame();
          }
        })
        .catch((error) => {
           if (error.response && error.response.status === 422) {
              Swal.fire({
                icon: "error",
                toast: true,
                text: this.$t("The number of elements must be at least 2."),
              });
           } else {
              Swal.fire({ icon: "error", toast: true, text: this.$t("An error occurred.") });
           }
        })
        .finally(() => {
          this.creatingGame = false;
        });
      $("#gameSettingPanel").modal("hide");
    },

    // 初始化前端遊戲
    initClientSideGame() {
      this.localVotes = [];
      this.unsentVotes = [];
      this.localElements = [];
      this.existingElementIds.clear();
      this.localStateSchemaVersion = LOCAL_GAME_STATE_VERSION;
      this.localStateRevision = 0;
      this.localBranchId = null;
      this.localBranchReason = null;
      this.isGameTabReadOnly = false;
      this.hasUnpersistedLocalProgress = false;
      this.selectLocalBranch(null);
      this.selectLocalRankResult(null);
      this.localVoteSequence = 0;
      this.currentLocalMatch = null;
      this.inFlightBatch = null;
      this.recoveredInterruptedBatch = null;
      this.isLocalOnlyAfterBatchConflict = false;
      this.cloudSyncDisabledReason = null;

      const url = this.getGameElementsEndpoint.replace("_serial", this.gameSerial);
      const initialLimit = 32;
      const params = { params: { limit: initialLimit} };
      this.isDataLoading = true;

      axios.get(url, params)
        .then(res => {
            this.updateServerVoteCount(res.data.server_vote_count);
            this.processNewElements(res.data.data);

            this.clientState = {
                stage: 1,
                matchIndex: 0,
                stageStartCount: this.elementsCount,
                matchesInStage: 0,
                targetMatches: 0
            };

            this.updateStageConfig();
            if (!this.saveToLocalStorage()) {
              this.isDataLoading = false;
              return;
            }
            this.nextLocalRound();

            // 啟動背景抓取剩餘資料
            if (this.localElements.length < this.elementsCount) {
              setTimeout(() => {
                this.fetchRemainingElements();
              }, 30000); // 30秒後執行
            }
        })
        .catch(err => {
            console.error("Failed to load elements", err);
        });
    },

    // 過濾並處理新資料
    processNewElements(newElements) {
        if (!newElements || newElements.length === 0) return 0;

        let addedCount = 0;

        newElements.forEach(e => {
            // 檢查 ID 是否已存在
            if (!this.existingElementIds.has(e.id)) {
                // 加入 Set
                this.existingElementIds.add(e.id);

                // Client mode 的本地賽程是唯一真相來源。背景 API 只補齊
                // 尚未載入的參賽者內容，不得用遠端淘汰狀態改寫本地分支。
                this.localElements.push({
                    ...e,
                    local_win_count: 0,
                    local_eliminated: false,
                    local_played: 0,
                    local_is_ready: true
                });

                addedCount++;
            }
        });

        return addedCount;
    },

    isEnabledGameElementFlag(value) {
      return value === true || value === 1 || value === "1";
    },

    updateServerVoteCount(value) {
      const parsed = Number(value);
      if (Number.isInteger(parsed) && parsed >= 0) {
        this.serverVoteCount = parsed;
      }
    },

    fetchRemainingElements(retryCount = 0) {
      if (this.isGameTabReadOnly && !this.localBranchId) return;

      // 檢查目標達成：數量已足夠
      if (this.localElements.length >= this.elementsCount) {
          console.log("All elements loaded successfully.");
          return;
      }

      // 安全閥
      if (retryCount > 10) {
          console.warn("Max retries reached. Stopping background fetch.");
          return;
      }

      const url = this.getGameElementsEndpoint.replace("_serial", this.gameSerial);

      const requestLimit = this.elementsCount;

      axios.get(url, { params: { limit: requestLimit } })
          .then(res => {
              if (!this.ensureGameTabWriteAccess(false)) return;
              const data = res.data.data;

              if (data && data.length > 0) {
                  // 過濾並加入 (利用 Set 查重)
                  const actuallyAdded = this.processNewElements(data);

                  this.saveToLocalStorage();

                  if (this.localElements.length < this.elementsCount) {
                      setTimeout(() => {
                          const nextRetry = actuallyAdded === 0 ? retryCount + 1 : 0;
                          this.fetchRemainingElements(nextRetry);
                      }, 30000);
                  }
              } else {
                  console.warn("Backend returned no data.");
              }
          })
          .catch(err => {
              console.error("Background fetch failed", err);
              setTimeout(() => {
                  this.fetchRemainingElements(retryCount + 1);
              }, 30000);
          });
    },

    getLocalGameStateKey(forceCanonical = false) {
      if (!forceCanonical && this.localBranchId) {
        return this.getLocalBranchStateKey();
      }
      return this.getCanonicalLocalGameStateKey();
    },

    normalizeStoredVote(vote, fallbackId) {
      if (!vote || vote.winner_id === undefined || vote.loser_id === undefined) {
        return null;
      }

      return {
        local_vote_id: vote.local_vote_id || fallbackId,
        winner_id: vote.winner_id,
        loser_id: vote.loser_id,
      };
    },

    createLocalVote(winnerId, loserId) {
      this.localVoteSequence += 1;
      return {
        local_vote_id: `${this.gameSerial}:${this.localVoteSequence}`,
        winner_id: winnerId,
        loser_id: loserId,
      };
    },

    // localStorage 的單一 key 寫入是同步且原子的。每個會影響賽程的欄位都放在
    // 同一份 versioned snapshot，避免重整時只恢復到一半的狀態。
    saveToLocalStorage(claimOwnership = false) {
      if (!this.gameSerial) return false;

      try {
        const isBranchWrite = !!this.localBranchId;
        if (!isBranchWrite && this.shouldCoordinateGameTabs()) {
          const hasWriteLease = claimOwnership
            ? this.acquireGameTabLease(true)
            : this.ensureGameTabWriteAccess(false);
          if (!hasWriteLease) return false;
        }

        const storageKey = this.getLocalGameStateKey();
        const currentRawState = localStorage.getItem(storageKey);
        let currentStoredState = null;

        if (currentRawState) {
          currentStoredState = JSON.parse(currentRawState);
        }

        if (currentStoredState && currentStoredState.gameSerial) {
          const sameGame = String(currentStoredState.gameSerial) === String(this.gameSerial);
          if (!sameGame) {
            console.warn("Skipped stale local game write for a different game serial.");
            return false;
          }

          const ownedByAnotherPage = currentStoredState.writerId
            && currentStoredState.writerId !== this.localWriterId;
          const storedRevision = Number(currentStoredState.localStateRevision || 0);
          const isNewerSnapshot = storedRevision > this.localStateRevision;
          const storedLeaseToken = Number(currentStoredState.writerLeaseToken || 0);
          const ownsNewerLease = !isBranchWrite
            && Number(this.gameLeaseToken || 0) > storedLeaseToken;

          // 即使取得新 lease，也必須先載入最新版 snapshot，不能用舊記憶體
          // 覆蓋較新的本地進度。fencing token 只允許接管已載入的同一 revision。
          if (isNewerSnapshot || (ownedByAnotherPage && !ownsNewerLease && !isBranchWrite)) {
            console.warn("Skipped stale local game write from an older page instance.");
            return false;
          }
        }

        const storedRevision = currentStoredState
          ? Number(currentStoredState.localStateRevision || 0)
          : 0;
        const nextRevision = Math.max(this.localStateRevision, storedRevision) + 1;
        const stateToSave = {
          schemaVersion: LOCAL_GAME_STATE_VERSION,
          writerId: this.localWriterId,
          writerLeaseToken: isBranchWrite ? null : this.gameLeaseToken,
          localStateRevision: nextRevision,
          gameSerial: this.gameSerial,
          localBranchId: this.localBranchId,
          localBranchReason: this.localBranchReason,
          localElements: this.localElements,
          localVotes: this.localVotes,
          localVoteSequence: this.localVoteSequence,
          unsentVotes: this.unsentVotes,
          inFlightBatch: this.inFlightBatch,
          serverVoteCount: this.serverVoteCount,
          localOnlyAfterBatchConflict: this.isLocalOnlyAfterBatchConflict,
          cloudSyncDisabledReason: this.cloudSyncDisabledReason,
          clientState: this.clientState,
          currentLocalMatch: this.currentLocalMatch,
          existingElementIds: Array.from(this.existingElementIds),
          elementsCount: this.elementsCount,
          matchHistory: this.matchHistory,
          lastVotePair: this.lastVotePair,
          batchVoteInterval: this.batchVoteInterval,
          updatedAt: Date.now(),
          clientMode: this.isClientMode,
        };

        if (!isBranchWrite && this.shouldCoordinateGameTabs() && !this.ownsGameTabLease()) {
          console.warn("Skipped local game write after losing the tab lease.");
          this.handleGameTabLeaseLost(this.readGameTabLease());
          return false;
        }

        localStorage.setItem(storageKey, JSON.stringify(stateToSave));
        this.localStateRevision = nextRevision;
        this.hasUnpersistedLocalProgress = false;
        return true;
      } catch (error) {
        console.error("Storage save failed", error);
        return false;
      }
    },

    // 只還原資料，不在這裡推進賽程或送 request。呼叫端完成所有欄位還原後，
    // 再透過 resumeLocalGame 恢復畫面，避免重整時 nextLocalRound 被執行兩次。
    loadFromLocalStorage(forceCanonical = false) {
      if (this.isHostingGameRank) return false;

      if (forceCanonical) {
        this.localBranchId = null;
        this.localBranchReason = null;
      } else if (!this.localBranchId) {
        this.localBranchId = this.getSelectedLocalBranchId();
      }

      let storageKey = this.getLocalGameStateKey(forceCanonical);
      let savedData = localStorage.getItem(storageKey);
      if (!savedData && this.localBranchId && !forceCanonical) {
        this.localBranchId = null;
        this.localBranchReason = null;
        this.selectLocalBranch(null);
        storageKey = this.getCanonicalLocalGameStateKey();
        savedData = localStorage.getItem(storageKey);
      }
      if (!savedData) return false;

      try {
        const parsed = JSON.parse(savedData);
        if (!parsed || !parsed.gameSerial) return false;

        const savedClientMode = parsed.clientMode !== undefined ? parsed.clientMode : false;
        this.localStateSchemaVersion = Number(parsed.schemaVersion || 1);
        this.localStateRevision = Number(parsed.localStateRevision || 0);
        this.gameSerial = parsed.gameSerial;
        this.localBranchId = parsed.localBranchId || null;
        this.localBranchReason = parsed.localBranchReason || null;
        this.isGameTabReadOnly = false;
        this.hasUnpersistedLocalProgress = false;
        if (this.localBranchId) {
          this.selectLocalBranch(this.localBranchId);
        }
        if (savedClientMode === false) {
          this.isClientMode = false;
          // Server mode 的時間軸獨立儲存在 matchHistory key。重整後必須在
          // 取得 gameSerial 後載入，否則左欄會因 showMatchHistory=false 顯示廣告。
          this.loadMatchHistory();
          console.log("Restored as Server Mode from localStorage");
          return true;
        }

        if (!Array.isArray(parsed.localElements) || !parsed.clientState) {
          return false;
        }

        this.isClientMode = true;
        this.localElements = parsed.localElements;
        this.clientState = parsed.clientState;

        const rawLocalVotes = Array.isArray(parsed.localVotes) ? parsed.localVotes : [];
        this.localVotes = rawLocalVotes
          .map((vote, index) => this.normalizeStoredVote(
            vote,
            `${this.gameSerial}:${index + 1}`
          ))
          .filter(Boolean);

        const rawUnsentVotes = Array.isArray(parsed.unsentVotes) ? parsed.unsentVotes : [];
        const unsentOffset = Math.max(0, this.localVotes.length - rawUnsentVotes.length);
        this.unsentVotes = rawUnsentVotes
          .map((vote, index) => {
            const matchingLocalVote = this.localVotes[unsentOffset + index];
            const fallbackId = matchingLocalVote
              ? matchingLocalVote.local_vote_id
              : `${this.gameSerial}:pending:${index + 1}`;
            return this.normalizeStoredVote(vote, fallbackId);
          })
          .filter(Boolean);

        this.localVoteSequence = Math.max(
          Number(parsed.localVoteSequence || 0),
          this.localVotes.length
        );

        // in-flight votes remain in unsentVotes until a success response is
        // durably acknowledged. Include the embedded copy as a recovery guard
        // for records written by an interrupted older page instance.
        const interruptedBatch = parsed.inFlightBatch || null;
        if (interruptedBatch && Array.isArray(interruptedBatch.votes)) {
          const knownVoteIds = new Set(this.unsentVotes.map(vote => vote.local_vote_id));
          interruptedBatch.votes.forEach((vote, index) => {
            const normalized = this.normalizeStoredVote(
              vote,
              `${this.gameSerial}:recovered:${index + 1}`
            );
            if (normalized && !knownVoteIds.has(normalized.local_vote_id)) {
              this.unsentVotes.push(normalized);
              knownVoteIds.add(normalized.local_vote_id);
            }
          });
        }
        this.recoveredInterruptedBatch = interruptedBatch;
        this.inFlightBatch = null;

        if (parsed.serverVoteCount !== undefined) {
          this.updateServerVoteCount(parsed.serverVoteCount);
        } else {
          this.serverVoteCount = Math.max(0, this.localVotes.length - this.unsentVotes.length);
        }

        this.isLocalOnlyAfterBatchConflict = this.localBranchId
          ? true
          : parsed.localOnlyAfterBatchConflict === true;
        this.cloudSyncDisabledReason = parsed.cloudSyncDisabledReason || null;
        this.currentLocalMatch = parsed.currentLocalMatch || null;
        this.matchHistory = Array.isArray(parsed.matchHistory)
          ? parsed.matchHistory.slice(0, KEEP_VOTE_RECORD_COUNT)
          : [];
        this.lastVotePair = parsed.lastVotePair || null;
        this.batchVoteInterval = Number(parsed.batchVoteInterval || BATCH_VOTE_SAVE_INTERVAL);

        this.existingElementIds = new Set(
          Array.isArray(parsed.existingElementIds)
            ? parsed.existingElementIds
            : this.localElements.map(element => element.id)
        );
        if (parsed.elementsCount) {
          this.elementsCount = parsed.elementsCount;
        }

        // 舊存檔相容
        if (!this.clientState.stageStartCount) {
          if (this.clientState.stage === 1) {
            this.clientState.stageStartCount = this.elementsCount || this.localElements.length;
          } else {
            this.clientState.stageStartCount = this.localElements
              .filter(element => !element.local_eliminated).length;
          }
        }

        return true;
      } catch (error) {
        console.error("Failed to parse saved game state", error);
        return false;
      }
    },

    // 清除 LocalStorage
    clearLocalStorage(force = false) {
      const storageKey = this.getLocalGameStateKey();

      if (force) {
        try {
          const savedState = this.parseLocalJson(localStorage.getItem(storageKey));
          const savedGameSerial = savedState && savedState.gameSerial;
          if (!this.localBranchId && savedGameSerial) {
            const lease = this.readGameTabLease(savedGameSerial);
            const activeForeignLease = lease
              && String(lease.gameSerial) === String(savedGameSerial)
              && Number(lease.expiresAt || 0) > Date.now()
              && lease.ownerId !== this.localWriterId;
            if (activeForeignLease) {
              this.gameLeaseOwnerId = lease.ownerId;
              this.gameLeaseExpiresAt = Number(lease.expiresAt || 0);
              this.isGameTabReadOnly = true;
              return false;
            }
          }

          localStorage.removeItem(storageKey);
          if (this.localBranchId) this.selectLocalBranch(null);
          return true;
        } catch (error) {
          console.error("Local game cleanup failed", error);
          return false;
        }
      }

      try {
        const savedData = localStorage.getItem(storageKey);
        if (!savedData) return true;

        const parsed = JSON.parse(savedData);
        const sameGame = !parsed.gameSerial
          || String(parsed.gameSerial) === String(this.gameSerial);
        const ownedByThisPage = !parsed.writerId
          || parsed.writerId === this.localWriterId
          || (this.localBranchId && parsed.localBranchId === this.localBranchId);
        const storedRevision = Number(parsed.localStateRevision || 0);

        if (!sameGame || !ownedByThisPage || storedRevision > this.localStateRevision) {
          console.warn("Skipped stale local game cleanup from an older page instance.");
          return false;
        }

        localStorage.removeItem(storageKey);
        if (this.localBranchId) this.selectLocalBranch(null);
        return true;
      } catch (error) {
        console.error("Local game cleanup failed", error);
        return false;
      }
    },

    saveLocalRankResult(cloudSyncPending = false) {
      if (!this.gameSerial || !Array.isArray(this.localElements)) {
        return false;
      }

      if (!this.localBranchId
        && this.shouldCoordinateGameTabs()
        && !this.ownsGameTabLease()) {
        this.hasUnpersistedLocalProgress = true;
        if (!this.forkCurrentLocalProgress('multi_tab_late_completion')) {
          return false;
        }
      }

      const key = this.localBranchId
        ? `gameresult_${this.postSerial}_branch_${this.localBranchId}`
        : `gameresult_${this.postSerial}`;
      const result = {
        schemaVersion: LOCAL_GAME_STATE_VERSION,
        writerId: this.localWriterId,
        gameSerial: this.gameSerial,
        localBranchId: this.localBranchId,
        localBranchReason: this.localBranchReason,
        localElements: this.localElements,
        localVotes: this.localVotes,
        unsentVotes: this.unsentVotes,
        inFlightBatch: this.inFlightBatch,
        serverVoteCount: this.serverVoteCount,
        matchHistory: this.matchHistory,
        completedAt: Date.now(),
        localOnlyAfterBatchConflict: this.isLocalOnlyAfterBatchConflict,
        cloudSyncPending: cloudSyncPending && !this.isLocalOnlyAfterBatchConflict,
        cloudSyncDisabledReason: this.cloudSyncDisabledReason,
      };

      try {
        localStorage.setItem(key, JSON.stringify(result));
        this.selectLocalRankResult(this.localBranchId ? key : null);
        return true;
      } catch (error) {
        console.error("Local rank result save failed", error);
        return false;
      }
    },

    clearLocalRankResult() {
      const key = `gameresult_${this.postSerial}`;
      localStorage.removeItem(key);
      this.selectLocalRankResult(null);
    },

    // 移植後端的 NextRound 計算邏輯
    calculateNextRoundNumber(remain) {
        let powerOf2 = Math.pow(2, Math.floor(Math.log2(remain)));
        if (remain === powerOf2) {
            powerOf2 = powerOf2 / 2;
        }
        return remain - powerOf2;
    },

    // 更新階段設定
    updateStageConfig() {
        let baseCount = 0;

        // 1. 決定基準人數
        if (this.clientState.stage === 1) {
            // Stage 1 使用總設定人數
            baseCount = this.elementsCount;
        } else {
            // Stage 2+ 使用存活人數
            baseCount = this.localElements.filter(e => !e.local_eliminated).length;
        }

        // 2. 計算目標場次 (ofRound)
        if (this.clientState.stage === 1) {
            this.clientState.targetMatches = Math.ceil(baseCount / 2);
        } else {
            this.clientState.targetMatches = this.calculateNextRoundNumber(baseCount);
        }
    },

    syncRemoteDataToLocal(responseData) {
        // 1. 基礎資料同步 (保持不變)
        if (responseData.total_count) {
            this.elementsCount = responseData.total_count;
        }
        this.updateServerVoteCount(responseData.server_vote_count);

        const remoteElements = responseData.data || [];
        this.localElements = [];
        this.existingElementIds.clear();

        remoteElements.forEach(e => {
            this.existingElementIds.add(e.id);
            const isEliminated = this.isEnabledGameElementFlag(e.is_eliminated);
            const winCount = parseInt(e.win_count || 0, 10);

            // 計算 played 次數
            const playedCount = winCount + (isEliminated ? 1 : 0);

            const localEl = {
                ...e,
                local_win_count: winCount,
                local_eliminated: isEliminated,
                local_played: playedCount,
                // 直接使用後端的 ready 狀態
                local_is_ready: this.isEnabledGameElementFlag(e.is_ready) && !isEliminated
            };

            this.localElements.push(localEl);
        });

        this.localVotes = [];
        this.unsentVotes = [];
        this.localStateSchemaVersion = LOCAL_GAME_STATE_VERSION;
        this.localStateRevision = 0;
        this.localBranchId = null;
        this.localBranchReason = null;
        this.isGameTabReadOnly = false;
        this.hasUnpersistedLocalProgress = false;
        this.selectLocalBranch(null);
        this.localVoteSequence = 0;
        this.currentLocalMatch = null;
        this.inFlightBatch = null;
        this.recoveredInterruptedBatch = null;
        this.isLocalOnlyAfterBatchConflict = false;
        this.cloudSyncDisabledReason = null;
        let stage = 1;
        let matchesInStage = 0;
        let targetMatches = Math.ceil(this.elementsCount/2);
        let remoteRemainElementsCount = this.localElements.filter(e => !e.local_eliminated).length;
        let stageStartCount = this.elementsCount;
        let remainElementsCount = this.elementsCount;
        while(remainElementsCount > remoteRemainElementsCount){
            if(matchesInStage >= targetMatches){
                stage += 1;
                matchesInStage = 0;
                stageStartCount = remainElementsCount;
                targetMatches = this.calculateNextRoundNumber(remainElementsCount);
            }
            matchesInStage++;
            remainElementsCount--;
            // console.log(`[stage ${stage}] After match ${matchesInStage} / ${targetMatches}, remain elements: ${remainElementsCount}`);
        }

        this.clientState = {
            stage: stage,
            matchIndex: matchesInStage,
            stageStartCount: stageStartCount,
            matchesInStage: matchesInStage,
            targetMatches: targetMatches
        };

    },

    resumeLocalGame(leaseAlreadyClaimed = false, refreshCurrentMatch = false) {
      this.isClientMode = true;
      this.showMatchHistory = true;

      // 新版 snapshot 已包含 history；只有舊存檔才回讀獨立 key。
      if (this.localStateSchemaVersion < LOCAL_GAME_STATE_VERSION
        && this.matchHistory.length === 0) {
        this.loadMatchHistory();
      }

      this.recoveredInterruptedBatch = null;
      this.inFlightBatch = null;
      this.isBatchVoting = false;
      this.isCloudSaving = false;
      this.pendingFinalBatchVote = false;

      const canWriteGame = this.localBranchId || this.ensureGameTabWriteAccess(false);
      if (!canWriteGame) {
        this.restoreCurrentLocalMatch();
        this.isDataLoading = false;
        return;
      }

      if (!leaseAlreadyClaimed && !this.saveToLocalStorage()) {
        this.isDataLoading = false;
        return;
      }

      let refreshMatchOptions = null;
      if (refreshCurrentMatch) {
        // Page refresh should show a newly randomized pairing instead of
        // restoring the pair that was visible before the reload. This only
        // replaces an unvoted display: it must not advance matchIndex or any
        // other durable tournament progress.
        const isReplacingDisplayedMatch = !!this.currentLocalMatch;
        this.currentLocalMatch = null;
        // Keep the previous displayed match in durable storage until the new
        // pairing is selected and saved. A crash during redraw can therefore
        // never leave a half-written snapshot or lose the recoverable match.
        refreshMatchOptions = {
          randomizeReadyCandidates: true,
          preserveMatchIndex: isReplacingDisplayedMatch,
        };
      }

      if (this.localElements.length < this.elementsCount) {
        this.fetchRemainingElements();
      }

      const activeCount = this.localElements.filter(element => !element.local_eliminated).length;
      if (activeCount < 2) {
        if (this.isLocalOnlyAfterBatchConflict) {
          this.finishLocalOnlyGame();
        } else {
          this.sendBatchVotes();
        }
        return;
      }

      if (refreshCurrentMatch) {
        this.nextLocalRound(refreshMatchOptions);
      } else if (!this.restoreCurrentLocalMatch()) {
        this.nextLocalRound();
      }

      // 重新整理前可能已有一批 request 到達伺服器，也可能尚未送達。
      // outbox 仍保留同一批票，稍後以相同內容安全重送。
      if (!this.isLocalOnlyAfterBatchConflict && this.unsentVotes.length > 0) {
        setTimeout(() => this.sendPartialBatchVotes(), 0);
      }
    },

    // 繼續遊戲時，只要存在可用的本地 snapshot 就永遠採用它；遠端進度
    // 不得自動覆蓋本地分支。只有完全沒有本地資料時才以遠端建立起始 snapshot。
    continueGame() {
      if (this.loadFromLocalStorage()) {
        if (this.isClientMode) {
          this.resumeLocalGame(false, true);
        } else {
          if (this.ensureGameTabWriteAccess(false)) {
            this.saveToLocalStorage();
            this.nextRound(null);
          }
        }
        $("#gameSettingPanel").modal("hide");
        this.resetTimer();
        this.startTimer();
        return;
      }

      const remoteData = this.userLastGame;
      if (remoteData) {
        this.gameSerial = remoteData.serial;
        this.isClientMode = true;
        const url = this.getGameElementsEndpoint.replace("_serial", this.gameSerial);

        this.isDataLoading = true;
        axios.get(url, {
          params: {
            limit: 1024,
            t: Date.now(),
          },
        }).then(response => {
          this.syncRemoteDataToLocal(response.data);
          this.currentLocalMatch = null;
          if (this.saveToLocalStorage()) {
            this.nextLocalRound();
          }
        }).catch(error => {
          console.error("Failed to restore remote game", error);
          Swal.fire({
            icon: 'error',
            title: 'Error',
            text: this.$t('An error occurred. Please try again later.'),
          });
        }).finally(() => {
          this.isDataLoading = false;
          this.resetTimer();
          this.startTimer();
        });

        $("#gameSettingPanel").modal("hide");
        return;
      }

      const cookieSerial = this.$cookies.get(this.postSerial);
      if (cookieSerial) {
        this.gameSerial = cookieSerial;
        this.nextRound(null, false);
      }
      $("#gameSettingPanel").modal("hide");
      this.loadMatchHistory();
      this.resetTimer();
      this.startTimer();
    },

    hintSelect() {
      this.showPopover = true;
      if (this.timeout) {
        clearTimeout(this.timeout);
      }
      this.timeout = setTimeout(() => {
        this.showPopover = false;
      }, 3000);
    },

    // nextRound 修正 Null Error
    nextRound(data, resetAnimation = true) {
      // 1. 如果是 Client Mode，直接呼叫本地邏輯
      const shouldUseLocalRound = this.isLocalOnlyAfterBatchConflict
        || (!this.isHostingGameRank && this.isClientMode);
      if (shouldUseLocalRound && (data == null || (data && !data.data))) {
          this.nextLocalRound();
          return;
      }

      // 2. Server Mode 的標準處理
      if (data == null) {
        const url = this.nextRoundEndpoint.replace("_serial", this.gameSerial);
        axios.get(url)
          .then((res) => { this.nextRound(res.data); })
          .catch((error) => {
            this.handleNextRoundError(data, error);
          });
        return;
      }

      this.handleAnimationAfterNextRound(data.data, resetAnimation);
    },

    restoreCurrentLocalMatch(resetAnimation = false) {
        if (!this.currentLocalMatch) return false;

        const left = this.localElements.find(element => {
          return String(element.id) === String(this.currentLocalMatch.left_id)
            && !element.local_eliminated;
        });
        const right = this.localElements.find(element => {
          return String(element.id) === String(this.currentLocalMatch.right_id)
            && !element.local_eliminated;
        });

        if (!left || !right || String(left.id) === String(right.id)) {
          this.currentLocalMatch = null;
          this.saveToLocalStorage();
          return false;
        }

        this.handleAnimationAfterNextRound({
          current_round: this.currentLocalMatch.current_round,
          of_round: this.currentLocalMatch.of_round,
          remain_elements: this.currentLocalMatch.remain_elements,
          total_elements: this.currentLocalMatch.total_elements,
          stage_start_count: this.currentLocalMatch.stage_start_count,
          elements: [left, right],
        }, resetAnimation);
        return true;
    },

    shuffleLocalMatchCandidates(elements) {
        const shuffled = elements.slice();
        for (let index = shuffled.length - 1; index > 0; index--) {
            const randomIndex = Math.floor(Math.random() * (index + 1));
            [shuffled[index], shuffled[randomIndex]] = [shuffled[randomIndex], shuffled[index]];
        }
        return shuffled;
    },

    nextLocalRound(options = {}) {
        const randomizeReadyCandidates = options.randomizeReadyCandidates === true;
        const preserveMatchIndex = options.preserveMatchIndex === true;
        if (!this.ensureGameTabWriteAccess(false)) return;
        let activeElements = this.localElements.filter(e => !e.local_eliminated);

        if (activeElements.length < 2) {
             this.sendBatchVotes();
             return;
        }

        // 沒有已保存的配對時才建立下一個隨機對戰。
        if (this.restoreCurrentLocalMatch()) {
            return;
        }

        let needTransition = false;

        if (this.clientState.matchesInStage >= this.clientState.targetMatches) {
            needTransition = true;
        }

        if (needTransition) {
            this.clientState.stage++;

            //進入新的一輪，更新 "本輪起始人數"
            this.clientState.stageStartCount = activeElements.length;
            this.clientState.matchesInStage = 0;

            this.localElements.forEach(e => {
                if (!e.local_eliminated) {
                    e.local_is_ready = true;
                }
            });
            this.updateStageConfig();
            this.saveToLocalStorage();
            this.nextLocalRound({ randomizeReadyCandidates });
            return;
        }

        let el1, el2;
        let readyElements = activeElements.filter(e => e.local_is_ready);

        if (randomizeReadyCandidates || this.clientState.stage !== 2) {
            // A reload may draw any candidate who has not fought in this
            // stage. Fisher-Yates avoids the bias of sort(() => random).
            readyElements = this.shuffleLocalMatchCandidates(readyElements);
        } else {
            // Normal stage-2 progression keeps the existing fairness rule;
            // shuffle first so candidates with the same played count are
            // still selected uniformly.
            readyElements = this.shuffleLocalMatchCandidates(readyElements)
                .sort((a, b) => a.local_played - b.local_played);
        }

        if (readyElements.length >= 2) {
            el1 = readyElements[0];
            el2 = readyElements[1];
        } else if (readyElements.length === 1) {
            el1 = readyElements[0];
            const notReadyElements = this.shuffleLocalMatchCandidates(
                activeElements.filter(e => !e.local_is_ready)
            );
            if (notReadyElements.length > 0) {
                el2 = notReadyElements[0];
            } else {
                this.sendBatchVotes();
                return;
            }
        } else {
            this.sendBatchVotes();
            return;
        }

        const eliminatedCount = this.localElements.filter(e => e.local_eliminated).length;
        const realRemainCount = this.elementsCount - eliminatedCount;
        if (!preserveMatchIndex) {
            this.clientState.matchIndex++;
        }
        const currentMatchInStage = this.clientState.matchesInStage + 1;
        const mockGameData = {
            current_round: currentMatchInStage,
            of_round: this.clientState.targetMatches,
            remain_elements: realRemainCount,
            total_elements: this.elementsCount,
            stage_start_count: this.clientState.stageStartCount,
            elements: [el1, el2]
        };

        this.currentLocalMatch = {
            left_id: el1.id,
            right_id: el2.id,
            current_round: mockGameData.current_round,
            of_round: mockGameData.of_round,
            remain_elements: mockGameData.remain_elements,
            total_elements: mockGameData.total_elements,
            stage_start_count: mockGameData.stage_start_count,
        };
        this.saveToLocalStorage();

        this.handleAnimationAfterNextRound(mockGameData, true);
    },

    handleAnimationAfterNextRound(game, resetAnimation) {
      return new Promise((resolve, reject) => {
        this.updateGame(game);
        resolve();
      })
        .then(() => {
          if (resetAnimation) {
            this.resetPlayerPosition();
            // this.scrollToLastPosition();
            this.resetPlayingStatus();
            // destroy viwer
            if (this.$refs.rightViewer) {
              this.$refs.rightViewer.updateViewer();
            }
            if (this.$refs.leftViewer) {
              this.$refs.leftViewer.updateViewer();
            }
            this.errorImages = [];
            this.isDataLoading = false;
            setTimeout(() => {
              this.showAllPlayers();
              this.doPlay(this.le, this.isLeftPlaying, "left");
              this.doPlay(this.re, this.isRightPlaying, "right");
            }, 300);
          } else {
            this.doPlay(this.le, this.isLeftPlaying, "left");
            this.doPlay(this.re, this.isRightPlaying, "right");
          }
        })
        .catch((error) => {
          console.error("Next round error:", error);
          this.handleNextRoundError(game, error || { response: { status: 500 }});})
        .finally(() => {
          this.isDataLoading = false;
          this.isVoting = false;
          $("#google-ad-container").css("top", "0");
          if (this.isMobileScreen) {
            $("#google-ad2").css("top", "0");
          }
          this.startAdRefreshTimer();
        });
    },
    // 更新遊戲數據 (核心同步樞紐)
    updateGame(game) {
      // console.log("isClientMode:", this.isClientMode);
      // console.log("Received game data:", game);
      // Server Mode
      if (!this.isClientMode) {
          const matchIdx = (game.current_round || 1) - 1;

          if ((game.current_round || 1) === 1) {
              this.localElements.forEach(e => {
                  if (!e.local_eliminated) {
                      e.local_is_ready = true;
                  }
              });
          }

          game.stage_start_count = game.remain_elements + matchIdx;

          // 同步後端數據到 clientState
          this.clientState.matchesInStage = matchIdx;
          this.clientState.targetMatches = game.of_round || 1;
          this.clientState.stageStartCount = game.stage_start_count;
          this.elementsCount = game.total_elements || this.elementsCount;
          // console.log("Updated clientState from server:", this.clientState);
          this.saveToLocalStorage();
      }

      // 更新主要遊戲物件 (這一刻，UI 才會跟著變動)
      this.game = game;
      this.requestTogawaAdDisplay();

      if (this.game.current_round == 1 || this.currentRemainElement == false) {
        this.currentRemainElement = this.game.remain_elements;
      }
      if (this.le && this.game.elements[0] && this.le.id !== this.game.elements[0].id) {
        this.leftImageLoaded = false;
      }
      if (this.re && this.game.elements[1] && this.re.id !== this.game.elements[1].id) {
        this.rightImageLoaded = false;
      }
      this.le = this.game.elements[0];
      this.re = this.game.elements[1];
      // console.log("Left Element:", this.le);
      // console.log("Right Element:", this.re);
    },
    requestTogawaAdDisplay() {
      if (typeof window === 'undefined' || typeof window.dispatchEvent !== 'function') {
        return;
      }

      const displayAd = () => {
        window.dispatchEvent(new Event('ranking:display-togawa-ad'));
      };

      if (typeof this.$nextTick === 'function') {
        this.$nextTick(displayAd);
      } else {
        displayAd();
      }
    },
    handleNextRoundError(data, error) {
      if (error.response.status === 429) {
        let timerInterval;
        Swal.fire({
          html:
            this.$t("You have voted too quickly. Please try again later.") + "(<b></b>)",
          timer: 5000,
          timerProgressBar: true,
          icon: "error",
          didOpen: () => {
            Swal.showLoading();
            const timer = Swal.getPopup().querySelector("b");
            timerInterval = setInterval(() => {
              let timeInMs = Swal.getTimerLeft();
              let timeInSec = timeInMs / 1000;
              timer.textContent = `${timeInSec.toFixed(1)}s`; // toFixed(1) will round to 1 decimal place
            }, 100);
          },
          willClose: () => {
            clearInterval(timerInterval);
          },
        }).then((result) => {
          if (result.dismiss === Swal.DismissReason.timer) {
            this.nextRound(data);
          }
        });
      } else {
        Swal.fire({
          icon: "error",
          toast: true,
          text: this.$t("An error occurred. Please try again later."),
        }).then(() => {
          location.reload();
        });
      }
    },
    leftPlay() {
      const myPlayer = this.getYoutubePlayer(this.le);
      if (myPlayer) {
        // window.p1 = myPlayer;
        myPlayer.playVideo();
        myPlayer.unMute();
        this.isLeftPlaying = true;
      }
      const theirPlayer = this.getYoutubePlayer(this.re);
      if (theirPlayer) {
        // window.p2 = theirPlayer;
        theirPlayer.pauseVideo();
        theirPlayer.mute();
        this.isRightPlaying = false;
      }

      const myVideoPlyaer = this.getVideoPlayer("left-video-player");
      if (myVideoPlyaer) {
        myVideoPlyaer.play();
        this.isLeftPlaying = true;
      }

      const theirVideoPlyaer = this.getVideoPlayer("right-video-player");
      if (theirVideoPlyaer) {
        theirVideoPlyaer.pause();
        this.isRightPlaying = false;
      }
    },
    leftWin(event) {
      if (!this.ensureGameTabWriteAccess()) return;
      this.rememberedScrollPosition = document.documentElement.scrollTop;
      this.isVoting = true;
      let sendWinnerData = () => {
        if (this.isBetGameClient) {
          this.bet(this.le, this.re);
        } else {
          this.vote(this.le, this.re, 'left');
        }
      };

      this.bounceThumbUp(event.target.children[0]);
      $("#left-player").css("z-index", "100");
      $("#right-player").css("opacity", 0.5);

      this.leftReady = false;

      if (this.isMobileScreen) {
        if (this.isBetGameClient) {
          // bet game send data firstly
          sendWinnerData();

          // 移除觀眾視角的投票動畫
          // let loseAnimate = $("#right-player").animate({ opacity: "0" }, 500).promise();
          // $.when(loseAnimate).then(() => {
          //   this.destroyRightPlayer();
          //   $('#right-part').css('display', 'none');
          //   this.leftReady = true;
          // });

        } else {
          $("#rounds-session").animate({ opacity: 0 }, 100, "linear");
          // move #left-plyaer to the certical center of screen
          let scrollPosition = window.scrollY;
          let verticalCenter = $(window).height() / 2 - $("#left-player").height() / 2;
          let playOriginalOffset = $("#left-player").offset().top;
          let titleHeight = $("#game-title").height();
          let screenCenterPosition = Math.max(
            verticalCenter + scrollPosition - playOriginalOffset,
            0
          );
          screenCenterPosition = Math.min(screenCenterPosition, 350);
          let winAnimate = $("#left-player")
            .animate({ top: screenCenterPosition }, null, () => {
              setTimeout(() => {
                this.leftReady = true;
              }, 1200);
            })
            .promise();
          let adTopPosition = titleHeight + screenCenterPosition;
          $("#google-ad-container").animate({ top: adTopPosition });
          let offset = 30;
          let adBottomPosition = -$("#right-player").height() - offset + screenCenterPosition;
          $("#google-ad2").animate({ top: adBottomPosition });
          let loseAnimate = $("#right-player").animate({ opacity: "0" }, 500).promise();
          $.when(loseAnimate).then(() => {
            sendWinnerData();
          });
        }
      } else {
        if (this.isBetGameClient) {
          // bet game send data firstly
          sendWinnerData();
        }else{
          let winAnimate = $("#left-player")
            .animate({ left: "50%" }, 500, () => {
              if (this.isBetGameClient) {
                this.leftReady = true;
              } else {
                $("#left-player")
                  .delay(500)
                  .animate({ top: "-2000" }, 500, () => {
                    this.leftReady = true;
                  });
              }
            })
            .promise();
          let loseAnimate = $("#right-player")
            .animate({ left: "2000" }, 500, () => {
              $("#right-player").css("opacity", "0");
            })
            .promise();

          $.when(loseAnimate).then(() => {
            if (this.isBetGameClient) {
              this.destroyRightPlayer();
            } else {
              sendWinnerData();
            }
          });
        }
      }
    },
    rightPlay() {
      const myYTPlayer = this.getYoutubePlayer(this.re);
      if (myYTPlayer) {
        myYTPlayer.playVideo();
        myYTPlayer.unMute();
        this.isRightPlaying = true;
      }
      const theirYTPlayer = this.getYoutubePlayer(this.le);
      if (theirYTPlayer) {
        theirYTPlayer.pauseVideo();
        theirYTPlayer.mute();
        this.isLeftPlaying = false;
      }

      const myVideoPlyaer = this.getVideoPlayer("right-video-player");
      if (myVideoPlyaer) {
        myVideoPlyaer.play();
        this.isRightPlaying = true;
      }

      const theirVideoPlyaer = this.getVideoPlayer("left-video-player");
      if (theirVideoPlyaer) {
        theirVideoPlyaer.pause();
        this.isLeftPlaying = false;
      }
    },
    rightWin(event) {
      if (!this.ensureGameTabWriteAccess()) return;
      this.rememberedScrollPosition = document.documentElement.scrollTop;
      this.isVoting = true;
      let sendWinnerData = () => {
        if (this.isBetGameClient) {
          this.bet(this.re, this.le);
        } else {
          this.vote(this.re, this.le, 'right');
        }
      };

      this.rightReady = false;

      if (event) {
        this.bounceThumbUp(event.target.children[0]);
      }
      $("#right-player").css("z-index", "100");
      $("#left-player").css("opacity", 0.5);

      if (this.isMobileScreen) {
        if (this.isBetGameClient) {
          // bet game send data firstly
          sendWinnerData();

          // 移除觀眾視角的投票動畫
          // let loseAnimate = $("#left-player").animate({ opacity: "0" }, 500).promise();
          // $.when(loseAnimate).then(() => {
          //   this.destroyLeftPlayer();
          //   $('#left-part').css('display', 'none');
          //   this.rightReady = true;
          //   sendWinnerData();
          // });


        } else {
          $("#rounds-session").animate({ opacity: 0 }, 100, "linear");
          // move #right-plyaer to the certical center of screen
          let scrollPosition = window.scrollY;
          let verticalCenter = $(window).height() / 2 - $("#right-player").height() / 2;
          let playOriginalOffset = $("#right-player").offset().top;
          let titleHeight = $("#game-title").height();
          let screenCenterPosition = Math.min(
            verticalCenter + scrollPosition - playOriginalOffset,
            0
          );
          screenCenterPosition = Math.max(screenCenterPosition, -320);
          // animate right player buttom to top
          let winAnimate = $("#right-player")
            .animate({ top: screenCenterPosition }, null, () => {
              setTimeout(() => {
                this.rightReady = true;
              }, 1200);
            })
            .promise();
          let offset = 30;
          let adTopPosition = titleHeight + screenCenterPosition + $("#left-player").height() + offset;
          $("#google-ad-container").animate({ top: adTopPosition });
          let adBottomPosition = screenCenterPosition;
          let ad2Offset = 0;
          $("#google-ad2").animate({ top: adBottomPosition - ad2Offset });
          let loseAnimate = $("#left-player").animate({ opacity: "0" }, 500).promise();
          $.when(loseAnimate).then(() => {
            sendWinnerData();
          });
        }
      } else {
        if (this.isBetGameClient) {
           // bet game send data firstly
          sendWinnerData();
        } else {
          let winAnimate = $("#right-player")
            .animate({ left: "-50%" }, 500, () => {
              if (this.isBetGameClient) {
                this.rightReady = true;
              } else {
                $("#right-player")
                  .delay(500)
                  .animate({ top: "-2000" }, 500, () => {
                    $("#right-player").hide();
                    this.rightReady = true;
                  });
              }
            })
            .promise();

          let loseAnimate = $("#left-player")
            .animate({ left: "-2000" }, 500, () => {
              $("#left-player").css("opacity", "0");
            })
            .promise();

          $.when(loseAnimate).then(() => {
            if (this.isBetGameClient) {
              this.destroyLeftPlayer();
            }
            sendWinnerData();
          });
        }
      }
    },
    destroyRightPlayer() {
      // make right as a dummy image
      this.re = {
        id: this.re.id,
        type: "image",
      };
    },
    destroyLeftPlayer() {
      // make left as a dummy image
      this.le = {
        id: this.le.id,
        type: "image",
      };
    },
    handleLeftLoaded() {
      this.leftImageLoaded = true;
    },
    handleRightLoaded() {
      this.rightImageLoaded = true;
    },
    bounceThumbUp(element) {
      // add class fa-bounce
      $(element).addClass("fa-bounce");
      setTimeout(() => {
        this.removeBoundThumbUp(element);
      }, 1000);
    },
    removeBoundThumbUp(element) {
      // remove fa-bounce
      $(element).removeClass("fa-bounce");
    },
    resetPlayerPosition() {
      $("#left-player").css("left", "0");
      $("#left-player").css("top", "0");
      $("#left-player").css("opacity", "0");
      $("#left-player").css("scale", "1");
      $("#left-player").removeClass("zoom-in");
      $("#left-player").css("z-index", "0");
      $('#left-part').css('display', 'block');

      $("#right-player").css("left", "0");
      $("#right-player").css("top", "0");
      $("#right-player").css("opacity", "0");
      $("#right-player").css("scale", "1");
      $("#right-player").removeClass("zoom-in");
      $("#right-player").css("z-index", "0");
      $('#right-part').css('display', 'block');

      $("#rounds-session").css("opacity", "0");
      $(".game-image-container img").css("object-fit", "contain");
    },
    scrollToLastPosition() {
      if (this.rememberedScrollPosition !== null) {
        window.scrollTo(0, this.rememberedScrollPosition);
      }
    },
    pauseAllVideo() {
      const player = this.getYoutubePlayer(this.le);
      if (player) {
        player.pauseVideo();
        player.seekTo(this.le.start_second);
      }

      const player2 = this.getYoutubePlayer(this.re);
      if (player2) {
        player2.pauseVideo();
        player2.seekTo(this.re.start_second);
      }
    },
    bet(winner, loser) {
      const route = this.betEndpoint.replace("_serial", this.gameRoomSerial);
      const data = {
        winner_id: winner.id,
        loser_id: loser.id,
        current_round: this.game.current_round,
        of_round: this.game.of_round,
        remain_elements: this.game.remain_elements,
      };
      axios.post(route, data).then(res => {
        this.currentBetRecord = {
          winner_id: winner.id,
          loser_id: loser.id,
        };
      }).catch((error) => {
        if (error.response.status === 429) {
          Swal.fire({
            icon: "error",
            toast: true,
            text: this.$t("You have voted too quickly. Please try again later."),
          });
        } else {
          Swal.fire({
            icon: "error",
            toast: true,
            text: this.$t("An error occurred. Please try again later."),
          }).then(() => {
            location.reload();
          });
        }
      });
    },

    vote(winner, loser, winSide = 'left') {
      if (!this.ensureGameTabWriteAccess()) return false;
      if (this.isLocalOnlyAfterBatchConflict) {
        this.handleClientVote(winner, loser, winSide);
      } else if (!this.isClientMode || this.isHostingGameRank) {
        const data = { game_serial: this.gameSerial, winner_id: winner.id, loser_id: loser.id };
        this.sendVote(data, winSide);
      } else {
        this.handleClientVote(winner, loser, winSide);
      }
    },

    handleClientVote(winner, loser, winSide = 'left') {
      if (!this.ensureGameTabWriteAccess()) return false;
      const winnerObj = this.localElements.find(e => e.id === winner.id);
      const loserObj = this.localElements.find(e => e.id === loser.id);

      if (winnerObj && loserObj) {
          this.hasUnpersistedLocalProgress = true;
          winnerObj.local_win_count++;
          winnerObj.local_played++;
          loserObj.local_played++;
          loserObj.local_eliminated = true;
          winnerObj.local_is_ready = false;
          loserObj.local_is_ready = false;
          this.clientState.matchesInStage++;

          const localVote = this.createLocalVote(winner.id, loser.id);
          this.localVotes.push(localVote);
          if (!this.isLocalOnlyAfterBatchConflict) {
            this.unsentVotes.push({ ...localVote });
          }

          // 目前配對已完成。這個欄位必須在同一份 snapshot 中清除，否則
          // 重整後會再次顯示剛投完的兩個選項。
          this.currentLocalMatch = null;

          // 紀錄最後一筆投票，用於時間軸
          this.lastVotePair = { winner_id: winner.id, loser_id: loser.id, winSide: winSide };
          this.recordMatchFromLastVote();

          // 所有本地運算與 outbox 先同步、原子地保存，之後才能進行動畫或 HTTP。
          const progressPersisted = this.saveToLocalStorage();
          if (!progressPersisted && this.isGameTabReadOnly) {
            this.forkCurrentLocalProgress('multi_tab_divergence');
          }

          // 預先計算剩餘人數
          const currentActiveCount = this.localElements.filter(e => !e.local_eliminated).length;

          // 最後一票先留下獨立結果，即使 final batch 尚未回應就重整或離頁，
          // 排行榜仍可使用完整的本地結果。
          if (currentActiveCount < 2) {
            this.saveLocalRankResult(true);
          }

          // 檢查是否需要觸發雲端備份
          // 只有在 "非最後一局" 的情況下才執行 Partial Batch Vote
          // 如果是最後一局 (currentActiveCount < 2)，則交由 handleAnimationAfterVoted 的 sendBatchVotes 統一送出
          if (!this.isLocalOnlyAfterBatchConflict
            && currentActiveCount >= 2
            && this.unsentVotes.length >= this.batchVoteInterval
            && !this.isCloudSaving) {
              this.sendPartialBatchVotes();
          }
      }

      const activeCount = this.localElements.filter(e => !e.local_eliminated).length;
      const mockRes = {
          data: {
              status: activeCount < 2 ? 'end_game' : 'processing',
              data: null
          }
      };
      this.handleAnimationAfterVoted(mockRes);
      return true;
    },

    sendPartialBatchVotes() {
      if (this.isGameTabReadOnly && !this.localBranchId) {
        return Promise.resolve(null);
      }
      if (this.isLocalOnlyAfterBatchConflict || this.unsentVotes.length === 0) {
        return Promise.resolve(null);
      }
      return this.submitBatchVotes(false);
    },

    // 送出最後一批；若雲端同步已停用，直接以本地結果完成。
    sendBatchVotes() {
      if (this.isGameTabReadOnly && !this.localBranchId) {
        return Promise.resolve(null);
      }
      if (this.isLocalOnlyAfterBatchConflict) {
        this.finishLocalOnlyGame();
        return Promise.resolve(null);
      }
      return this.submitBatchVotes(true);
    },

    /**
     * 所有 batch vote 共用同一個送出流程，避免 partial/final request 互相超車。
     */
    submitBatchVotes(isFinalBatch) {
      if (this.isGameTabReadOnly && !this.localBranchId) {
        return Promise.resolve(null);
      }
      if (this.isLocalOnlyAfterBatchConflict) {
        if (isFinalBatch) {
          this.finishLocalOnlyGame();
        }
        return Promise.resolve(null);
      }

      if (this.isBatchVoting) {
        if (isFinalBatch) {
          this.pendingFinalBatchVote = true;
          this.isDataLoading = true;
        }
        return Promise.resolve(null);
      }

      if (!isFinalBatch && this.unsentVotes.length === 0) {
        return Promise.resolve(null);
      }

      this.isBatchVoting = true;
      this.isCloudSaving = true;
      if (isFinalBatch) {
        this.isDataLoading = true;
      }

      let finalBatchHandled = false;

      return this.performBatchVote(isFinalBatch)
        .then(response => {
          if (response && response.ignoredBecauseLeaseLost) {
            finalBatchHandled = true;
            this.pendingFinalBatchVote = false;
            this.finishingGame = false;
            this.isDataLoading = false;
            this.isVoting = false;
            return;
          }

          this.batchVoteInterval = BATCH_VOTE_SAVE_INTERVAL;

          const shouldFinishGame = isFinalBatch || this.pendingFinalBatchVote;
          if (!shouldFinishGame) {
            return;
          }

          // 如果請求送出後又新增了投票，由 finally 再串接一次 final batch。
          if (this.unsentVotes.length > 0) {
            this.pendingFinalBatchVote = true;
            return;
          }

          this.pendingFinalBatchVote = false;
          finalBatchHandled = true;
          if (response) {
            this.handleSendVote(response);
          } else {
            this.finishLocalOnlyGame();
          }
        })
        .catch(error => {
          if (error && error.code === 'game_tab_lease_lost') {
            finalBatchHandled = true;
            this.pendingFinalBatchVote = false;
            this.finishingGame = false;
            this.isDataLoading = false;
            this.isVoting = false;
            this.handleGameTabLeaseLost(this.readGameTabLease());
            return;
          }

          const conflictReason = this.getCloudSyncStopReason(error);
          if (conflictReason) {
            const shouldFinishGame = isFinalBatch || this.pendingFinalBatchVote;
            finalBatchHandled = true;
            this.pendingFinalBatchVote = false;
            console.warn("Cloud vote sync stopped; preserving the local game state.", conflictReason);
            this.disableCloudSync(conflictReason);

            if (shouldFinishGame) {
              this.finishLocalOnlyGame();
            }
            return;
          }

          this.batchVoteInterval = (this.batchVoteInterval * 2) + 1;
          this.saveToLocalStorage();

          const responseData = error.response ? error.response.data : null;
          console.error("Batch vote failed", responseData || error);

          if (isFinalBatch || this.pendingFinalBatchVote) {
            this.pendingFinalBatchVote = false;
            this.finishingGame = false;
            this.isDataLoading = false;
            this.isVoting = false;

            // 單純網路錯誤仍保留佇列重試；後端明確拒絕這條
            // 本地分支時，才停用雲端同步。
            Swal.fire({
              icon: "error",
              text: this.$t("An error occurred. Please try again later."),
              allowOutsideClick: false,
            }).then(() => {
              this.sendBatchVotes();
            });
          }
        })
        .finally(() => {
          this.isBatchVoting = false;
          this.isCloudSaving = false;

          if (!finalBatchHandled && this.pendingFinalBatchVote) {
            this.pendingFinalBatchVote = false;
            this.sendBatchVotes();
          }
        });
    },

    /**
     * 只送出當下 outbox 的快照。請求前先將這份 batch
     * 持久化；只有成功 response 才會依 local_vote_id 確認並移除。
     */
    performBatchVote(isFinalBatch = false) {
      // 舊版 snapshot 沒有 local_vote_id，在第一次送出前補成穩定 ID。
      this.unsentVotes = this.unsentVotes.map(vote => {
        if (vote.local_vote_id) return vote;
        this.localVoteSequence += 1;
        return {
          ...vote,
          local_vote_id: `${this.gameSerial}:${this.localVoteSequence}`,
        };
      });

      const votesToSend = this.unsentVotes.map(vote => ({ ...vote }));
      if (votesToSend.length === 0) {
        return Promise.resolve(null);
      }

      const batch = {
        id: `${this.gameSerial}:batch:${this.localStateRevision + 1}:${votesToSend[0].local_vote_id}:${votesToSend[votesToSend.length - 1].local_vote_id}`,
        vote_ids: votesToSend.map(vote => vote.local_vote_id),
        votes: votesToSend,
        expected_vote_count: this.serverVoteCount,
        is_final: isFinalBatch,
        started_at: Date.now(),
      };

      this.inFlightBatch = batch;
      if (!this.saveToLocalStorage()) {
        this.inFlightBatch = null;
        const storageError = new Error("Local game state could not be persisted before cloud sync.");
        storageError.code = this.isGameTabReadOnly
          ? "game_tab_lease_lost"
          : "local_state_save_failed";
        return Promise.reject(storageError);
      }

      return axios.post(this.batchVoteEndpoint, {
        game_serial: this.gameSerial,
        expected_vote_count: batch.expected_vote_count,
        votes: batch.votes.map(vote => ({
          winner_id: vote.winner_id,
          loser_id: vote.loser_id,
        })),
      }, {
        timeout: BATCH_REQUEST_TIMEOUT_MS,
      }).then(response => {
        if (!this.localBranchId && !this.ownsGameTabLease()) {
          this.handleGameTabLeaseLost(this.readGameTabLease());
          return { ignoredBecauseLeaseLost: true };
        }
        this.updateServerVoteCount(response.data.server_vote_count);
        this.acknowledgeSubmittedBatch(batch);
        this.saveToLocalStorage();
        return response;
      }).catch(error => {
        if (error && error.code === "game_tab_lease_lost") {
          throw error;
        }
        // 明確失敗後請求已結束；in-flight 可清掉，但 outbox
        // 不動。若瀏覽器在 response 前重整，這段不會執行，
        // 已持久化的 in-flight 會在下次載入時恢復。
        if (this.inFlightBatch && this.inFlightBatch.id === batch.id) {
          this.inFlightBatch = null;
          this.saveToLocalStorage();
        }
        throw error;
      });
    },

    acknowledgeSubmittedBatch(batch) {
      const acknowledgedVoteIds = new Set(batch.vote_ids);
      this.unsentVotes = this.unsentVotes.filter(vote => {
        return !acknowledgedVoteIds.has(vote.local_vote_id);
      });

      if (this.inFlightBatch && this.inFlightBatch.id === batch.id) {
        this.inFlightBatch = null;
      }
    },

    getCloudSyncStopReason(error) {
      if (error && error.code === "local_state_save_failed") {
        return error.code;
      }

      const status = error && error.response ? error.response.status : null;
      const isPermanentClientRejection = Number.isInteger(status)
        && status >= 400
        && status < 500
        && ![408, 425, 429].includes(status);
      if (!isPermanentClientRejection) return null;

      const responseData = error.response.data || {};
      return responseData.reason || responseData.code || `http_${status}`;
    },

    disableCloudSync(reason = "game_state_conflict") {
      this.isLocalOnlyAfterBatchConflict = true;
      this.isClientMode = true;
      this.cloudSyncDisabledReason = reason;
      this.inFlightBatch = null;
      this.recoveredInterruptedBatch = null;
      this.pendingFinalBatchVote = false;
      this.saveToLocalStorage();
    },

    finishLocalOnlyGame() {
      const activeCount = this.localElements.filter(element => !element.local_eliminated).length;
      if (activeCount >= 2) {
        this.finishingGame = false;
        this.isDataLoading = false;
        return false;
      }

      if (this.status === "end_game") {
        return true;
      }

      this.status = "end_game";
      this.finishingGame = true;
      this.pendingFinalBatchVote = false;
      this.recordMatchFromLastVote();
      this.$cookies.remove(this.postSerial);
      // Rank.vue 需要這份本地結果。只有獨立結果保存成功後，才清掉可續玩的遊戲進度。
      if (this.saveLocalRankResult()) {
        this.clearLocalStorage();
      }
      this.clearMatchHistory();
      this.showGameResult();
      return true;
    },

    sendVote(data, winSide = 'left') {

      // 1. 在送出前，更新本地 localElements 的狀態
      // 這是為了確保本地端的 "淘汰名單" 與 Server 端同步
      const winnerObj = this.localElements.find(e => e.id === data.winner_id);
      const loserObj = this.localElements.find(e => e.id === data.loser_id);

      if (winnerObj && loserObj) {
          // 更新勝場與場次
          winnerObj.local_win_count++;
          winnerObj.local_played++;
          loserObj.local_played++;

          loserObj.local_eliminated = true;

          winnerObj.local_is_ready = false;
          loserObj.local_is_ready = false;
      }

      this.localVotes.push({
          winner_id: data.winner_id,
          loser_id: data.loser_id
      });

      // 紀錄最後一筆投票，用於時間軸
      this.lastVotePair = { winner_id: data.winner_id, loser_id: data.loser_id, winSide: winSide };

      axios
        .post(this.voteGameEndpoint, data)
        .then((res) => {
          this.handleAnimationAfterVoted(res);
        })
        .catch((error) => {
          if (error.response.status === 429) {
            Swal.fire({
              icon: "error",
              toast: true,
              text: this.$t("You have voted too quickly. Please try again later."),
            });
          } else {
            Swal.fire({
              icon: "error",
              toast: true,
              text: this.$t("An error occurred. Please try again later."),
            }).then(() => {
              location.reload();
            });
          }
          let interval = setInterval(() => {
            if (this.leftReady && this.rightReady) {
              this.resetPlayerPosition();

              this.resetPlayingStatus();
              clearInterval(interval);
              setTimeout(() => {
                this.showAllPlayers();
                this.isDataLoading = false;
                this.isVoting = false;
              }, 300);
            }
          }, 10);
        })
        .finally(() => { });
    },
    handleAnimationAfterVoted(res) {
      let interval = setInterval(() => {
        // console.log('leftReady: '+this.leftReady+' | rightReady: '+this.rightReady);
        if (this.leftReady && this.rightReady) {
          // 判斷最後一局的邏輯調整
          let isFinalRound = false;
          if (this.isClientMode) {
              const activeCount = this.localElements.filter(e => !e.local_eliminated).length;
              isFinalRound = activeCount < 2;
          } else {
              isFinalRound = (this.game.current_round == 1 && this.currentRemainElement == 2);
          }

          if (isFinalRound) {
            // final round
            this.isDataLoading = true;
            this.finishingGame = true;
          }

          if (!this.finishingGame) {
            // to void still playing video if next round loaded the same element
            this.pauseAllVideo();
          }

          clearInterval(interval);
          if (this.isMobileScreen) {
            Promise.all([
              $("#left-player").animate({ left: 300, opacity: 0 }, 150).promise(),
              $("#right-player").animate({ left: 300, opacity: 0 }, 150).promise(),
              $("#google-ad-container").animate({ top: 100, opacity: 0 }, 150).promise(),
              $("#google-ad2").animate({ top: 100, opacity: 0 }, 150).promise(),
            ]).then(() => {
              this.animationShowLeftPlayer = false;
              this.animationShowRightPlayer = false;
              this.animationShowRoundSession = false;
              this.isDataLoading = true;

              if (this.isClientMode && isFinalRound) {
                   this.sendBatchVotes();
              } else {
                   this.handleSendVote(res);
              }
            });
          } else {
            this.isDataLoading = true;
            if (this.isClientMode && isFinalRound) {
                 this.sendBatchVotes();
            } else {
                 this.handleSendVote(res);
            }
          }
        }
      }, 10);
    },
    showAllPlayers() {
      this.animationShowLeftPlayer = true;
      this.animationShowRightPlayer = true;
      this.animationShowRoundSession = true;
      $("#left-player").show();
      $("#right-player").show();
      $("#rounds-session").show();
      $("#left-player").css("opacity", "1");
      $("#right-player").css("opacity", "1");
      $("#rounds-session").css("opacity", "1");
      if (this.isMobileScreen) {
        $("#google-ad-container").css("opacity", "1");
        $("#google-ad2").css("opacity", "1");
      }
    },
    handleSendVote(res) {
      if (!this.localBranchId
        && this.shouldCoordinateGameTabs()
        && !this.ownsGameTabLease()) {
        this.handleGameTabLeaseLost(this.readGameTabLease());
        return;
      }
      if(this.autoRefreshRoomInterval){
        this.autoRefreshRoomCounter = 0;
      }
      // 先記錄思考時間，再重置計時器，避免被歸零
      this.recordMatchFromLastVote();
      this.resetTimer();
      this.status = res.data.status;
      if (this.status === "end_game") {
        this.$cookies.remove(this.postSerial);
        // 先留下完整本地結果，再清理可續玩 snapshot。如果
        // 專用結果寫入失敗，保留 snapshot 作為 Rank.vue fallback。
        const canClearLocalGame = !this.isClientMode || this.saveLocalRankResult(false);
        if (canClearLocalGame) {
          this.clearLocalStorage();
        }
        this.clearMatchHistory();
        this.showGameResult();
      } else {
        this.keepGameCookie();
        this.nextRound(res.data);
      }
    },
    keepGameCookie() {
      this.$cookies.set(this.postSerial, this.gameSerial, "1y");
    },
    resetPlayingStatus() {
      this.isLeftPlaying = false;
      this.isRightPlaying = false;
    },
    showGameSettingPanel: function () {
      $("#gameSettingPanel").modal("show");
    },
    showGameResult() {
      const url = this.getRankResultUrl();
      setTimeout(() => {
        this.gameResultUrl = url;
        window.open(url, "_self");
      }, 1000);
    },
    getRankResultUrl() {
      if (this.gameSerial) {
        return this.getRankRoute.replace("_serial", this.postSerial) + "?g=" + this.gameSerial;
      }
      return this.getRankRoute.replace("_serial", this.postSerial);
    },

    startTimer() {
      if (this.timerInterval) {
        return;
      }
      this.timerInterval = setInterval(() => {
        this.timerSeconds += 1;
      }, 1000);
    },
    resetTimer() {
      this.timerSeconds = 0;
    },
    stopTimer() {
      if (this.timerInterval) {
        clearInterval(this.timerInterval);
        this.timerInterval = null;
      }
    },

    // --- Timeline helpers ---
    getBestThumb(element) {
      if (!element) return '';
      return element.lowthumb_url || element.mediumthumb_url || element.thumb_url || '';
    },
    findElementById(id) {
      if (!id) return null;
      let found = this.localElements.find(e => e.id === id);
      if (!found) {
        if (this.le && this.le.id === id) found = this.le;
        if (this.re && this.re.id === id) found = this.re;
      }
      return found || null;
    },
    recordMatchFromLastVote() {
      if (!this.lastVotePair) return;
      const { winner_id, loser_id } = this.lastVotePair;
      const winner = this.findElementById(winner_id);
      const loser = this.findElementById(loser_id);
      if (!winner || !loser) return;

      // 使用與頁面相同的回合標籤邏輯
      let roundLabel = '';
      if (this.roundTitleCount <= 2) {
        roundLabel = this.$t('game_round_final');
      } else if (this.roundTitleCount <= 4) {
        roundLabel = this.$t('game_round_semifinal');
      } else if (this.roundTitleCount <= 8) {
        roundLabel = this.$t('game_round_quarterfinal');
      } else if (this.roundTitleCount <= 1024) {
        roundLabel = this.$t('game_round_of', { round: this.roundTitleCount });
      }

      const progressLabel = `${this.displayCurrentRound}/${this.displayTotalRound}`;
      const thinkingTime = this.timerSeconds;
      const winSide = this.lastVotePair.winSide || 'left';

      this.matchHistory.unshift({
        id: `${Date.now()}-${winner_id}-${loser_id}-${this.matchHistory.length}`,
        roundLabel,
        progressLabel,
        thinkingTime,
        winSide,
        winner: {
          title: winner.title,
          thumb: this.getLowThumbUrl(winner),
        },
        loser: {
          title: loser.title,
          thumb: this.getLowThumbUrl(loser),
        },
      });

      // 保留筆數
      if (this.matchHistory.length > KEEP_VOTE_RECORD_COUNT) {
        this.matchHistory.pop();
      }

      // Client mode 會緊接著把 history 和賽程寫入同一份 atomic
      // snapshot，不先寫獨立 key，避免只留下一半的投票狀態。
      if (!this.isClientMode) {
        this.saveMatchHistory();
      }

      // 清空暫存
      this.lastVotePair = null;
    },
    loadMatchHistory() {
      try {
        this.showMatchHistory = true;
        const key = `matchHistory_${this.postSerial}`;
        const saved = localStorage.getItem(key);
        if (saved && this.gameSerial) {
          const data = JSON.parse(saved);
          // 驗證資料格式、gameSerial並限制在 50 筆內
          if (data && data.gameSerial === this.gameSerial && Array.isArray(data.matches)) {
            this.matchHistory = data.matches.slice(0, 50);
          } else {
            // gameSerial不匹配，清空
            this.matchHistory = [];
          }
        }
      } catch (error) {
        console.error('Failed to load match history:', error);
      }
    },
    saveMatchHistory() {
      try {
        const key = `matchHistory_${this.postSerial}`;
        const data = {
          gameSerial: this.gameSerial,
          matches: this.matchHistory
        };
        localStorage.setItem(key, JSON.stringify(data));
      } catch (error) {
        console.error('Failed to save match history:', error);
      }
    },
    clearMatchHistory() {
      try {
        const key = `matchHistory_${this.postSerial}`;
        localStorage.removeItem(key);
        this.matchHistory = [];
      } catch (error) {
        console.error('Failed to clear match history:', error);
      }
    },
    getYoutubePlayer(element) {
      if (!element) {
        return null;
      }
      return _.get(this.$refs, element.id + ".player", null);
    },
    getVideoPlayer(id) {
      return document.getElementById(id);
    },
    getTwitchPlayer(element) {
      //check twitch-video-{{element.id}} is exist
      if (document.getElementById("twitch-video-" + element.id) === null) {
        return null;
      }

      if (element.twitchPlayer === undefined) {
        element.twitchPlayer =  Twitch.Embed("twitch-video-" + element.id, {
          width: "100%",
          height: this.elementHeight,
          video: element.video_id,
          layout: "video",
          autoplay: false,
          muted: false,
          time: this.formatTime(element.video_start_second),
        });
        // console.log(element.twitchPlayer);
      }

      return element.twitchPlayer;
    },
    doPlay(element, loud = false, name) {
      let player = null;
      if ((player = this.getYoutubePlayer(element))) {
        if (loud) {
          player.unMute();
        } else {
          player.mute();
        }
        this.initPlayerEventLister(player, element);
        player.getPlayerState().then((state) => {
          //resumed if video is paused
          if (state === 2) {
            player.playVideo();
          }
        });
      } else if ((player = this.getTwitchPlayer(element))) {
        if (element.video_source === "twitch_video") {

          player.seek(element.video_start_second);
        } else if (element.video_source === "twitch_clip") {
          //
        }
      }
    },
    initPlayerEventLister(player, element) {
      player.addEventListener("onStateChange", (event) => {
        let status = event.target.getPlayerState();
        // -1 – 未啟動
        // 0 - 已結束
        // 1 – 播放
        // 2 – 已暫停
        // 3 – 緩衝處理中
        // 5 – 隱藏影片
        // console.log(element.title +' | '+ status);
        if (status === 0 || status === -1) {
          player.seekTo(element.video_start_second, true);
        }
      });
    },
    videoHoverOut(myElement, theirElement, left) {
      if (this.isMobileScreen || this.isBetGameClient) {
        return;
      }

      this.isHoverIn = false;

    },
    videoHoverIn(myElement, theirElement, left) {
      if (this.isMobileScreen || this.isBetGameClient) {
        return;
      }

      // Set a flag to track if the mouse is still hovering
      this.mousePosition = left;
      this.isHoverIn = true;

      // Delay handling by 500ms
      setTimeout(() => {
        // Check if the mouse is still hovering after the delay
        if (this.mousePosition !== left) {
          return; // Mouse has moved out, stop further handling
        }

        if (this.isHoverIn === false) {
          return; // Mouse has moved out, stop further handling
        }

        const myPlayer = this.getYoutubePlayer(myElement);
        if (myPlayer) {
          myPlayer.playVideo();
          myPlayer.unMute();
        }

        const theirPlayer = this.getYoutubePlayer(theirElement);
        if (theirPlayer) {
          theirPlayer.getPlayerState().then((state) => {
            if (state === -1 || state === 3) {
              let interval = setInterval(() => {
                theirPlayer.getPlayerState().then((state) => {
                  if (state === -1 || state === 3) {
                    theirPlayer.mute();
                  } else {
                    clearInterval(interval);
                    if (this.mousePosition) {
                      this.videoHoverIn(this.le, this.re, true);
                    } else {
                      this.videoHoverIn(this.re, this.le, false);
                    }
                  }
                }).catch((error) => {
                  console.error('getPlayerState failed in interval:', error);
                  clearInterval(interval);
                });
              }, 100);
            } else {
              theirPlayer.pauseVideo();
              theirPlayer.mute();
            }
          }).catch((error) => {
            console.error('getPlayerState failed:', error);
          });
        }

        let myVideoPlayer = null;
        let theirVideoPlayer = null;
        if (left) {
          myVideoPlayer = this.getVideoPlayer("left-video-player");
          theirVideoPlayer = this.getVideoPlayer("right-video-player");
        } else {
          myVideoPlayer = this.getVideoPlayer("right-video-player");
          theirVideoPlayer = this.getVideoPlayer("left-video-player");
        }

        if (myVideoPlayer) {
          myVideoPlayer.play();
        }
        if (theirVideoPlayer) {
          theirVideoPlayer.pause();
        }

        if (left) {
          this.isLeftPlaying = true;
          this.isRightPlaying = false;
        } else {
          this.isLeftPlaying = false;
          this.isRightPlaying = true;
        }
      }, 300);
    },
    isImageSource: function (element) {
      return element.type === "image";
    },
    isVideoSource: function (element) {
      return element.type === "video";
    },
    isVideoUrlSource: function (element) {
      return element.type === "video" && element.video_source === "url";
    },
    isYoutubeSource: function (element) {
      return element.type === "video" && element.video_source === "youtube";
    },
    isTwitchVideoSource: function (element) {
      return element.type === "video" && element.video_source === "twitch_video";
    },
    isTwitchClipSource: function (element) {
      return element.type === "video" && element.video_source === "twitch_clip";
    },
    isBilibiliSource: function (element) {
      return element.type === "video" && element.video_source === "bilibili_video";
    },
    isYoutubeEmbedSource: function (element) {
      return element.type === "video" && element.video_source === "youtube_embed";
    },
    isGfycatSource: function (element) {
      return element.type === "video" && element.video_source === "gfycat";
    },
    onImageError: function (id, replaceUrl, event) {
      // avoid infinite loop
      if (this.errorImages.includes(id)) {
        return;
      }

      if (replaceUrl !== null) {
        event.target.src = replaceUrl;
      }
      this.errorImages.push(id);
    },
    loadGoogleAds() {
      try {
        if (window.adsbygoogle) {
          try {
            window.adsbygoogle.push({});
          } catch (e) { }
        }
      } catch (e) { }
    },
    startAdRefreshTimer() {
      if (this.adRefreshInterval !== null) return;

      // 第一個對戰畫面出現時只載入廣告，不做重新掛載；之後固定每 30 秒刷新。
      const loadInitialAds = () => {
        if (this.game) this.loadGoogleAds();
      };
      if (typeof this.$nextTick === 'function') {
        this.$nextTick(loadInitialAds);
      } else {
        loadInitialAds();
      }

      this.adRefreshInterval = setInterval(() => {
        if (!this.needReloadAD()) return;
        this.reloadGoogleAds();
        this.loadGoogleAds();
      }, AD_REFRESH_INTERVAL_MS);
    },
    stopAdRefreshTimer() {
      if (this.adRefreshInterval === null) return;
      clearInterval(this.adRefreshInterval);
      this.adRefreshInterval = null;
    },
    reloadGoogleAds() {
      $("#google-ad2-container").css("height", "340px").css("position", "relative");
      this.refreshAD = true;
      setTimeout(() => {
        this.refreshAD = false;
      }, 0);

      let retry = 5;
      let interval = setInterval(() => {
        if (retry <= 0) {
          clearInterval(interval);
          return;
        }
        retry--;
        if (window.adsbygoogle) {
          try {
            window.adsbygoogle.push({});
          } catch (e) {
            if (
              e.message.includes(
                `All 'ins' elements in the DOM with class=adsbygoogle already have ads in them`
              )
            ) {
              clearInterval(interval);
            }
          }
        }
        if ($("#google-ad")) {
          $("#google-ad").addClass("d-flex justify-content-center");
        }
      }, 500);
    },
    needReloadAD() {
      if (this.refreshAD) {
        return false;
      }

      if (!this.game) {
        return false;
      }

      return true;
    },
    closeBottomAd() {
      $("#ad2-reserver").remove();
      $("#ad2-container-desktop").css("height", "0").remove();
    },
    formatTime(time) {
      // format second to 0h0m0s
      let hour = Math.floor(time / 3600);
      let minute = Math.floor((time % 3600) / 60);
      let second = time % 60;
      return `${hour}h${minute}m${second}s`;
    },
    getThumbUrl(element) {
      if (element.thumb_url && element.thumb_url.endsWith('.gif')) {
        return element.thumb_url;
      }
      if (this.isMobileScreen) {
        return element.mediumthumb_url ? element.mediumthumb_url : element.lowthumb_url;
      } else {
        return element.thumb_url ? element.thumb_url : element.mediumthumb_url;
      }
    },
    getLowThumbUrl(element) {
      return element.lowthumb_url ? element.lowthumb_url : element.thumb_url;
    },
    enableTooltip() {
      Vue.nextTick(() => {
        $(function () {
          $('[data-toggle="tooltip"]').tooltip()
        })
      });
    },
    bootScreenSize() {
      this.isMobileScreen = $(window).width() < MD_WIDTH_SIZE;
    },
    resizeElementHeight() {
      // force update isMobileScreen
      this.bootScreenSize();
      if (this.isMobileScreen) {
        this.elementHeight = 200;
        this.gameBodyHeight = Math.max(this.elementHeight + 260, MOBILE_HEIGHT);
      } else {
        this.elementHeight = Math.max(window.innerHeight * 0.618 - 100, 413);
        this.gameBodyHeight = Math.max(this.elementHeight + 260, 650);
      }
    },
    registerResizeEvent() {
      window.addEventListener("resize", this.resizeElementHeight);
    },
    registerScrollEvent() {
      window.addEventListener("scroll", () => {
        if (!this.isMobileScreen) {
          return;
        }
        let ad2Top = $("#google-ad2").offset() ? $("#google-ad2").offset().top : 0;
        let offset = 100;

        // if scroll reach the bottom of the ad2
        if (window.scrollY + window.innerHeight >= ad2Top + offset) {
          this.showCreateRoomButton = true;
        } else {
          this.showCreateRoomButton = false;
        }
      });
    },

    // 時間軸拖曳相關方法
    startDrag(e) {
      const container = e.currentTarget;
      this.isDragging = true;
      this.startX = e.pageX - container.offsetLeft;
      this.scrollLeft = container.scrollLeft;
      container.style.cursor = 'grabbing';
      container.style.userSelect = 'none';
    },
    onDrag(e) {
      if (!this.isDragging) return;
      e.preventDefault();
      const container = e.currentTarget;
      const x = e.pageX - container.offsetLeft;
      const walk = (x - this.startX) * 2; // 拖曳速度係數
      container.scrollLeft = this.scrollLeft - walk;
    },
    stopDrag(e) {
      this.isDragging = false;
      const container = e.currentTarget;
      container.style.cursor = 'grab';
      container.style.userSelect = '';
    },
  },
};
</script>

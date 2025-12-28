<script>
import Swal from "sweetalert2";
import ICountUp from 'vue-countup-v2';
import QRCode from 'qrcode';


const MD_WIDTH_SIZE = 576;
const MOBILE_HEIGHT = 700;
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
  },
  components: {
    ICountUp
  },
  props: {
    postSerial: String,
    userLastGameSerial: String,
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

      // [New] Client Side Mode Data
      isClientMode: false,
      localElements: [], // 儲存所有參賽者物件 { ...element, local_win_count: 0, local_eliminated: false }
      localVotes: [], // 儲存投票紀錄 [{winner_id, loser_id}, ...]
      clientState: {
          currentRound: 1,
          matchesPlayedInRound: 0,
          ofRound: 0, // 這一輪總共要打幾場
          roundStartRemain: 0 // 這一輪開始時有多少人
      },

      // [New] Cloud Save 相關變數
      batchVoteInterval: 10, // 每 10 票存一次 (參數可設定)
      unsentVotes: [],       // 尚未同步到雲端的投票
      isCloudSaving: false,  // 是否正在儲存中
      existingElementIds: new Set(), // 已存在的元素 ID 集合 (用來避免重複加入)
    };
  },
  computed: {
    // [Modified] UI 顯示：目前場次 (改讀 game)
    displayCurrentRound() {
        return (this.game && this.game.current_round) ? this.game.current_round : 1;
    },

    // [Modified] UI 顯示：本輪總場次 (改讀 game)
    displayTotalRound() {
        return (this.game && this.game.of_round) ? this.game.of_round : 1;
    },

    // [New] UI 顯示：總參賽人數 (灰階旗標)
    displayTotalElements() {
        // 優先使用設定值，若無則回退到 game 物件
        return this.elementsCount || (this.game ? this.game.total_elements : 0);
    },

    // [Modified] UI 顯示：剩餘人數 (改讀 game)
    displayRemainElements() {
        // 不再自己算 localElements，直接用 game 裡的 snapshot
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
  },
  methods: {
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
    closeGameRoom() {
      clearInterval(this.autoRefreshRoomInterval);
      this.leaveGameRoom();
      this.isHostingGameRank = false;
      this.gameRoomSerial = null;
      this.gameRoom = null;
      this.gameBetRanks = [];
      this.gameRoomVotes = [];
      this.showGameRoomVotes = false;
      this.autoRefreshRoomInterval = null;
      $("#close-game-room").tooltip("dispose");
      this.$bus.$emit("closeGameRoom");
    },
    changeSortRanks() {
      this.sortByTop = !this.sortByTop;
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
    listenGameBet() {
      if (this.gameRoomSerial) {
        const channel = "game-room." + this.gameRoomSerial + ".game-serial." + this.gameSerial;
        Echo.channel(channel).listen(".GameBet", (data) => {
          this.gameRoomVotes = data;
        });
      }
    },
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
        this.isVoting = false
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

    // Room server
    handleCreatedRoom(data, roomUrl) {
      this.gameRoomSerial = data.serial
      this.gameRoom = data;
      this.gameBetRanks = this.gameRoom.ranks;
      this.gameRoomUrl = roomUrl;
      this.enableTooltip();
      Vue.nextTick(() => {
        QRCode.toCanvas(document.getElementById('qrcode'), roomUrl, {
          width: 200,
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

      this.nextRound(null);
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

    // [Modified] createGame
    createGame() {
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
          this.keepGameCookie();
          if (this.isHostingGameRank) {
              // --- 多人模式 (Server Mode) ---
              this.isClientMode = false;
              // 直接向後端請求第一回合資料
              this.nextRound(null);
          } else {
              // --- 單人模式 (Client Mode) ---
              this.initClientSideGame();
          }
        })
        .catch((error) => {
           // ... Error handling ...
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

    // [Modified] 初始化前端遊戲
    initClientSideGame() {
      this.isClientMode = true;
      this.localVotes = [];
      this.localElements = [];
      this.existingElementIds.clear(); // 清空 ID 紀錄

      const url = this.getGameElementsEndpoint.replace("_serial", this.gameSerial);
      const initialLimit = 32;
      const params = { params: { limit: initialLimit} };
      this.isDataLoading = true;

      axios.get(url, params)
        .then(res => {
            this.processNewElements(res.data.data);

            const total = this.elementsCount;

            this.clientState = {
                stage: 1,
                matchIndex: 0,
                stageStartCount: total,
                matchesInStage: 0,
                targetMatches: 0
            };

            this.updateStageConfig();
            this.saveToLocalStorage();

            // 3. 開始遊戲
            this.nextLocalRound();

            // 4. [New] 啟動背景抓取剩餘資料
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

    // [Modified] 過濾並處理新資料
    // [Modified] 過濾並處理新資料
    processNewElements(newElements) {
        if (!newElements || newElements.length === 0) return 0;

        let addedCount = 0;

        newElements.forEach(e => {
            // [Check] 檢查 ID 是否已存在
            if (!this.existingElementIds.has(e.id)) {

                // 加入 Set
                this.existingElementIds.add(e.id);

                // 格式化並加入 localElements
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

        return addedCount; // 回傳實際加入了幾筆 (用來判斷效率)
    },

    fetchRemainingElements(retryCount = 0) {
      // 1. 檢查目標達成：數量已足夠
      if (this.localElements.length >= this.elementsCount) {
          console.log("All elements loaded successfully.");
          return;
      }

      // 2. 安全閥
      if (retryCount > 10) { // 減少重試次數，因為我們現在是大批請求
          console.warn("Max retries reached. Stopping background fetch.");
          return;
      }

      const url = this.getGameElementsEndpoint.replace("_serial", this.gameSerial);

      const requestLimit = this.elementsCount;

      console.log(`Background fetching: requesting limit ${requestLimit}...`);

      axios.get(url, { params: { limit: requestLimit } })
          .then(res => {
              const data = res.data.data;

              if (data && data.length > 0) {
                  // 過濾並加入 (利用 Set 查重)
                  const actuallyAdded = this.processNewElements(data);

                  console.log(`Fetched ${data.length} items, actually added unique: ${actuallyAdded}`);

                  this.saveToLocalStorage();

                  // 解除 Loading 狀態 (如果剛好卡住)
                  if (this.waitingForData && this.localElements.length >= 2) {
                      this.waitingForData = false;
                      this.nextLocalRound();
                  }

                  // [Recursive] 檢查是否還沒抓滿
                  // 雖然我們請求了全部，但如果後端有分頁限制 (例如最多只給 100)，
                  // 我們可能還需要再抓一次。如果已經滿了，第一行的檢查會擋掉。
                  if (this.localElements.length < this.elementsCount) {
                      setTimeout(() => {
                          // 如果這次完全沒抓到新資料 (actuallyAdded === 0)，增加 retry
                          const nextRetry = actuallyAdded === 0 ? retryCount + 1 : 0;
                          this.fetchRemainingElements(nextRetry);
                      }, 30000); // 稍微休息一下再試
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

    // [Modified] 儲存狀態
    saveToLocalStorage() {
        if (!this.gameSerial || !this.isClientMode) return;
        // 如果是多人模式禁止存檔 (前面加的邏輯)
        if (this.gameRoomSerial) return;

        const key = `gamestate_${this.gameSerial}`;

        const stateToSave = {
            localElements: this.localElements,
            localVotes: this.localVotes,
            unsentVotes: this.unsentVotes,
            clientState: this.clientState,
            existingElementIds: Array.from(this.existingElementIds), // Set 轉 Array 才能存

            // [Fix] 必須把總人數設定也存起來！
            elementsCount: this.elementsCount,

            updatedAt: new Date().getTime()
        };

        localStorage.setItem(key, JSON.stringify(stateToSave));
    },

    // [Modified] 從 LocalStorage 讀取遊戲狀態
    loadFromLocalStorage() {
      if (this.isHostingGameRank) return false;
      if (!this.gameSerial) return false;

      const key = `gamestate_${this.gameSerial}`;
      const savedData = localStorage.getItem(key);

      if (savedData) {
          try {
              const parsed = JSON.parse(savedData);

              if (parsed.localElements && parsed.clientState) {
                  this.isClientMode = true;
                  this.localElements = parsed.localElements;
                  this.localVotes = parsed.localVotes || [];
                  this.clientState = parsed.clientState;
                  this.unsentVotes = parsed.unsentVotes || [];

                  // 還原 Set
                  if (parsed.existingElementIds) {
                      this.existingElementIds = new Set(parsed.existingElementIds);
                  } else {
                      this.existingElementIds = new Set(this.localElements.map(e => e.id));
                  }

                  // [Fix] 補丁：如果讀出來的存檔沒有 stageStartCount (例如舊存檔)
                  if (!this.clientState.stageStartCount) {
                    if (this.clientState.stage === 1) {
                        // 第一輪：起始人數 = 總人數
                        this.clientState.stageStartCount = this.localElements.length;
                    } else {
                        // 其他輪：暫時用活著的人數當作起始 (這是最接近的猜測)
                        this.clientState.stageStartCount = this.localElements.filter(e => !e.local_eliminated).length;
                    }
                  }

                  if (parsed.elementsCount) {
                    this.elementsCount = parsed.elementsCount;
                  }

                  console.log("Game restored from localStorage");
                  this.nextLocalRound();
                  return true;
              }
          } catch (e) {
              console.error("Failed to parse saved game state", e);
              return false;
          }
      }
      return false;
    },

    // [New] 清除 LocalStorage
    clearLocalStorage() {
        if (!this.gameSerial) return;
        const key = `gamestate_${this.gameSerial}`;
        localStorage.removeItem(key);
    },
    // [New] 移植後端的 NextRound 計算邏輯
    calculateNextRoundNumber(remain) {
        let powerOf2 = Math.pow(2, Math.floor(Math.log2(remain)));
        if (remain === powerOf2) {
            powerOf2 = powerOf2 / 2;
        }
        return remain - powerOf2;
    },

    // [Modified] updateStageConfig
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
            // 對應 PHP: if ($lastRound === null) { ... ceil($game->element_count / 2) }
            this.clientState.targetMatches = Math.ceil(baseCount / 2);
        } else {
            // 對應 PHP: else { calculateNextRoundNumber(...) }
            // 無論是 Stage 2, 3, 4... 通通套用這個公式
            this.clientState.targetMatches = this.calculateNextRoundNumber(baseCount);
        }
    },

    // [Modified] 繼續遊戲 (增加 LocalStorage 檢查)
    continueGame() {

      // 1. 多人模式檢查
      if (this.isHostingGameRank) {
          // 房間模式下，遊戲序號通常由後端房間資訊提供，或已經在 mounted 時設定好
          // 這裡假設 this.gameSerial 已經透過 getRoomEndpoint 取得了
          if (this.gameSerial) {
              this.isClientMode = false;
              this.nextRound(null);
          } else {
              // 如果還沒拿到 gameSerial，可能需要先 refresh room info
              console.warn("Room mode: Game serial not found yet.");
          }
          $("#gameSettingPanel").modal("hide");
          return;
      }

      let gameSerial = '';
      if (this.userLastGameSerial) {
        gameSerial = this.userLastGameSerial;
      } else {
        gameSerial = this.$cookies.get(this.postSerial)
      }

      if (gameSerial) {
        this.gameSerial = gameSerial;

        // [New] 優先嘗試從 LocalStorage 讀取進度
        if (this.loadFromLocalStorage()) {
             // 讀取成功，已經在 loadFromLocalStorage 裡呼叫了 nextLocalRound
             // 這裡不需要做任何事
        } else {
             // 讀取失敗（可能是 Server Mode 舊資料），走後端流程
             this.nextRound(null, false);
        }
      }
      $("#gameSettingPanel").modal("hide");
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

    // [Modified] nextRound 修正 Null Error
    nextRound(data, reset = true) {
      // 1. 如果是 Client Mode，且資料不完整 (null 或 data.data 為 null)，直接呼叫本地邏輯
      if (!this.isHostingGameRank && this.isClientMode && (data == null || (data && !data.data))) {
          console.log("Client Mode: Proceeding to next local round.");
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

      this.handleAnimationAfterNextRound(data.data, reset);
    },

    nextLocalRound() {
        let activeElements = this.localElements.filter(e => !e.local_eliminated);

        if (activeElements.length < 2) {
             this.sendBatchVotes();
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
            this.nextLocalRound();
            return;
        }

        let el1, el2;
        let readyElements = activeElements.filter(e => e.local_is_ready);

        if (this.clientState.stage === 2) {
            readyElements.sort((a, b) => {
                if (a.local_played !== b.local_played) return a.local_played - b.local_played;
                return 0.5 - Math.random();
            });
            el1 = readyElements[0];
            el2 = readyElements[1];
        } else {
            readyElements.sort(() => 0.5 - Math.random());
            if (readyElements.length >= 2) {
                el1 = readyElements[0];
                el2 = readyElements[1];
            } else if (readyElements.length === 1) {
                el1 = readyElements[0];
                const notReadyElements = activeElements.filter(e => !e.local_is_ready);
                if (notReadyElements.length > 0) {
                    const randomIndex = Math.floor(Math.random() * notReadyElements.length);
                    el2 = notReadyElements[randomIndex];
                } else {
                    this.sendBatchVotes();
                    return;
                }
            }
        }

        const eliminatedCount = this.localElements.filter(e => e.local_eliminated).length;
        const realRemainCount = this.elementsCount - eliminatedCount;
        this.clientState.matchIndex++;
        const currentMatchInStage = this.clientState.matchesInStage + 1;
        const mockGameData = {
            current_round: currentMatchInStage,
            of_round: this.clientState.targetMatches,
            remain_elements: realRemainCount,
            total_elements: this.elementsCount,
            stage_start_count: this.clientState.stageStartCount,
            elements: [el1, el2]
        };

        this.handleAnimationAfterNextRound(mockGameData, true);
    },

    handleAnimationAfterNextRound(game, reset) {
      return new Promise((resolve, reject) => {
        this.updateGame(game);
        resolve();
      })
        .then(() => {
          if (reset) {
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

          if (this.needReloadAD()) {
            this.reloadGoogleAds();
          }
          this.loadGoogleAds();
        });
    },
    // [Modified] 更新遊戲數據 (核心同步樞紐)
    updateGame(game) {
      // Server Mode
      if (!this.isClientMode) {
          // 計算 current_match_index (0-based)
          const matchIdx = (game.current_round || 1) - 1;

          game.stage_start_count = game.remain_elements + matchIdx;
      }

      // 更新主要遊戲物件 (這一刻，UI 才會跟著變動)
      this.game = game;

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
      this.rememberedScrollPosition = document.documentElement.scrollTop;
      this.isVoting = true;
      let sendWinnerData = () => {
        if (this.isBetGameClient) {
          this.bet(this.le, this.re);
        } else {
          this.vote(this.le, this.re);
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
      this.rememberedScrollPosition = document.documentElement.scrollTop;
      this.isVoting = true;
      let sendWinnerData = () => {
        if (this.isBetGameClient) {
          this.bet(this.re, this.le);
        } else {
          this.vote(this.re, this.le);
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
        }
        else{
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
      }).finally(() => {
        if (this.needReloadAD()) {
          this.reloadGoogleAds();
        }
        this.loadGoogleAds();
      });
    },

    // 請確保 vote 方法有正確分流：
    vote(winner, loser) {
      if (!this.isClientMode || this.isHostingGameRank) {
        const data = { game_serial: this.gameSerial, winner_id: winner.id, loser_id: loser.id };
        this.sendVote(data);
      } else {
        this.handleClientVote(winner, loser);
      }
    },

    // [Modified] 處理投票 (加入存檔邏輯)
    handleClientVote(winner, loser) {
      const winnerObj = this.localElements.find(e => e.id === winner.id);
      const loserObj = this.localElements.find(e => e.id === loser.id);

      if (winnerObj && loserObj) {
          winnerObj.local_win_count++;
          winnerObj.local_played++;
          loserObj.local_played++;
          loserObj.local_eliminated = true;
          winnerObj.local_is_ready = false;
          loserObj.local_is_ready = false;
          this.clientState.matchesInStage++;

          this.localVotes.push({ winner_id: winner.id, loser_id: loser.id });
          this.unsentVotes.push({ winner_id: winner.id, loser_id: loser.id });

          // [New] 狀態改變後立即存檔
          this.saveToLocalStorage();

          // [New] 檢查是否需要觸發雲端備份 (滿 10 票 且 目前沒在存)
          if (this.unsentVotes.length >= this.batchVoteInterval && !this.isCloudSaving) {
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
    },

    sendPartialBatchVotes() {
      if (this.unsentVotes.length === 0) return;

      this.isCloudSaving = true;

      // 複製一份要傳送的資料，避免在傳送過程中又有新投票進來導致競態條件
      const votesToSend = [...this.unsentVotes];

      // 樂觀更新：先清空陣列，如果失敗再補回去 (或者鎖住，但在遊戲中不建議鎖住 UI)
      // 這裡採取：傳送當下的 snapshot

      const data = {
          game_serial: this.gameSerial,
          votes: votesToSend
      };

      // 使用原本的 batchVoteEndpoint，後端邏輯通用
      axios.post(this.batchVoteEndpoint, data)
          .then(res => {
              // 成功：從 unsentVotes 中移除已經發送的那些
              // 注意：不能直接 this.unsentVotes = []，因為傳送期間使用者可能又投了幾票
              // 我們只移除 `votesToSend` 裡面的數量
              this.unsentVotes.splice(0, votesToSend.length);

              // 更新 LocalStorage (確保 unsentVotes 變空這件事被記住)
              this.saveToLocalStorage();

              // 為了讓 UI Icon 顯示久一點，延遲關閉 (可選)
              setTimeout(() => {
                  this.isCloudSaving = false;
              }, 1000);
          })
          .catch(err => {
              console.error("Cloud save failed", err);
              // 失敗：不做任何事，資料保留在 unsentVotes，等待下一次觸發或結束時發送
              this.isCloudSaving = false;
          });
    },

    // [Modified] 批次送出投票 (成功後清除存檔)
    sendBatchVotes() {
        this.finishingGame = true;
        this.isDataLoading = true;

        if (this.unsentVotes.length === 0) {
            this.clearLocalStorage();
            // 模擬一個後端回應讓 handleSendVote 處理跳轉
            this.handleSendVote({ data: { status: 'end_game' } });
            return;
        }

        const data = {
            game_serial: this.gameSerial,
            votes: this.unsentVotes // 送出最後一批
        };

        axios.post(this.batchVoteEndpoint, data)
            .then(res => {
                // [New] 遊戲正常結束，清除存檔
                this.clearLocalStorage();

                this.handleSendVote(res);
            })
            .catch(err => {
                console.error("Batch vote failed", err);
                Swal.fire({
                    icon: 'error',
                    text: 'Error submitting game results.'
                });
                this.isDataLoading = false;
                this.finishingGame = false;
            });
    },

    sendVote(data) {
      axios
        .post(this.voteGameEndpoint, data)
        .then((res) => {
          console.log("Vote response:", res);
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
              // this.scrollToLastPosition();
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
          // [Modified] 判斷最後一局的邏輯調整
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

              // [Modified] Client Mode 分流
              if (this.isClientMode && isFinalRound) {
                   this.sendBatchVotes();
              } else {
                   this.handleSendVote(res);
              }
            });
          } else {
            this.isDataLoading = true;
            // [Modified] Client Mode 分流
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
      if(this.autoRefreshRoomInterval){
        this.autoRefreshRoomCounter = 0;
      }
      this.status = res.data.status;
      if (this.status === "end_game") {
        this.$cookies.remove(this.postSerial);
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
        element.twitchPlayer = new Twitch.Embed("twitch-video-" + element.id, {
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
                });
              }, 100);
            } else {
              theirPlayer.pauseVideo();
              theirPlayer.mute();
            }
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
        let offset = 50;

        // if scroll reach the bottom of the ad2
        if (window.scrollY + window.innerHeight >= ad2Top + offset) {
          this.showCreateRoomButton = true;
        } else {
          this.showCreateRoomButton = false;
        }
      });
    },
  },
};
</script>

<template>

  <div class="container-fluid">
    <div class="text-center" v-if="game">
      <h2>{{game.title}}</h2>
      <h2>{{game.current_round}} / {{game.of_round}}</h2>
    </div>
    <div class="row" v-if="game">
      <div class="col-md-6 pr-md-0">
        <div class="card">
          <div v-if="le.type === 'image'"
               @click="clickImage"
               :style="{backgroundImage: 'url('+le.source_url+')' }"
               class="game-image"
          ></div>
          <div class="d-flex" v-if="le.type === 'video'"
               @mouseover="videoHoverIn(le, re)"
               @mouseleave="videoHoverOut(le, re)"
          >
            <youtube :videoId="le.video_id"
                     width="100%" height="600"
                     :ref="le.id"
                     @ready="doPlay(le)"
                     :player-vars="{
                      controls:1,
                      autoplay:1,
                      rel: 0,
                      host: 'https://www.youtube.com'
                     }"
            ></youtube>
          </div>
          <div class="card-body text-center">
            <div style="height: 50px">
              <h5 class="card-title">{{le.title}}</h5>
            </div>
            <label class="btn btn-primary btn-lg btn-block"
                   @click="vote(le, re)">Vote</label>
          </div>
        </div>
      </div>
      <div class="col-md-6 pl-md-0">
        <div class="card">
          <div v-if="re.type === 'image'"
               @click="clickImage"
               :style="{backgroundImage: 'url('+re.source_url+')' }"
               class="game-image"
          ></div>
          <div class="d-flex" v-if="re.type === 'video'"
               @mouseover="videoHoverIn(re, le)"
               @mouseleave="videoHoverOut(re, le)"
          >
            <youtube :videoId="re.video_id"
                     width="100%" height="600"
                     :ref="re.id"
                     @ready="doPlay(re)"
                     :player-vars="{
                      controls:1,
                      autoplay:1,
                      rel: 0,
                      host: 'https://www.youtube.com'
                     }"
            ></youtube>
          </div>
          <div class="card-body text-center">
            <div style="height: 50px">
              <h5 class="card-title">{{re.title}}</h5>
            </div>
            <label class="btn btn-danger btn-lg btn-block"
                   @click="vote(re, le)">Vote</label>
          </div>
        </div>
      </div>
    </div>

    <!-- Modal -->
    <div class="modal fade" id="gameSettingPanel" data-backdrop="static" data-keyboard="false" tabindex="-1"
         aria-labelledby="gameSettingPanelLabel" aria-hidden="true">
      <div class="modal-dialog">
        <div class="modal-content">
          <div class="modal-header">
            <h5 class="modal-title" id="gameSettingPanelLabel">比賽設定</h5>
          </div>
          <ValidationObserver v-slot="{ invalid }" v-if="setting">
            <form @submit.prevent>
              <div class="modal-body">
                <div class="alert alert-danger" v-if="processingGameSerial">
                  有未完成的比賽，是否繼續？
                  <span class="btn btn-outline-danger" @click="continueGame">
                    <i class="fas fa-play"></i>
                    繼續比賽
                  </span>
                </div>
                <h2>{{setting.title}}</h2>
                <div class="row">
                  <div class="col-12">
                    <ValidationProvider rules="required" v-slot="{ errors }">
                      <div class="input-group mb-3">
                        <div class="input-group-prepend">
                          <label class="input-group-text" for="elementsCount">參戰數</label>
                        </div>
                        <select v-model="elementsCount" class="custom-select" id="elementsCount" required>
                          <option value="" disabled selected="selected">請選擇</option>
                          <option v-for="count in [4,8,16,32,64,128,256,512,1024]"
                                  :value="count" v-if="setting.elements_count >= count">
                            {{count}}
                          </option>
                          <option :value="setting.elements_count" v-if="!isElementsPowerOfTwo">
                            {{setting.elements_count}}
                          </option>
                        </select>
                      </div>
                    </ValidationProvider>
                  </div>
                </div>
              </div>
              <div class="modal-footer">
                <button type="submit" class="btn btn-primary" :disabled="invalid" @click="createGame">開戰!</button>
              </div>
            </form>
          </ValidationObserver>
        </div>
      </div>
    </div>
  </div>
</template>

<script>
  export default {
    mounted() {
      this.loadGameSetting();
      this.showGameSettingPanel();
    },
    props: {
      postSerial: String,
      getRankRoute: String,
      getGameSettingEndpoint: String,
      nextRoundEndpoint: String,
      createGameEndpoint: String,
      voteGameEndpoint: String
    },
    data: function () {
      return {
        gameSerial: null,
        game: null,
        status: null,
        setting: null,
        elementsCount: ""
      }
    },
    computed: {
      isElementsPowerOfTwo: function () {
        if (!this.setting || !this.setting.elements_count) {
          return false;
        }

        return Number.isInteger(Math.log2(this.setting.elements_count));
      },
      processingGameSerial: function () {
        return this.$cookies.get(this.postSerial);
      },
      le: function () {
        if (this.game) {
          return this.game.elements[0]
        }
        return null;
      },
      re: function () {
        if (this.game) {
          return this.game.elements[1]
        }
        return null;
      }
    },
    methods: {
      loadGameSetting: function () {
        axios.get(this.getGameSettingEndpoint)
          .then(res => {
            this.setting = res.data.data;
          });
      },
      createGame: function () {
        const data = {
          'post_serial': this.postSerial,
          'element_count': this.elementsCount
        };
        axios.post(this.createGameEndpoint, data)
          .then(res => {
            this.gameSerial = res.data.game_serial;
            this.nextRound();
          });
        $('#gameSettingPanel').modal('hide');
      },
      continueGame: function () {
        const gameSerial = this.$cookies.get(this.postSerial);
        if (gameSerial) {
          this.gameSerial = gameSerial;
          this.nextRound();
        }
        $('#gameSettingPanel').modal('hide');
      },
      nextRound: function () {
        const url = this.nextRoundEndpoint.replace('_serial', this.gameSerial);
        axios.get(url)
          .then(res => {
            this.game = res.data.data;
            this.doPlay(this.le);
            this.doPlay(this.re);
          });
      },
      vote: function (winner, loser) {
        const data = {
          'game_serial': this.gameSerial,
          'winner_id': winner.id,
          'loser_id': loser.id
        };
        axios.post(this.voteGameEndpoint, data)
          .then(res => {
            this.status = res.data.status;
            if (this.status === 'end_game') {
              this.$cookies.remove(this.postSerial);
              this.showGameResult();
            } else {
              this.$cookies.set(this.postSerial, this.gameSerial, "1d");
              this.nextRound();
            }
          })
      },
      showGameSettingPanel: function () {
        $('#gameSettingPanel').modal('show');
      },
      showGameResult: function () {
        const url = this.getRankRoute.replace('_serial', this.postSerial) + '?g='+this.gameSerial;
        window.open(url, '_self');
      },
      getPlayer(element) {
        return _.get(this.$refs, element.id + '.player', null);
      },
      doPlay(element) {
        const player = this.getPlayer(element);
        if (player) {
          player.mute();
          // reset when video is stop
          player.addEventListener('onStateChange', (event) => {
            if (event.target.getPlayerState() === 0) {
              player.stopVideo();
              player.cueVideoById({
                videoId: element.video_id,
                startSeconds: element.video_start_second
              });
            }
          });
          player.loadVideoById({
            videoId: element.video_id,
            startSeconds: element.video_start_second,
            endSeconds: element.video_end_second
          });
        }
      },
      destroyElements() {
        let player = null;
        player = this.getPlayer(this.le);
        if (player) {
          player.stopVideo();
        }

        player = this.getPlayer(this.re);
        if (player) {
          player.stopVideo();
        }
      },
      videoHoverIn(myElement, theirElement) {
        const myPlayer = this.getPlayer(myElement);
        if (myPlayer) {
          window.p1 = myPlayer;
          myPlayer.unMute();
        }

        const theirPlayer = this.getPlayer(theirElement);
        if (theirPlayer) {
          window.p2 = theirPlayer;
          theirPlayer.mute();
        }

      },
      videoHoverOut(myElement, theirElement) {
        // nothing
      },
      clickImage(event) {
        const obj = $(event.target);
        const size = obj.css('background-size');
        if (size === 'contain') {
          obj.css('background-size', 'cover');
        } else if (size === 'cover') {
          obj.css('background-size', 'contain');
        } else {
          obj.css('background-size', 'contain');
        }
      },
    }
  }

</script>

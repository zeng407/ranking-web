<template>
  <div class="container-fluid">
    <div v-if="game">
      <h2 class="text-center text-break">{{ game.title }}</h2>
      <div class="d-flex" style="flex-flow: row wrap">
        <h5 style="width: 20%"></h5>
        <h5 class="text-center align-self-center" style="width: 60%">ROUND {{ game.current_round }} / TOP {{
          game.of_round
        }} </h5>
        <h5 class="text-right align-self-center" style="width: 20%">({{ game.remain_elements }} / {{
          game.total_elements
        }})</h5>
      </div>
    </div>
    <div class="row game-body" v-if="game">
      <!--left part-->
      <div class="col-md-6 pr-md-1 mb-2 mb-md-0">
        <div class="card game-player left-player" id="left-player">
          <div v-if="isImageSource(le)" @click="clickImage"
            :style="{ backgroundImage: 'url(' + le.thumb_url + ')', height: this.elementHeight + 'px' }"
            class="game-image"></div>
          <div class="d-flex" v-if="isYoutubeSource(le)" @mouseover="videoHoverIn(le, re)"
            @mouseleave="videoHoverOut(le, re)">
            <youtube :videoId="le.video_id" width="100%" :height="elementHeight" :ref="le.id"
              @ready="doPlay(le, true, 'left')" :player-vars="{
                controls: 1,
                autoplay: 1,
                rel: 0,
                origin: host,
                loop: 1,
                playlist: le.video_id
              }"></youtube>
          </div>
          <div v-else-if="isVideoSource(le)">
            <video width="100%" :height="elementHeight" loop autoplay muted playsinline :src="le.thumb_url"></video>
          </div>
          <div class="card-body text-center">
            <div class="my-1" style="height: 70px">
              <p class="my-1">{{ le.title }}</p>
            </div>
            <button class="btn btn-primary btn-lg btn-block d-none d-md-block" :disabled="isVoting"
              @click="leftWin()">Vote
            </button>
            <div class="row" v-if="isYoutubeSource(le)">
              <div class="col-3">
                <button class="btn btn-outline-primary btn-lg btn-block d-block d-md-none" :disabled="isVoting"
                  @click="leftPlay()">
                  <i class="fas fa-volume-mute" v-show="!isLeftPlaying"></i>
                  <i class="fas fa-volume-up" v-show="isLeftPlaying"></i>
                </button>
              </div>
              <div class="col-9">
                <button class="btn btn-primary btn-lg btn-block d-block d-md-none" :disabled="isVoting"
                  @click="leftWin()">
                  Vote
                </button>
              </div>
            </div>
            <div v-else>
              <button class="btn btn-primary btn-block btn-lg d-block d-md-none" :disabled="isVoting"
                @click="leftWin()">Vote
              </button>
            </div>

          </div>
        </div>
      </div>

      <!--right part-->
      <div class="col-md-6 pl-md-1 mb-4 mb-md-0">
        <div class="card game-player right-player" :class="{ 'flex-column-reverse': isMobileScreen }" id="right-player">
          <div v-if="isImageSource(re)" @click="clickImage"
            :style="{ backgroundImage: 'url(' + re.thumb_url + ')', height: this.elementHeight + 'px' }"
            class="game-image"></div>
          <div class="d-flex" v-else-if="isYoutubeSource(re)" @mouseover="videoHoverIn(re, le)"
            @mouseleave="videoHoverOut(re, le)">
            <youtube :videoId="re.video_id" width="100%" :height="elementHeight" :ref="re.id"
              @ready="doPlay(re, false, 'right')" :player-vars="{
                controls: 1,
                autoplay: 1,
                rel: 0,
                host: host,
                loop: 1,
                playlist: re.video_id
              }"></youtube>
          </div>
          <div v-else-if="isVideoSource(re)">
            <video width="100%" :height="elementHeight" loop autoplay muted playsinline :src="re.thumb_url"></video>
          </div>

          <!-- reverse when device size width less md(768px)-->
          <div class="card-body text-center" :class="{ 'flex-column-reverse': isMobileScreen, 'd-flex': isMobileScreen }">
            <div class="my-1" style="height: 70px"
              :class="{ 'flex-column-reverse': isMobileScreen, 'd-flex': isMobileScreen }">
              <p class="my-1">{{ re.title }}</p>
            </div>
            <button class="btn btn-danger btn-lg btn-block d-none d-md-block" :disabled="isVoting"
              @click="rightWin()">Vote
            </button>
            <div class="row" v-if="isYoutubeSource(re)">
              <div class="col-3">
                <button class="btn btn-outline-danger btn-lg btn-block d-block d-md-none" :disabled="isVoting"
                  @click="rightPlay()">
                  <i class="fas fa-volume-mute" v-show="!isRightPlaying"></i>
                  <i class="fas fa-volume-up" v-show="isRightPlaying"></i>
                </button>
              </div>
              <div class="col-9">
                <button class="btn btn-danger btn-lg btn-block d-block d-md-none" :disabled="isVoting"
                  @click="rightWin()">
                  Vote
                </button>
              </div>
            </div>
            <div v-else>
              <button class="btn btn-danger btn-lg btn-block d-block d-md-none" :disabled="isVoting"
                @click="rightWin()">Vote
              </button>
            </div>
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
            <h5 class="modal-title" id="gameSettingPanelLabel">{{ $t('game.setting') }}</h5>
          </div>
          <ValidationObserver v-slot="{ invalid }" v-if="post">
            <form @submit.prevent>
              <div class="modal-body">
                <div class="alert alert-danger" v-if="processingGameSerial">
                  <i class="fas fa-exclamation-triangle"></i>{{ $t('game.continue_hint') }}
                  <span class="btn btn-outline-danger" @click="continueGame">
                    <i class="fas fa-play"></i>
                    {{ $t('game.continue') }}
                  </span>
                </div>
                <div class="card">
                  <div class="card-header text-center">
                    <h3>{{ post.title }}</h3>
                  </div>
                  <div class="row no-gutters">
                    <div class="col-6">
                      <div :style="{
                        'background': 'url(' + post.image1.url + ')',
                        'width': '100%',
                        'height': '300px',
                        'background-repeat': 'no-repeat',
                        'background-size': 'cover',
                        'background-position': 'center center',
                        'display': 'flex'
                      }"></div>
                      <h5 class="text-center mt-1 p-1">{{ post.image1.title }}</h5>
                    </div>
                    <div class="col-6">
                      <div :style="{
                        'background': 'url(' + post.image2.url + ')',
                        'width': '100%',
                        'height': '300px',
                        'background-repeat': 'no-repeat',
                        'background-size': 'cover',
                        'background-position': 'center center',
                        'display': 'flex'
                      }">
                      </div>
                      <h5 class="text-center mt-1 p-1">{{ post.image2.title }}</h5>
                    </div>
                    <div class="card-body pt-0 text-center">
                      <h5 class="text-break">{{ post.description }}</h5>
                      <div v-if="post.tags.length > 0" class="d-flex">
                        <span class="badge badge-secondary m-1" v-for="tag in post.tags" style="font-size:medium">#{{ tag }}</span>
                      </div>
                      <span class="mt-2 card-text d-flex justify-content-end">
                        <span class="pr-2">
                          <i class="fas fa-play-circle"></i>&nbsp;{{post.play_count}}
                        </span>
                        <small class="text-muted">{{ post.created_at | datetime }}</small>
                      </span>
                    </div>
                  </div>
                </div>
                <div class="row mt-2">
                  <div class="col-12">
                    <ValidationProvider rules="required" v-slot="{ errors }">
                      <div class="input-group mb-3">
                        <div class="input-group-prepend">
                          <label class="input-group-text" for="elementsCount">{{ $t('game.rounds') }}</label>
                        </div>
                        <select v-model="elementsCount" class="custom-select" id="elementsCount" required>
                          <option value="" disabled selected="selected">{{ $t('game.select') }}</option>
                          <option v-for="count in [8, 16, 32, 64, 128, 256, 512, 1024]" :value="count"
                            v-if="post.elements_count >= count">
                            {{ count }}
                          </option>
                          <option :value="post.elements_count" v-if="!isElementsPowerOfTwo">
                            {{ post.elements_count }}
                          </option>
                        </select>
                      </div>
                    </ValidationProvider>
                  </div>
                </div>
              </div>
              <div class="modal-footer">
                <a type="submit" class="btn btn-secondary fa-pull-left" href="/">{{ $t('game.cancel') }}</a>
                <button type="submit" class="btn btn-primary" :disabled="invalid" @click="createGame">{{ $t('game.start') }}</button>
              </div>
            </form>
          </ValidationObserver>
        </div>
      </div>
    </div>
  </div>
</template>

<script>
const MD_WIDTH_SIZE = 768;
export default {
  mounted() {
    this.loadGameSetting();
    this.showGameSettingPanel();
    this.host = window.location.origin;
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
      clientWidth: null,
      host: '',
      elementHeight: 450,
      gameSerial: null,
      game: null,
      status: null,
      post: null,
      elementsCount: "",
      isVoting: false,
      isLeftPlaying: true,
      isRightPlaying: false,
      rememberedScrollPosition: null,
      isLeftPlayerInit: false,
      isRightPlayerInit: false,
    }
  },
  computed: {
    isMobileScreen: function () {
      return $(window).width() < MD_WIDTH_SIZE;
    },
    isElementsPowerOfTwo: function () {
      if (!this.post || !this.post.elements_count) {
        return false;
      }

      return Number.isInteger(Math.log2(this.post.elements_count));
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
          this.post = res.data.data;
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
          this.doPlay(this.le, true, 'left');
          this.doPlay(this.re, false, 'right');
        })
        .then(() => {
          this.resetPlayerPosition();
          this.scrollToLastPosition();
        });
    },
    leftPlay() {
      this.isLeftPlaying = true;
      this.isRightPlaying = false;
      const myPlayer = this.getPlayer(this.le);
      if (myPlayer) {
        // window.p1 = myPlayer;
        myPlayer.unMute();
      }

      const theirPlayer = this.getPlayer(this.re);
      if (theirPlayer) {
        // window.p2 = theirPlayer;
        theirPlayer.mute();
      }
    },
    leftWin() {
      this.rememberedScrollPosition = document.documentElement.scrollTop;
      this.isVoting = true;
      let sendWinnerData = () => {
        this.vote(this.le, this.re);
      }

      if (this.isMobileScreen) {
        let winAnimate = $('#left-player').toggleClass('zoom-in').promise();
        let loseAnimate = $('#right-player').animate({ opacity: '0' }, 500, () => {
          $('#right-player').hide();
        }).promise();
        $.when(winAnimate, loseAnimate).then(() => {
          sendWinnerData();
        });
        return;
      }

      let winAnimate = $('#left-player').animate({ left: '50%' }, 500, () => {
        $('#left-player').delay(500).animate({ top: '-2000' }, 500, () => {
          $('#left-player').hide();
        });
      }).promise();
      let loseAnimate = $('#right-player').animate({ top: '2000' }, 500, () => {
        $('#right-player').hide();
      }).promise();

      $.when(winAnimate, loseAnimate).then(() => {
        sendWinnerData();
      });
    },
    rightPlay() {

      this.isLeftPlaying = false;
      this.isRightPlaying = true;
      const myPlayer = this.getPlayer(this.re);
      if (myPlayer) {
        myPlayer.unMute();
      }

      const theirPlayer = this.getPlayer(this.le);
      if (theirPlayer) {
        theirPlayer.mute();
      }

    },
    rightWin() {
      this.rememberedScrollPosition = document.documentElement.scrollTop;
      this.isVoting = true;
      let sendWinnerData = () => {
        this.vote(this.re, this.le);
      }

      if (this.isMobileScreen) {
        let winAnimate = $('#right-player').toggleClass('zoom-in').promise();
        let loseAnimate = $('#left-player').animate({ opacity: '0' }, 500, () => {
          $('#left-player').hide();
        }).promise();
        $.when(winAnimate, loseAnimate).then(() => {
          sendWinnerData();
        });
        return;
      }

      let winAnimate = $('#right-player').animate({ left: '-50%' }, 500, () => {
        $('#right-player').delay(500).animate({ top: '-2000' }, 500, () => {
          $('#right-player').hide();
        });
      }).promise();

      let loseAnimate = $('#left-player').animate({ top: '2000' }, 500, () => {
        $('#left-player').hide();
      }).promise();

      $.when(winAnimate, loseAnimate).then(() => {
        sendWinnerData();
      });
    },
    resetPlayerPosition() {
      $('#left-player').hide();
      $('#left-player').css('left', '0');
      $('#left-player').css('top', '0');
      $('#left-player').css('opacity', '1');
      $('#left-player').css('scale', '1');
      $('#left-player').removeClass('zoom-in');
      $('#left-player').show();

      $('#right-player').hide();
      $('#right-player').css('left', '0');
      $('#right-player').css('top', '0');
      $('#right-player').css('opacity', '1');
      $('#right-player').css('scale', '1');
      $('#right-player').removeClass('zoom-in');
      $('#right-player').show();
    },
    scrollToLastPosition() {
      if (this.rememberedScrollPosition !== null) {
        // console.log(this.rememberedScrollPosition);
        window.scrollTo(0, this.rememberedScrollPosition);
      }
    },
    vote: function (winner, loser) {
      const data = {
        'game_serial': this.gameSerial,
        'winner_id': winner.id,
        'loser_id': loser.id
      };

      // let loseAnimation = $('#' + loseObj).animate({top: '2000px'}, 500, () => {
      //   $('#' + loseObj).hide();
      //   $('#' + loseObj).css('top', '0');
      // }).promise();
      // let winAnimation = $('#' + winObj).animate({left: '50%'}, 500, () => {
      //   $('#' + winObj).hide();
      //   $('#' + winObj).css('left', '0');
      // }).promise();
      // let loseAnimation = $('#' + winObj).trigger('win');
      // let winAnimation = $('#' + loseObj).trigger('lose');
      return axios.post(this.voteGameEndpoint, data)
        .then(res => {
          this.isVoting = false;
          this.status = res.data.status;
          if (this.status === 'end_game') {
            this.$cookies.remove(this.postSerial);
            this.showGameResult();
          } else {
            this.$cookies.set(this.postSerial, this.gameSerial, "1d");
            this.nextRound();
            this.destroyElements();
          }
        })

    },
    showGameSettingPanel: function () {
      $('#gameSettingPanel').modal('show');
    },
    showGameResult: function () {
      const url = this.getRankRoute.replace('_serial', this.postSerial) + '?g=' + this.gameSerial;
      window.open(url, '_self');
    },
    getPlayer(element) {
      return _.get(this.$refs, element.id + '.player', null);
    },
    doPlay(element, loud = false, name) {
      const player = this.getPlayer(element);

      if (player) {
        if (loud) {
          player.unMute();
        } else {
          player.mute();
        }

        if (name === 'left' && this.isLeftPlayerInit === false) {
          this.isLeftPlayerInit = true;
          this.initPlayerEventLister(player);
        } else if (name === 'right' && this.isRightPlaying === false) {
          this.isLeftPlayerInit = true
          this.initPlayerEventLister(player);
        }

        player.loadVideoById({
          videoId: element.video_id,
          startSeconds: element.video_start_second,
          endSeconds: element.video_end_second
        });
      }
    },
    initPlayerEventLister(player) {
      player.addEventListener('onStateChange', (event) => {
        if (event.target.getPlayerState() === 0) {
          player.playVideo();
        }
      });
    },
    destroyElements() {
      let player = null;
      player = this.getPlayer(this.le);
      //default left player un-mute
      if (player) {
        player.unMute();
        player.stopVideo();
        this.isLeftPlaying = true;
      }

      player = this.getPlayer(this.re);
      if (player) {
        player.mute();
        player.stopVideo();
        this.isRightPlaying = false;
      }
    },
    videoHoverIn(myElement, theirElement) {
      if (this.isMobileScreen) {
        return;
      }
      const myPlayer = this.getPlayer(myElement);
      if (myPlayer) {
        // window.p1 = myPlayer;
        myPlayer.unMute();
      }

      const theirPlayer = this.getPlayer(theirElement);
      if (theirPlayer) {
        // window.p2 = theirPlayer;
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
    isImageSource: function (element) {
      return element.type === 'image';
    },
    isVideoSource: function (element) {
      return element.type === 'video';
    },
    isYoutubeSource: function (element) {
      return element.type === 'video' && element.video_source === 'youtube';
    },
    isGfycatSource: function (element) {
      return element.type === 'video' && element.video_source === 'gfycat';
    },
    // isMobileScreen() {
    //   return this.clientWidth < MD_WIDTH_SIZE;
    // },
  },

  beforeMount() {
    // less md size
    if ($(window).width() < MD_WIDTH_SIZE) {
      this.elementHeight = 200
    }
  }
}

</script>

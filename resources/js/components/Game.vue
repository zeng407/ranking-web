<template>
  <div class="container-fluid">
    <div v-if="game">
      <h2 class="text-center text-break">{{ game.title }}</h2>
      <div class="d-none d-sm-flex" style="flex-flow: row wrap">
        <h5 style="width: 20%"></h5>
        
        <h5 class="text-center align-self-center" style="width: 60%">
          <span v-if="currentRemainElement <= 2">{{ $t('game_round_final') }}</span>
          <span v-else-if="currentRemainElement <= 4">{{ $t('game_round_semifinal') }}</span>
          <span v-else-if="currentRemainElement <= 8">{{ $t('game_round_quarterfinal') }}</span>
          <span v-else-if="currentRemainElement <= 16">{{ $t('game_round_of', {round:16}) }}</span>
          <span v-else-if="currentRemainElement <= 32">{{ $t('game_round_of', {round:32}) }}</span>
          <span v-else-if="currentRemainElement <= 64">{{ $t('game_round_of', {round:64}) }}</span>
          <span v-else-if="currentRemainElement <= 128">{{ $t('game_round_of', {round:128}) }}</span>
          <span v-else-if="currentRemainElement <= 256">{{ $t('game_round_of', {round:256}) }}</span>
          <span v-else-if="currentRemainElement <= 512">{{ $t('game_round_of', {round:512}) }}</span>
          <span v-else-if="currentRemainElement <= 1024">{{ $t('game_round_of', {round:1024}) }}</span>
           {{ game.current_round }} / {{ game.of_round }} </h5>
        <h5 class="text-right align-self-center" style="width: 20%">({{ game.remain_elements }} /{{ game.total_elements }})
        
        </h5>
      </div>
    </div>
    <div class="row game-body" v-if="game">
      <!--left part-->
      <div class="col-sm-12 col-md-6 pr-md-1 mb-2 mb-md-0">
        <div class="card game-player left-player" id="left-player">
          <div v-show="isImageSource(le)" class="game-image-container" v-cloak>
            <img @click="clickImage" @error="onImageError(le.id, le.thumb_url2,$event)" class="game-image" :src="le.thumb_url"
              :style="{ height: this.elementHeight + 'px' }">
          </div>
          <div v-show="isDataLoading">
            <div class="d-flex justify-content-center align-items-center" :style="{height: elementHeight}">
              <div class="spinner-border text-primary" role="status">
                <span class="sr-only">Loading...</span>
              </div>
            </div>
          </div>
          <div v-if="isYoutubeSource(le) && !isDataLoading" class="d-flex" @mouseover="videoHoverIn(le, re, true)">
            <youtube :videoId="le.video_id" width="100%" :height="elementHeight" :ref="le.id"
              :player-vars="{ controls: 1, autoplay: 1, rel: 0 , origin: host, playlist: le.video_id, start:le.video_start_second, end:le.video_end_second }">
            </youtube>
          </div>
          <div v-else-if="isYoutubeEmbedSource(le) && !isDataLoading" class="d-flex">
            <YoutubeEmbed v-if="le" :element="le" width="100%" :height="elementHeight" />
          </div>
          <div v-else-if="isVideoSource(le)">
            <video width="100%" :height="elementHeight" loop autoplay controls playsinline :src="le.thumb_url"></video>
          </div>
          <div class="card-body text-center">
            <div class="my-1" style="max-height: 120px" v-if="isMobileScreen">
              <h5 class="my-1 font-size-small">{{ le.title }}</h5>
            </div>
            <div class="my-1" style="height: 120px" v-else>
              <h5 class="my-1">{{ le.title }}</h5>
            </div>
            <button id="left-btn" class="btn btn-primary btn-lg btn-block d-none d-md-block" :disabled="isVoting"
              @click="leftWin()">Vote
            </button>
            <div class="row" v-if="isYoutubeSource(le)">
              <div class="col-3">
                <button class="btn btn-outline-primary btn-lg btn-block d-block d-md-none" :class="{active: isLeftPlaying}" :disabled="isVoting"
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

      <!-- mobile rounds session -->
      <div id="rounds-session" class="col-sm-12 d-md-none">
        <div class="d-flex d-sm-none justify-content-between" style="flex-flow: row wrap">
          <h5 class="">
            <span v-if="currentRemainElement <= 2">{{ $t('game_round_final') }}</span>
            <span v-else-if="currentRemainElement <= 4">{{ $t('game_round_semifinal') }}</span>
            <span v-else-if="currentRemainElement <= 8">{{ $t('game_round_quarterfinal') }}</span>
            <span v-else-if="currentRemainElement <= 16">{{ $t('game_round_of', {round:16}) }}</span>
            <span v-else-if="currentRemainElement <= 32">{{ $t('game_round_of', {round:32}) }}</span>
            <span v-else-if="currentRemainElement <= 64">{{ $t('game_round_of', {round:64}) }}</span>
            <span v-else-if="currentRemainElement <= 128">{{ $t('game_round_of', {round:128}) }}</span>
            <span v-else-if="currentRemainElement <= 256">{{ $t('game_round_of', {round:256}) }}</span>
            <span v-else-if="currentRemainElement <= 512">{{ $t('game_round_of', {round:512}) }}</span>
            <span v-else-if="currentRemainElement <= 1024">{{ $t('game_round_of', {round:1024}) }}</span>
               {{ game.current_round }} / {{ game.of_round }} </h5>
          <h5 class="">({{ game.remain_elements }} /{{ game.total_elements }})</h5>
        </div>
      </div>

      <!--right part-->
      <div class="col-sm-12 col-md-6 pl-md-1 mb-4 mb-md-0">
        <div class="card game-player right-player" :class="{ 'flex-column-reverse': isMobileScreen }" id="right-player">
          <div v-show="isImageSource(re)" class="game-image-container" v-cloak>
            <img @click="clickImage" @error="onImageError(re.id, re.thumb_url2, $event)" class="game-image" :src="re.thumb_url"
              :style="{ height: this.elementHeight + 'px' }">
          </div>
          <div v-show="isDataLoading">
            <div class="d-flex justify-content-center align-items-center" :style="{height: elementHeight}">
              <div class="spinner-border text-danger" role="status">
                <span class="sr-only">Loading...</span>
              </div>
            </div>
          </div>
          <div v-if="isYoutubeSource(re) && !isDataLoading" class="d-flex" @mouseover="videoHoverIn(re, le, false)">
            <youtube :videoId="re.video_id" width="100%" :height="elementHeight" :ref="re.id"
              :player-vars="{ controls: 1, autoplay: 1, rel: 0, origin: host,  playlist: re.video_id, start:re.video_start_second, end:re.video_end_second}">
            </youtube>
          </div>
          <div v-else-if="isYoutubeEmbedSource(re) && !isDataLoading" class="d-flex">
            <YoutubeEmbed v-if="re" :element="re" width="100%" :height="elementHeight"/>
          </div>
          <div v-else-if="isVideoSource(re)">
            <video width="100%" :height="elementHeight" loop autoplay controls playsinline :src="re.thumb_url"></video>
          </div>

          <!-- reverse when device size width less md(768px)-->
          <div class="card-body text-center"
            :class="{ 'flex-column-reverse': isMobileScreen, 'd-flex': isMobileScreen }">
            <div class="my-1 flex-column-reverse d-flex" style="max-height: 120px" v-if="isMobileScreen">
              <h5 class="my-1 font-size-small">{{ re.title }}</h5>
            </div>
            <div class="my-1" style="height: 120px" v-else>
              <h5 class="my-1">{{ re.title }}</h5>
            </div>
            <button id="right-btn" class="btn btn-danger btn-lg btn-block d-none d-md-block" :disabled="isVoting"
              @click="rightWin()">Vote
            </button>
            <div class="row" v-if="isYoutubeSource(re)">
              <div class="col-3">
                <button class="btn btn-outline-danger btn-lg btn-block d-block d-md-none" :class="{active: isRightPlaying}" :disabled="isVoting"
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
      <div :class="{ 'modal-dialog': true, 'modal-lg': !isMobileScreen }">
        <div class="modal-content">
          <div class="modal-header">
            <h5 class="modal-title align-self-center" id="gameSettingPanelLabel">{{ $t('game.setting') }}</h5>
            <div>
              <a class="btn btn-outline-secondary" :href="gameRankUrl">
                {{$t('game.rank')}}&nbsp;<i class="fas fa-trophy"></i>
              </a>
              <a class="btn btn-outline-secondary" href="/">
                {{ $t('game.cancel') }}&nbsp;<i class="fas fa-times"></i>
              </a>
            </div>
          </div>
          <ValidationObserver v-slot="{ invalid }">
            <form @submit.prevent>
              <div class="modal-body">
                <div class="alert alert-danger" v-if="processingGameSerial">
                  <i class="fas fa-exclamation-triangle"></i>{{ $t('game.continue_hint') }}
                  <span class="btn btn-outline-danger" @click="continueGame">
                    
                    {{ $t('game.continue') }}&nbsp;<i class="fas fa-play"></i>
                  </span>
                </div>
                <div class="alert alert-danger" v-if="error403WhenLoad">
                  {{ $t('game.403') }}
                </div>
                <div class="alert alert-warning" v-if="post && post.is_private">
                  {{ $t('game.pivate_text') }}
                </div>
                <div class="card" v-if="post">
                  <div class="card-header text-center">
                    <h3>{{ post.title }}</h3>
                  </div>
                  <div class="row no-gutters">
                    <div class="col-6">
                      <div class="post-element-container">
                        <img v-if="post.element1.type == 'image' || post.element1.video_source == 'youtube' || post.element1.video_source == 'youtube_embed'" @error="onImageError(post.element1.id, post.element1.url2, $event)" :src="post.element1.url"></img>
                        <video v-else :src="post.element1.url+'#t=1'"></video>
                      </div>
                      <h5 class="text-center mt-1 p-1">{{ post.element1.title }}</h5>
                    </div>
                    <div class="col-6">
                      <div class="post-element-container">
                        <img v-if="post.element2.type == 'image' || post.element2.video_source == 'youtube' || post.element2.video_source == 'youtube_embed'" @error="onImageError(post.element2.id, post.element2.url2, $event)" :src="post.element2.url"></img>
                        <video v-else :src="post.element2.url+'#t=1'"></video>
                      </div>
                      <h5 class="text-center mt-1 p-1">{{ post.element2.title }}</h5>
                    </div>
                    <div class="card-body pt-0 text-center">
                      <h5 class="text-break">{{ post.description }}</h5>
                      <div v-if="post.tags.length > 0" class="d-flex flex-wrap">
                        <span class="badge badge-secondary m-1" v-for="tag in post.tags"
                          style="font-size:medium">#{{ tag }}</span>
                      </div>
                      <span class="mt-2 card-text d-flex justify-content-end">
                        <span class="pr-2">
                          <i class="fas fa-play-circle"></i>&nbsp;{{ post.play_count }}
                        </span>
                        <small class="text-muted">{{ post.created_at | datetime }}</small>
                      </span>
                    </div>
                  </div>
                </div>
                <div class="row mt-2" v-if="post">
                  <div class="col-12">
                    <ValidationProvider rules="required" v-slot="{ errors }">
                      <div class="input-group mb-3">
                        <div class="input-group-prepend">
                          <label class="input-group-text" for="elementsCount">{{ $t('Number of participants') }}</label>
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

              <div class="modal-footer mb-sm-0 mb-4">
                <button v-if="post" type="submit" class="btn btn-primary" :disabled="invalid" @click="createGame">
                  {{$t('game.start') }}&nbsp;<i class="fas fa-play"></i>
                </button>
              </div>
            </form>
          </ValidationObserver>
        </div>
      </div>
    </div>
  </div>
</template>

<script>
import { post } from 'jquery';
import Swal from 'sweetalert2';

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
      errorImages: [],
      currentRemainElement: false
    }
  },
  computed: {
    gameRankUrl: function () {
      return this.getRankRoute.replace('_serial', this.postSerial);
    },
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
    // le: function () {
    //   if (this.game) {
    //     return this.game.elements[0]
    //   }
    //   return null;
    // },
    // re: function () {
    //   if (this.game) {
    //     return this.game.elements[1]
    //   }
    //   return null;
    // }
  },
  methods: {
    loadGameSetting: function () {
      axios.get(this.getGameSettingEndpoint)
        .then(res => {
          this.error403WhenLoad = false;
          this.post = res.data.data;
        })
        .catch(error => {
          if (error.response.status === 403) {
            this.error403WhenLoad = true;
          }
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
          this.nextRound(false);
        }).catch(error => {
          if (error.response.status === 403) {
            this.error403WhenLoad = true;
          }
        });
      $('#gameSettingPanel').modal('hide');
    },
    continueGame: function () {
      const gameSerial = this.$cookies.get(this.postSerial);
      if (gameSerial) {
        this.gameSerial = gameSerial;
        this.nextRound(false);
      }
      $('#gameSettingPanel').modal('hide');
    },
    nextRound:  function (reset = true) {
      const url = this.nextRoundEndpoint.replace('_serial', this.gameSerial);
      return axios.get(url)
        .then(res => {
          this.game = res.data.data;
          if(this.game.current_round == 1 || this.currentRemainElement == false){
            this.currentRemainElement = this.game.remain_elements;
          }
          this.le = this.game.elements[0];
          this.re = this.game.elements[1];
        })
        .then(async () => {
          
          if(reset){
            this.resetPlayerPosition();
            this.scrollToLastPosition();
            this.resetPlayingStatus();
            this.errorImages = [];
            this.isDataLoading = false;
            setTimeout(() => {
              $('#left-player').show();
              $('#right-player').show();
              $('#rounds-session').show();
              $('#left-player').css('opacity', '1');
              $('#right-player').css('opacity', '1');
              $('#rounds-session').css('opacity', '1');
              
              this.doPlay(this.le, this.isLeftPlaying, 'left');
              this.doPlay(this.re, this.isRightPlaying, 'right');
            }, 300);
          }else{
            this.doPlay(this.le, this.isLeftPlaying, 'left');
            this.doPlay(this.re, this.isRightPlaying, 'right');
          }
        })
        .catch(error => {
          if (error.response.status === 429) {
            let timerInterval;
            Swal.fire({
              html: this.$t('You have voted too quickly. Please try again later.') + "(<b></b>)",
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
              }
            }).then(result => {
              if (result.dismiss === Swal.DismissReason.timer) {
                this.nextRound();
              }
            });
          }
        }).finally(() => {
          this.isDataLoading = false;
        })
    },
    leftPlay() {
      const myPlayer = this.getPlayer(this.le);
      if (myPlayer) {
        // window.p1 = myPlayer;
        myPlayer.playVideo();
        myPlayer.unMute();
        this.isLeftPlaying = true;
      }

      const theirPlayer = this.getPlayer(this.re);
      if (theirPlayer) {
        // window.p2 = theirPlayer;
        theirPlayer.pauseVideo();
        theirPlayer.mute();
        this.isRightPlaying = false;
      }
    },
    leftWin() {
      this.rememberedScrollPosition = document.documentElement.scrollTop;
      this.isVoting = true;
      let sendWinnerData = () => {
        this.vote(this.le, this.re);
      }
      
      $('#left-player').css('z-index', '100');
      $('#right-player').css('opacity', 0.5);
      if (this.isMobileScreen) {
        $('#rounds-session').animate({opacity: 0}, 500, "linear");
        let winAnimate = $('#left-player').toggleClass('zoom-in').promise();
        let loseAnimate = $('#right-player').animate({ opacity: '0' }, 500, () => {
        }).promise();
        $.when(winAnimate, loseAnimate).then(() => {
          sendWinnerData();
        });
      } else {
        let winAnimate = $('#left-player').animate({ left: '50%' }, 500, () => {
          $('#left-player').delay(500).animate({ top: '-2000' }, 500, () => {
            $('#left-player').hide();
          });
        }).promise();
        let loseAnimate = $('#right-player').animate({ top: '2000' }, 500, () => {
          $('#right-player').hide();
        }).promise();

        $.when(winAnimate, loseAnimate).then(() => {
          this.pauseAllVideo();
          sendWinnerData();
        });
      }
    },
    rightPlay() {
      this.isLeftPlaying = false;
      this.isRightPlaying = true;
      const myPlayer = this.getPlayer(this.re);
      if (myPlayer) {
        myPlayer.playVideo();
        myPlayer.unMute();
      }

      const theirPlayer = this.getPlayer(this.le);
      if (theirPlayer) {
        theirPlayer.pauseVideo();
        theirPlayer.mute();
      }

    },
    rightWin() {
      this.rememberedScrollPosition = document.documentElement.scrollTop;
      this.isVoting = true;
      let sendWinnerData = () => {
        this.vote(this.re, this.le);
      }
      
      $('#right-player').css('z-index', '100');
      $('#left-player').css('opacity', 0.5);
      if (this.isMobileScreen) {
        $('#rounds-session').animate({opacity: 0}, 500, "linear");
        let winAnimate = $('#right-player').toggleClass('zoom-in').promise();
        let loseAnimate = $('#left-player').animate({ opacity: '0' }, 500).promise();
        $.when(winAnimate, loseAnimate).then(() => {
          this.pauseAllVideo();
          sendWinnerData();
        });
      } else {

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
      }
    },
    resetPlayerPosition() {

      $('#left-player').css('left', '0');
      $('#left-player').css('top', '0');
      $('#left-player').css('opacity', '0');
      $('#left-player').css('scale', '1');
      $('#left-player').removeClass('zoom-in');
      $('#left-player').css('z-index', '0');

      $('#right-player').css('left', '0');
      $('#right-player').css('top', '0');
      $('#right-player').css('opacity', '0');
      $('#right-player').css('scale', '1');
      $('#right-player').removeClass('zoom-in');
      $('#right-player').css('z-index', '0');
      
      $('#rounds-session').css('opacity', '0');
      $('.game-image-container img').css('object-fit', 'contain');
    },
    scrollToLastPosition() {
      if (this.rememberedScrollPosition !== null) {
        window.scrollTo(0, this.rememberedScrollPosition);
      }
    },
    pauseAllVideo(){
      const player = this.getPlayer(this.le);
      if (player) {
        player.pauseVideo();
        player.seekTo(this.le.start_second);
      }

      const player2 = this.getPlayer(this.re);
      if (player2) {
        player2.pauseVideo();
        player2.seekTo(this.re.start_second);
      }

    },
    vote: function (winner, loser) {
      const data = {
        'game_serial': this.gameSerial,
        'winner_id': winner.id,
        'loser_id': loser.id
      };

      this.isDataLoading = true;
      return axios.post(this.voteGameEndpoint, data)
        .then(res => {
          this.status = res.data.status;
          if (this.status === 'end_game') {
            this.$cookies.remove(this.postSerial);
            this.showGameResult();
          } else {
            this.$cookies.set(this.postSerial, this.gameSerial, "3d");
            this.nextRound();
          }
        })
        .catch(error => {
          if (error.response.status === 429) {
            Swal.fire({
              icon: 'error',
              toast: true,
              text: this.$t('You have voted too quickly. Please try again later.'),
            });
          }else{
            Swal.fire({
              icon: 'error',
              toast: true,
              text: this.$t('An error occurred. Please try again later.'),
            });
          }
          this.resetPlayerPosition();
          this.scrollToLastPosition();
          this.resetPlayingStatus();
          
          setTimeout(() => {
            $('#left-player').show();
            $('#right-player').show();
            $('#rounds-session').show();
            $('#left-player').css('opacity', '1');
            $('#right-player').css('opacity', '1');
            $('#rounds-session').css('opacity', '1');
            this.isDataLoading = false;
          }, 300);
        }).finally(() => {
          this.isVoting = false;
        })
    },
    resetPlayingStatus() {
      this.isLeftPlaying = false;
      this.isRightPlaying = false;
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
        this.initPlayerEventLister(player, element);
        player.getPlayerState().then((state) => {
          //resumed if video is paused
          if(state === 2){
            player.playVideo();
          }
        });
      }
    },
    initPlayerEventLister(player, element) {
      player.addEventListener('onStateChange', (event) => {
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
    videoHoverIn(myElement, theirElement, left) {
      if (this.isMobileScreen) {
        return;
      }

      const myPlayer = this.getPlayer(myElement);
      if (myPlayer) {
        // window.p1 = myPlayer;
        myPlayer.playVideo();
        myPlayer.unMute();
      }

      const theirPlayer = this.getPlayer(theirElement);
      if (theirPlayer) {
        // window.p2 = theirPlayer;
        // let retry = 0;
        let interval = setInterval(() => {
          theirPlayer.getPlayerState().then((state) => {
            // console.log('retry: '+retry+' | theirPlayer status: '+state);
            if(state === -1 || state === 3){
              theirPlayer.mute();
            }else{
              theirPlayer.pauseVideo();
              theirPlayer.mute();
              clearInterval(interval);
            }
            // retry++;
          });
        }, 100);
      }

      if(left){
        this.isLeftPlaying = true;
        this.isRightPlaying = false;
      }else{
        this.isLeftPlaying = false;
        this.isRightPlaying = true;
      }

    },
    clickImage(event) {
      const obj = $(event.target);
      const size = obj.css('object-fit');
      if (size === 'contain') {
        obj.css('object-fit', 'cover');
      } else if (size === 'cover') {
        obj.css('object-fit', 'contain');
      } else {
        obj.css('object-fit', 'contain');
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
    isYoutubeEmbedSource: function (element) {
      return element.type === 'video' && element.video_source === 'youtube_embed';
    },
    isGfycatSource: function (element) {
      return element.type === 'video' && element.video_source === 'gfycat';
    },
    onImageError: function (id, replaceUrl, event) {
      if(this.errorImages.includes(id)) {
        return;
      }

      if(replaceUrl !== null) {
        event.target.src = replaceUrl;
      }
      this.errorImages.push(id);
    },
  },

  beforeMount() {
    // less md size
    if ($(window).width() < MD_WIDTH_SIZE) {
      this.elementHeight = 200
    }
  }
}

</script>

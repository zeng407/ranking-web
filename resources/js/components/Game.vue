<script>
import Swal from 'sweetalert2';

const MD_WIDTH_SIZE = 768;
const MOBILE_HEIGHT = 700;
export default {
  mounted() {
    if(!this.requirePassword){
      this.loadGameSetting();
    }
    this.showGameSettingPanel();
    this.origin = window.location.origin;
    this.host = window.location.host;
    // update elementHeight to 50% of current window height
    if(!this.isMobileScreen){
      this.elementHeight = Math.max(window.innerHeight * 0.5, 360);
      this.gameBodyHeight = Math.max(this.elementHeight+260, 700);
    }else{
      this.gameBodyHeight = Math.max(this.elementHeight+260, MOBILE_HEIGHT);
    }

  },
  props: {
    postSerial: String,
    getRankRoute: String,
    getGameSettingEndpoint: String,
    nextRoundEndpoint: String,
    createGameEndpoint: String,
    voteGameEndpoint: String,
    requirePassword: Boolean,
    accessEndpoint: String,
  },
  data: function () {
    return {
      origin: '',
      host: '',
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
      showPopover: false,
      refreshAD: false,
      leftImageLoaded: false,
      rightImageLoaded: false,
      creatingGame: false,
      finishingGame: false,
      gameResultUrl: '',
      inputPassword: '',
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
          zoomOut :1,
          reset: 1,
          rotateRight: 1,
        },
        rotatable: true,
      },
      animationShowLeftPlayer: true,
      animationShowRightPlayer: true,
      animationShowRoundSession: true,
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
    processingGame: function () {
      return this.$cookies.get(this.postSerial);
    },
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
    accessGame: function () {
      if(this.inputPassword){
        axios.defaults.headers.common['Authorization'] = this.inputPassword;
      }else{
        this.isInvalidPassword = true;
        return;
      }

      axios.get(this.accessEndpoint)
      .then(response => {
        if (response.status === 200) {
          this.hideInvalidPasswordHint();
          this.loadGameSetting();
        } else {
          this.showInvalidPasswordHint();
          this.knownIncorrectPassword = true;
        }
      })
      .catch(error => {
        if(error.response.status === 403){
          this.showInvalidPasswordHint();
          this.knownIncorrectPassword = true;
        }else if(error.response.status === 429){
          Swal.fire({
            icon: 'error',
            toast: true,
            text: this.$t('You have tried too many times. Please try again later.'),
          });
        }else{
          Swal.fire({
            icon: 'error',
            toast: true,
            text: this.$t('An error occurred. Please try again later.'),
          });
        }
      });
    },
    showInvalidPasswordHint: function () {
      this.invalidPasswordWhenLoad = true;
      Swal.fire({
        icon: 'error',
        position: 'top-end',
        timer: 3000,
        toast: true,
        text: this.$t('game.invalid_password'),
      });
    },
    hideInvalidPasswordHint: function () {
      this.invalidPasswordWhenLoad = false;
    },
    createGame: function () {
      const data = {
        'post_serial': this.postSerial,
        'element_count': this.elementsCount,
        'password': this.inputPassword,
      };
      this.creatingGame = true;
      axios.post(this.createGameEndpoint, data)
        .then(res => {
          this.gameSerial = res.data.game_serial;
          this.nextRound(res.data, false);
        }).catch(error => {
          if (error.response.status === 403) {
            this.error403WhenLoad = true;
          }else if(error.response.status === 422){
            Swal.fire({
              icon: 'error',
              toast: true,
              text: this.$t('The number of elements must be at least 2.'),
            }).then(() => {
              location.reload();
            });
          }else{
            Swal.fire({
              icon: 'error',
              toast: true,
              text: this.$t('An error occurred. Please try again later.'),
            }).then(() => {
              location.reload();
            });
          }
        })
        .finally(() => {
          this.creatingGame = false;
        });

      $('#gameSettingPanel').modal('hide');
    },
    continueGame: function () {
      const gameSerial = this.$cookies.get(this.postSerial);
      if (gameSerial) {
        this.gameSerial = gameSerial;
        this.nextRound(null, false);
      }
      $('#gameSettingPanel').modal('hide');
    },
    hintSelect: function () {
      this.showPopover = true;
      if(this.timeout){
        clearTimeout(this.timeout);
      }
      this.timeout = setTimeout(() => {
        this.showPopover = false;
      }, 3000);
    },
    nextRound:  function (data, reset = true) {
      if(data == null){
        const url = this.nextRoundEndpoint.replace('_serial', this.gameSerial);
        axios.get(url)
          .then(res => {
            this.nextRound(res.data);
          })
          .catch(error => {
            this.handleNextRoundError(data, error);
          });
          return;
      }
      new Promise((resolve, reject) => {
          this.game = data.data;
          if(this.game.current_round == 1 || this.currentRemainElement == false){
            this.currentRemainElement = this.game.remain_elements;
          }
          if(this.le && this.game.elements[0] && this.le.id !== this.game.elements[0].id){
            this.leftImageLoaded = false;
          }
          if(this.re && this.game.elements[1] && this.re.id !== this.game.elements[1].id){
            this.rightImageLoaded = false;
          }
          this.le = this.game.elements[0];
          this.re = this.game.elements[1];
          resolve();
        }).then(() => {
          if(reset){
            this.resetPlayerPosition();
            // this.scrollToLastPosition();
            this.resetPlayingStatus();
            // destroy viwer
            if(this.$refs.rightViewer){
              this.$refs.rightViewer.updateViewer();
            }
            if(this.$refs.leftViewer){
              this.$refs.leftViewer.updateViewer();
            }
            this.errorImages = [];
            this.isDataLoading = false;
            setTimeout(() => {
              this.showAllPlayers();

              this.doPlay(this.le, this.isLeftPlaying, 'left');
              this.doPlay(this.re, this.isRightPlaying, 'right');
            }, 300);
          }else{
            this.doPlay(this.le, this.isLeftPlaying, 'left');
            this.doPlay(this.re, this.isRightPlaying, 'right');
          }
        })
        .catch(error => {
          this.handleNextRoundError(data, error);
        }).finally(() => {
          this.isDataLoading = false;
          this.isVoting = false;
          $('#google-ad-container').css('top', '0');
          if(this.isMobileScreen){
            $('#google-ad2').css('top', '0');
          }

          if(this.needReloadAD()){
            this.reloadGoogleAds();
          }
          this.loadGoogleAds();
        })
    },
    handleNextRoundError(data, error) {
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
            this.nextRound(data);
          }
        });
      }else{
        Swal.fire({
          icon: 'error',
          toast: true,
          text: this.$t('An error occurred. Please try again later.'),
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

      const myVideoPlyaer = this.getVideoPlayer('left-video-player');
      if(myVideoPlyaer){
        myVideoPlyaer.play();
        this.isLeftPlaying = true;
      }

      const theirVideoPlyaer = this.getVideoPlayer('right-video-player');
      if(theirVideoPlyaer){
        theirVideoPlyaer.pause();
        this.isRightPlaying = false;
      }

    },
    leftWin(event) {
      this.rememberedScrollPosition = document.documentElement.scrollTop;
      this.isVoting = true;
      let sendWinnerData = () => {
        this.vote(this.le, this.re);
      }

      this.bounceThumbUp(event.target.children[0]);
      $('#left-player').css('z-index', '100');
      $('#right-player').css('opacity', 0.5);

      this.leftReady = false;

      if (this.isMobileScreen) {
        $('#rounds-session').animate({opacity: 0}, 100, "linear");
        // move #left-plyaer to the certical center of screen
        let scrollPosition = window.scrollY;
        let verticalCenter = $(window).height() / 2 - $('#left-player').height() / 2;
        let playOriginalOffset = $('#left-player').offset().top;
        let titleHeight = $('#game-title').height();
        let screenCenterPosition = Math.max(verticalCenter + scrollPosition - playOriginalOffset, 0);
        screenCenterPosition = Math.min(screenCenterPosition, 350);
        let winAnimate = $('#left-player').animate({top: screenCenterPosition}, null, () => {
          setTimeout(() => {
            this.leftReady = true;
          }, 1200);
        }).promise();


        let adTopPosition = titleHeight + screenCenterPosition;
        $('#google-ad-container').animate({top: adTopPosition});

        let adBottomPosition = -$('#right-player').height() - 30 + screenCenterPosition;
        $('#google-ad2').animate({top: adBottomPosition});

        let loseAnimate = $('#right-player').animate({ opacity: '0' }, 500).promise();
        $.when(loseAnimate).then(() => {
          sendWinnerData();
        });
      } else {
        let winAnimate = $('#left-player').animate({ left: '50%' }, 500, () => {
          $('#left-player').delay(500).animate({ top: '-2000' }, 500, () => {
            this.leftReady = true;
          });
        }).promise();
        let loseAnimate = $('#right-player').animate({ left: '2000' }, 500, () => {
          $('#right-player').css('opacity', '0');
        }).promise();

        $.when(loseAnimate).then(() => {
          sendWinnerData();
        });
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

      const myVideoPlyaer = this.getVideoPlayer('right-video-player');
      if(myVideoPlyaer){
        myVideoPlyaer.play();
        this.isRightPlaying = true;
      }

      const theirVideoPlyaer = this.getVideoPlayer('left-video-player');
      if(theirVideoPlyaer){
        theirVideoPlyaer.pause();
        this.isLeftPlaying = false;
      }

    },
    rightWin(event) {
      this.rememberedScrollPosition = document.documentElement.scrollTop;
      this.isVoting = true;
      let sendWinnerData = () => {
        this.vote(this.re, this.le);
      }

      this.rightReady = false;

      this.bounceThumbUp(event.target.children[0]);
      $('#right-player').css('z-index', '100');
      $('#left-player').css('opacity', 0.5);

      if (this.isMobileScreen) {
        $('#rounds-session').animate({opacity: 0}, 100, "linear");

        // move #right-plyaer to the certical center of screen
        let scrollPosition = window.scrollY;
        let verticalCenter = $(window).height() / 2 - $('#right-player').height() / 2;
        let playOriginalOffset = $('#right-player').offset().top;
        let titleHeight = $('#game-title').height();

        let screenCenterPosition = Math.min(verticalCenter + scrollPosition - playOriginalOffset, 0);
        screenCenterPosition = Math.max(screenCenterPosition, -320);
        // animate right player buttom to top
        let winAnimate = $('#right-player').animate({top: screenCenterPosition}, null, () => {
          setTimeout(() => {
          this.rightReady = true;
          }, 1200);
        }).promise();

        let adTopPosition = titleHeight + screenCenterPosition+ $('#left-player').height() + 30;
        $('#google-ad-container').animate({top: adTopPosition});

        let adBottomPosition = screenCenterPosition;
        $('#google-ad2').animate({top: adBottomPosition});

        let loseAnimate = $('#left-player').animate({ opacity: '0' }, 500).promise();

        $.when(loseAnimate).then(() => {
          sendWinnerData();
        });
      } else {

        let winAnimate = $('#right-player').animate({ left: '-50%' }, 500, () => {
          $('#right-player').delay(500).animate({ top: '-2000' }, 500, () => {
            $('#right-player').hide();
            this.rightReady = true;
          });
        }).promise();

        let loseAnimate = $('#left-player').animate({ left: '-2000' }, 500, () => {
          $('#left-player').css('opacity', '0');
        }).promise();

        $.when(loseAnimate).then(() => {
          sendWinnerData();
        });
      }
    },
    handleLeftLoaded() {
      this.leftImageLoaded = true;
    },
    handleRightLoaded() {
      this.rightImageLoaded = true;
    },
    bounceThumbUp(element) {
      // add class fa-bounce
      $(element).addClass('fa-bounce');
      setTimeout(() => {
        this.removeBoundThumbUp(element);
      }, 1000);
    },
    removeBoundThumbUp(element) {
      // remove class fa-bounce
      $(element).removeClass('fa-bounce');
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
    vote: function (winner, loser) {
      const data = {
        'game_serial': this.gameSerial,
        'winner_id': winner.id,
        'loser_id': loser.id
      };

      this.sendVote(data);
    },
    sendVote(data){
      axios.post(this.voteGameEndpoint, data)
        .then(res => {
          let interval = setInterval(() => {
            // console.log('leftReady: '+this.leftReady+' | rightReady: '+this.rightReady);
            if(this.leftReady && this.rightReady){

              if(this.game.current_round == 1 && this.currentRemainElement == 2){
                // final round
                this.isDataLoading = true;
                this.finishingGame = true;
              }

              if(!this.finishingGame){
                this.pauseAllVideo();
              }

              clearInterval(interval);
              if(this.isMobileScreen){
                Promise.all([
                  $('#left-player').animate({left: 300, opacity:0}, 150).promise(),
                  $('#right-player').animate({left: 300, opacity:0}, 150).promise(),
                  $('#google-ad').animate({top: 100, opacity:0}, 150).promise(),
                  $('#google-ad2').animate({top: 100, opacity:0}, 150).promise()
                ]).then(() => {
                  this.animationShowLeftPlayer = false;
                  this.animationShowRightPlayer = false;
                  this.animationShowRoundSession = false;
                  this.isDataLoading = true;
                  this.handleSendVote(res);
                });
              }else{
                this.isDataLoading = true;
                this.handleSendVote(res);
              }
            }
          }, 10);
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
            }).then(() => {
              location.reload();
            });
          }
          let interval = setInterval(() => {
            if(this.leftReady && this.rightReady){
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

        }).finally(() => {

        });
    },
    showAllPlayers(){
      this.animationShowLeftPlayer = true;
      this.animationShowRightPlayer = true;
      this.animationShowRoundSession = true;
      $('#left-player').show();
      $('#right-player').show();
      $('#rounds-session').show();
      $('#left-player').css('opacity', '1');
      $('#right-player').css('opacity', '1');
      $('#rounds-session').css('opacity', '1');
      if(this.isMobileScreen){
        $('#google-ad').css('opacity', '1');
        $('#google-ad2').css('opacity', '1');
      }
    },
    handleSendVote(res){
      this.status = res.data.status;
      if (this.status === 'end_game') {
        this.$cookies.remove(this.postSerial);
        this.showGameResult();
      } else {
        this.$cookies.set(this.postSerial, this.gameSerial, "30d");
        this.nextRound(res.data);
      }
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
      setTimeout(() => {
        this.gameResultUrl = url;
        window.open(url, '_self');
      }, 1000);
    },
    getYoutubePlayer(element) {
      if(!element){
        return null;
      }
      return _.get(this.$refs, element.id + '.player', null);
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
      if (player = this.getYoutubePlayer(element)) {
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
      else if (player = this.getTwitchPlayer(element)) {
        if(element.video_source === 'twitch_video'){
            player.addEventListener(Twitch.Embed.VIDEO_READY, () => {
              embed.getPlayer().play();
            });
            player.seek(element.video_start_second);
        }else if(element.video_source === 'twitch_clip'){
          //
        }
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
      this.mousePosition = left;

      const myPlayer = this.getYoutubePlayer(myElement);
      if (myPlayer) {
        // window.p1 = myPlayer;
        myPlayer.playVideo();
        myPlayer.unMute();
      }

      const theirPlayer = this.getYoutubePlayer(theirElement);
      if (theirPlayer) {
        // window.p2 = theirPlayer;
        // let retry = 0;
        theirPlayer.getPlayerState().then((state) => {
          if(state === -1 || state === 3){
            let interval = setInterval(() => {
              theirPlayer.getPlayerState().then((state) => {
                // console.log('mouse:' + this.mousePosition + ' left:' + left);

                // console.log('retry: '+retry+' | theirPlayer status: '+state);
                if(state === -1 || state === 3){
                  theirPlayer.mute();
                }else{
                  clearInterval(interval);
                  if(this.mousePosition){
                    this.videoHoverIn(this.le, this.re, true);
                  }else{
                    this.videoHoverIn(this.re, this.le, false);
                  }
                }
                // retry++;
            });
            }, 100);
          }else{
            theirPlayer.pauseVideo();
            theirPlayer.mute();
          }
        });
      }

      let myVideoPlyaer = null;
      let theirVideoPlyaer = null;
      if(left){
        myVideoPlyaer = this.getVideoPlayer('left-video-player');
        theirVideoPlyaer = this.getVideoPlayer('right-video-player');
      }else{
        myVideoPlyaer = this.getVideoPlayer('right-video-player');
        theirVideoPlyaer = this.getVideoPlayer('left-video-player');
      }

      if(myVideoPlyaer){
        myVideoPlyaer.play();
      }
      if(theirVideoPlyaer){
        theirVideoPlyaer.pause();
      }

      if(left){
        this.isLeftPlaying = true;
        this.isRightPlaying = false;
      }else{
        this.isLeftPlaying = false;
        this.isRightPlaying = true;
      }
    },
    isImageSource: function (element) {
      return element.type === 'image';
    },
    isVideoSource: function (element) {
      return element.type === 'video';
    },
    isVideoUrlSource: function (element) {
      return element.type === 'video' && element.video_source === 'url';
    },
    isYoutubeSource: function (element) {
      return element.type === 'video' && element.video_source === 'youtube';
    },
    isTwitchVideoSource: function (element) {
      return element.type === 'video' && element.video_source === 'twitch_video';
    },
    isTwitchClipSource: function (element) {
      return element.type === 'video' && element.video_source === 'twitch_clip';
    },
    isBilibiliSource: function (element) {
      return element.type === 'video' && element.video_source === 'bilibili_video';
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
    loadGoogleAds() {
      try{
        if (window.adsbygoogle) {
          try{
            window.adsbygoogle.push({});
          }catch(e){}
        }
      }catch(e){

      }

    },
    reloadGoogleAds() {
      $('#google-ad2-container').css('height', '360px').css('position', 'relative');
      this.refreshAD = true;
      setTimeout(() => {
        this.refreshAD = false;
      }, 0);

      let retry = 5;
      let interval = setInterval(() => {
        if(retry <= 0){
          clearInterval(interval);
          return;
        }
        retry--;
        if (window.adsbygoogle) {
          try{
            window.adsbygoogle.push({});
          }catch(e){
            if (e.message.includes(`All 'ins' elements in the DOM with class=adsbygoogle already have ads in them`)) {
                clearInterval(interval);
            }
          }
        }
        if($('#google-ad')){
          $('#google-ad').addClass('d-flex justify-content-center');
        }
      }, 500);
    },
    needReloadAD() {
      if(this.refreshAD){
        return false;
      }

      if(!this.game){
        return false;
      }

      return true;
    },
    formatTime(time) {
      // format second to 0h0m0s
      let hour = Math.floor(time / 3600);
      let minute = Math.floor((time % 3600) / 60);
      let second = time % 60;
      return `${hour}h${minute}m${second}s`;
    },
    getThumbUrl(element){
      if(this.isMobileScreen){
        return element.lowthumb_url ? element.lowthumb_url : element.thumb_url;
      }else{
        return element.thumb_url;
      }
    }
  },

  beforeMount() {
    // less md size
    if ($(window).width() < MD_WIDTH_SIZE) {
      this.elementHeight = 200
    }
  }
}

</script>

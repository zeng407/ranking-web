<script>
import Swal from 'sweetalert2';


const MD_WIDTH_SIZE = 768;
export default {
  mounted() {
    if(!this.requirePassword){
      this.loadGameSetting();
    }
    this.showGameSettingPanel();
    this.origin = window.location.origin;
    this.host = window.location.host;
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
      clientWidth: null,
      origin: '',
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
        //console.log(error.response);
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
          this.invalidPasswordWhenLoad = false;
          this.loadGameSetting();
        } else {
          this.invalidPasswordWhenLoad = true;
        }
      })
      .catch(error => {
        if(error.response.status === 403){
          this.invalidPasswordWhenLoad = true;
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
          this.nextRound(false);
        }).catch(error => {
          if (error.response.status === 403) {
            this.error403WhenLoad = true;
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
        this.nextRound(false);
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
    nextRound:  function (reset = true) {
      const url = this.nextRoundEndpoint.replace('_serial', this.gameSerial);
      return axios.get(url)
        .then(res => {
          this.game = res.data.data;
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
          $('#google-ad-container').css('top', '0');
          if(this.needReloadAD()){
            this.reloadGoogleAds();
          }
          this.loadGoogleAds();
        })
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
      if (this.isMobileScreen) {
        $('#rounds-session').animate({opacity: 0}, 500, "linear");
        let winAnimate = $('#left-player').animate({top: 200}, null, () => {
          $('#left-player').delay(500).animate({'opacity': '0'}, 0);
        }).promise();
        $('#google-ad-container').animate({top: 200});
        let loseAnimate = $('#right-player').animate({ opacity: '0' }, 500, () => {
        }).promise();
        $.when(winAnimate, loseAnimate).then(() => {
          this.pauseAllVideo(); // to void still playing video if next round loaded the same element
          sendWinnerData();
        });
      } else {
        let winAnimate = $('#left-player').animate({ left: '50%' }, 500, () => {
          $('#left-player').delay(500).animate({ top: '-2000' }, 500, () => {
            $('#left-player').hide();
          });
        }).promise();
        let loseAnimate = $('#right-player').animate({ top: '2000' }, 500, () => {
          $('#right-player').css('opacity', '0');
        }).promise();

        $.when(winAnimate, loseAnimate).then(() => {
          this.pauseAllVideo(); // to void still playing video if next round loaded the same element
          sendWinnerData();
        });
      }
    },
    rightPlay() {
      this.isLeftPlaying = false;
      this.isRightPlaying = true;
      const myPlayer = this.getYoutubePlayer(this.re);
      if (myPlayer) {
        myPlayer.playVideo();
        myPlayer.unMute();
      }

      const theirPlayer = this.getYoutubePlayer(this.le);
      if (theirPlayer) {
        theirPlayer.pauseVideo();
        theirPlayer.mute();
      }

    },
    rightWin(event) {
      this.rememberedScrollPosition = document.documentElement.scrollTop;
      this.isVoting = true;
      let sendWinnerData = () => {
        this.vote(this.re, this.le);
      }

      this.bounceThumbUp(event.target.children[0]);
      $('#right-player').css('z-index', '100');
      $('#left-player').css('opacity', 0.5);
      if (this.isMobileScreen) {
        $('#rounds-session').animate({opacity: 0}, 500, "linear");
        let winAnimate = $('#right-player').animate({top: -200}, null, () => {
          $('#right-player').delay(500).animate({'opacity': '0'}, 0);
        }).promise();
        $('#google-ad2').animate({top: -200});
        let loseAnimate = $('#left-player').animate({ opacity: '0' }, 500).promise();
        $.when(winAnimate, loseAnimate).then(() => {
          this.pauseAllVideo(); // to void still playing video if next round loaded the same element
          sendWinnerData();
        });
      } else {

        let winAnimate = $('#right-player').animate({ left: '-50%' }, 500, () => {
          $('#right-player').delay(500).animate({ top: '-2000' }, 500, () => {
            $('#right-player').hide();
          });
        }).promise();

        let loseAnimate = $('#left-player').animate({ top: '2000' }, 500, () => {
          $('#left-player').css('opacity', '0');
        }).promise();

        $.when(winAnimate, loseAnimate).then(() => {
          this.pauseAllVideo(); 
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

      this.isDataLoading = true;
      if(this.game.current_round == 1 && this.currentRemainElement == 2){
        // final round
        this.finishingGame = true;
      }
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
    getTwitchPlayer(element) {
      // console.log('getTwitchPlayer');

      //check twitch-video-{{element.id}} is exist
        if (document.getElementById("twitch-video-" + element.id) === null) {
          return null;
        }

      if (element.twitchPlayer === undefined) {
        // console.log('new Twitch.Embed');
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
      // console.log('reloadGoogleAds');
      $('#google-ad2-container').css('height', '300px').css('position', 'relative');
      // $('#google-ad2').css('opacity', '0')
      this.refreshAD = true;
      setTimeout(() => {
        this.refreshAD = false;
      }, 0);

      let interval = setInterval(() => {
        // console.log(window.adsbygoogle);
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
    formatTime: function (time) {
      // format second to 0h0m0s
      let hour = Math.floor(time / 3600);
      let minute = Math.floor((time % 3600) / 60);
      let second = time % 60;
      return `${hour}h${minute}m${second}s`;
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

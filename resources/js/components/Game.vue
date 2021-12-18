<template>

  <div class="container-fluid">
    <div class="text-center" v-if="game">
      <h2>{{game.title}}</h2>
      <h2>{{game.current_round}} / {{game.of_round}}</h2>
    </div>
    <!--    <div class="row justify-content-center pt-sm-4" v-if="game">-->
    <!--      <div class="col-md-6 text-center">-->
    <!--        <img :src="game.elements[0].source_url">-->
    <!--        <h3>{{game.elements[0].title}}</h3>-->
    <!--      </div>-->
    <!--      <div class="col-md-6 text-center">-->
    <!--        <img :src="game.elements[1].source_url">-->
    <!--        <p>{{game.elements[1].title}}</p>-->
    <!--      </div>-->
    <!--    </div>-->
    <div class="row" v-if="game">
      <div class="col-md-6 pr-md-0">
        <div class="card">
          <div v-if="game.elements[0].type === 'image'"
               @click="clickImage"
               :style="{
                'background': 'url('+game.elements[0].source_url+')',
                'width': '100%',
                'height': '600px',
                'background-repeat': 'no-repeat',
                'background-size': 'cover',
                'background-position': 'center center',
                'display': 'flex'
               }"></div>
          <div v-if="game.elements[0].type === 'video'"
               @mouseover="videoHoverIn(game.elements[0])"
               @mouseleave="videoHoverOut(game.elements[0])"
          >
            <youtube :videoId="game.elements[0].video_id"
                     width="100%" height="600"
                     :ref="game.elements[0].id"
                     @ready="doPlay(game.elements[0])"
                     :player-vars="{
                      controls:1,
                      autoplay:1,
                      start: game.elements[0].video_start_second,
                      end:game.elements[0].video_end_second,
                      rel: 0,
                      host: 'https://www.youtube.com'
                     }"
            ></youtube>
          </div>
          <div class="card-body text-center">
            <div style="height: 50px">
              <h5 class="card-title">{{game.elements[0].title}}</h5>
            </div>
            <label class="btn btn-primary btn-lg btn-block"
                   @click="vote(game.elements[0], game.elements[1])">Vote</label>
          </div>
        </div>
      </div>
      <div class="col-md-6 pl-md-0">
        <div class="card">
          <div v-if="game.elements[1].type === 'image'"
               @click="clickImage"
               :style="{
                'background': 'url('+game.elements[1].source_url+')',
                'width': '100%',
                'height': '600px',
                'background-repeat': 'no-repeat',
                'background-size': 'cover',
                'background-position': 'center center',
                'display': 'flex'
               }" ></div>
          <div v-if="game.elements[1].type === 'video'"
               @mouseover="videoHoverIn(game.elements[1])"
               @mouseleave="videoHoverOut(game.elements[1])"
          >
            <youtube :videoId="game.elements[1].video_id"
                     width="100%" height="600"
                     :ref="game.elements[1].id"
                     @ready="doPlay(game.elements[1])"
                     :player-vars="{
                      controls:1,
                      autoplay:1,
                      start: game.elements[1].video_start_second,
                      end:game.elements[1].video_end_second,
                      rel: 0,
                      host: 'https://www.youtube.com'
                     }"
            ></youtube>
          </div>
          <div class="card-body text-center">
            <div style="height: 50px">
              <h5 class="card-title">{{game.elements[1].title}}</h5>
            </div>
            <label class="btn btn-danger btn-lg btn-block"
                   @click="vote(game.elements[1], game.elements[0])">Vote</label>
          </div>
        </div>
      </div>
    </div>

    <!--  Game Result  -->
    <div class="container" v-if="showResult && winner">
      <div>
        <h2>{{game.title}}</h2>
        <img :src="winner.source_url" class="img-fluid" alt="winner.title">
      </div>
      <div class="pt-1">
        <button class="btn btn-primary" @click="seeRank">
          <i class="fas fa-trophy"></i>
          RANK
        </button>
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
      rankRoute: String,
      getGameSettingEndpoint: String,
      nextRoundEndpoint: String,
      createGameEndpoint: String,
      voteGameEndpoint: String
    },
    data: function () {
      return {
        gameSerial: null,
        game: null,
        winner: null,
        status: null,
        showResult: false,
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
      nextRound: function () {
        const url = this.nextRoundEndpoint.replace('_serial', this.gameSerial);
        axios.get(url)
          .then(res => {
            this.game = res.data.data;
          });
      },
      vote: function (winner, loser) {
        //for debug
        // this.winner = winner;

        const data = {
          'game_serial': this.gameSerial,
          'winner_id': winner.id,
          'loser_id': loser.id
        };
        axios.post(this.voteGameEndpoint, data)
          .then(res => {
            this.status = res.data.status;
            if (this.status === 'end_game') {
              this.winner = winner;
              this.showGameResult();
            } else {
              this.nextRound();
            }
          })
      },
      showGameSettingPanel: function () {
        $('#gameSettingPanel').modal('show');
      },
      showGameResult: function () {
        this.showResult = true;
        console.log("showGameResult");
      },
      seeRank() {
        window.open(this.rankRoute);
      },
      getPlayer(element) {
        return _.get(this.$refs, element.id + '.player', null);
      },
      doPlay(element) {
        const player = this.getPlayer(element);
        if (player) {
          player.mute();
          player.loadVideoById({
            videoId: element.video_id,
            startSeconds: element.video_start_second,
            endSeconds: element.video_end_second
          });
        }
      },
      videoHoverIn(element){
        const player = this.getPlayer(element);
        if(player){
          player.unMute();
        }
      },
      videoHoverOut(element){
        const player = this.getPlayer(element);
        if(player){
          player.mute();
        }
      },
      clickImage(event){
        const obj = $(event.target);
        const size = obj.css('background-size');
        if(size === 'contain'){
          obj.css('background-size', 'cover');
        }else if(size === 'cover'){
          obj.css('background-size','contain');
        }
      },
    }
  }

</script>

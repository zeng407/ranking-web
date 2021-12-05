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
          <div
            :style="{'width':'100%','height':'600px','background': 'url(' + game.elements[0].source_url +') no-repeat center', 'background-size': 'cover'}"></div>
          <div class="card-body text-center">
            <h5 class="card-title">{{game.elements[0].title}}</h5>
            <label class="btn btn-primary btn-lg btn-block"
                   @click="vote(game.elements[0], game.elements[1])">Vote</label>
          </div>
        </div>
      </div>
      <div class="col-md-6 pl-md-0">
        <div class="card">
          <div
            :style="{'width':'100%','height':'600px','background': 'url(' + game.elements[1].source_url +') no-repeat center', 'background-size': 'cover'}"></div>
          <div class="card-body text-center">
            <h5 class="card-title">{{game.elements[1].title}}</h5>
            <label class="btn btn-secondary btn-lg btn-block" @click="vote(game.elements[1], game.elements[0])">Vote</label>
          </div>
        </div>
      </div>
    </div>

    <!--    <div>-->
    <!--      <button class="btn btn-primary">showGameSettingPanel</button>-->
    <!--    </div>-->

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
            <h5 class="modal-title" id="gameSettingPanelLabel">Modal title</h5>
            <!--            <button type="button" class="close" data-dismiss="modal" aria-label="Close">-->
            <!--              <span aria-hidden="true">&times;</span>-->
            <!--            </button>-->

          </div>
          <div class="modal-body">
            <div class="input-group mb-3">
              <div class="input-group-prepend">
                <label class="input-group-text" for="elementCount">參戰數</label>
              </div>
              <select v-model="elementCount" class="custom-select" id="elementCount">
                <option selected></option>
                <option value="4">4</option>
                <option value="8">8</option>
                <option value="16">16</option>
              </select>
            </div>
          </div>
          <div class="modal-footer">
            <button type="button" class="btn btn-primary" @click="createGame">開戰!</button>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script>
  export default {
    mounted() {
      console.log('Component mounted.');
      this.showGameSettingPanel();

      //debug
      // this.elementCount = 8;
      // this.createGame();
      // this.showGameResult();
    },
    props: {
      postSerial: String,
      rankRoute: String,
      showGameEndpoint: String,
      createGameEndpoint: String,
      voteGameEndpoint: String
    },
    data: function () {
      return {
        elementCount: null,
        gameSerial: null,
        game: null,
        winner: null,
        status: null,
        showResult: false
      }
    },
    methods: {
      createGame: function () {
        const data = {
          'post_serial': this.postSerial,
          'element_count': this.elementCount
        };
        axios.post(this.createGameEndpoint, data)
          .then(res => {
            this.gameSerial = res.data.game_serial;
            this.nextRound();
          });
        $('#gameSettingPanel').modal('hide');
      },
      nextRound: function () {
        const url = this.showGameEndpoint.replace('_serial', this.gameSerial);
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
      seeRank(){
        window.open(this.rankRoute);
      }
    }
  }

</script>

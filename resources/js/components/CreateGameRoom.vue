<template>
  <div>
    <button class="btn btn-secondary create-game-button" data-toggle="modal" @click.prevent="showModal">
      <h5 class="m-0">
        <i class="fa-solid fa-gamepad">&nbsp;{{$t('game_room.multiplayer_mode')}}</i>
      </h5>
    </button>

    <!-- modal -->
    <div class="modal fade" id="createGameRoomModal" tabindex="-1" aria-labelledby="createGameRoomModalLabel" aria-hidden="true">
      <div class="modal-dialog">
        <div class="modal-content">
          <div class="modal-header">
            <h5 v-if="step == 0" class="modal-title" id="createGameRoomModalLabel">{{$t('game_room.create_game.title')}}</h5>
            <h5 v-if="step == 1" class="modal-title" id="createGameRoomModalLabel">{{$t('game_room.create_game.invite')}}</h5>
            <button type="button" class="close" data-dismiss="modal" aria-label="Close">
              <span aria-hidden="true">&times;</span>
            </button>
          </div>
          <div class="modal-body text-center">
            <div class="row" v-show="step == 0">
              <div class="col-12 col-md-6 my-2">
                <button class="btn btn-outline-dark"
                  @click="updateStep(1)" @mouseenter="flipOnhover" @mouseleave="flipOffhover"
                  data-target="flip-item-1" style="width: 100%;">
                  <h2 >
                    <i id="flip-item-1" class="fa-solid fa-heart"></i>
                  </h2>
                  <h2>{{ $t('game_room.create_game.preference') }}</h2>
                  <hr>
                  <p>{{ $t('game_room.create_game.preference.description') }}</p>
                  <div class="d-flex justify-content-center">
                    <table>
                      <tr>
                        <th class="text-left pr-1">{{ $t('game_room.create_game.preference.leaderboard') }}</th>
                        <th><i class="fa-solid fa-check"></i></th>
                      </tr>
                      <tr>
                        <th class="text-left">{{ $t('game_room.create_game.preference.black_box') }}</th>
                        <th><i class="fa-solid fa-check"></i></th>
                      </tr>
                      <tr>
                        <th class="text-left">{{ $t('game_room.create_game.preference.points') }}</th>
                        <th><i class="fa-solid fa-check"></i></th>
                      </tr>
                      <tr>
                        <th class="text-left">{{ $t('game_room.create_game.preference.combo') }}</th>
                        <th><i class="fa-solid fa-check"></i></th>
                      </tr>
                    </table>
                  </div>
                </button>
              </div>
              <div class="col-12 col-md-6 my-2">
                <button class="btn btn-outline-dark disabled position-relative" disabled >
                  <div class=" h-100 w-100 position-absolute" style="top: 0; left: 0; background: #FFF7;"></div>
                  <h2>
                    <i id="flip-item-2" class="fa-solid fa-users"></i>
                  </h2>
                  <h2>{{ $t('game_room.create_game.majority_rule') }}</h2>
                  <hr>
                  <p>{{ $t('game_room.create_game.majority_rule.description') }}</p>
                  <div class="d-flex justify-content-center">
                    <table>
                      <tr>
                        <th class="text-left pr-1">{{ $t('game_room.create_game.majority_rule.leaderboard_majority') }}</th>
                        <th><i class="fa-solid fa-check"></i></th>
                      </tr>
                      <tr>
                        <th class="text-left pr-1">{{ $t('game_room.create_game.majority_rule.leaderboard_minority') }}</th>
                        <th><i class="fa-solid fa-check"></i></th>
                      </tr>
                      <tr>
                        <th class="text-left">{{ $t('game_room.create_game.majority_rule.black_box') }}</th>
                        <i class="fa-solid fa-xmark"></i>
                      </tr>
                      <tr>
                        <th class="text-left">{{ $t('game_room.create_game.majority_rule.points') }}</th>
                        <th><i class="fa-solid fa-check"></i></th>
                      </tr>
                    </table>
                  </div>
                </button>
                <h4 class="position-absolute top-50 left-50 translate-50-50">{{ $t('game_room.create_game.coming_soon') }}</h4>
              </div>
            </div>
            <div v-show="step == 1">
              <div class="row">
                <div class="col-12 text-left mb-1">
                  <button class="btn btn-outline-dark btn-sm" @click="updateStep(0)">
                    <i class="fa-solid fa-arrow-left"></i>&nbsp;{{ $t('game_room.create_game.back') }}
                  </button>
                </div>
              </div>
              <div class="row">
                <div class="col-12">
                  <h2>{{$t('game_room.create_game.host')}}</h2>
                  <div class="mb-2">
                    <div class="mb-1">
                      <canvas id="gamemode1-qrcode"></canvas>
                    </div>
                    <div v-if="gameRoomUrl">
                      <h4 v-html="$t('game_room.create_game.invite_description')"></h4>
                      <h2 class="break-word">
                        <p>{{ gameRoomUrl }}</p>
                      </h2>
                      <a @click="downloadQrcode" class="btn btn-outline-dark btn-sm">
                        <i class="fa-solid fa-download"></i> {{ $t('Download QR code') }}
                      </a>
                      <copy-link id="game1-room-url" placement="bottom" :url="gameRoomUrl" :text="$t('Copy link')" :after-copy-text="$t('Copied link')"></copy-link>
                    </div>
                  </div>
                </div>
              </div>
            </div>
          </div>
          <div class="modal-footer">
            <button type="button" class="btn btn-secondary" data-dismiss="modal">{{ $t('Close') }}</button>
          </div>
        </div>
      </div>
    </div>


  </div>
</template>

<script>
import QRCode from 'qrcode';

export default {
  components: {
  },
  mounted() {
    this.registerAfterCloseGameRoom();
  },
  props: {
    getRoomEndpoint: {
      type: String,
      required: true,
    },
    createGameRoomEndpoint: {
      type: String,
      required: true,
    },
    getGameSerial: {
      type: Function,
      required: true,
    },
    gameRoomRoute: {
      type: String,
      required: true,
    },
    handleCreatedRoom: {
      type: Function,
    },

  },
  data() {
    return {
      roomSerial: '',
      step: 0,
      gameRoomUrl: '',
      gameBetRanks: null,
      gameRoom: ''
    }
  },
  computed: {

  },
  methods: {
    showModal(){
      $('#createGameRoomModal').modal('show');
    },
    getRoom(){
      if(!this.getRoomEndpoint || !this.roomSerial){
        return;
      }
      const route = this.getRoomEndpoint.replace('_serial', this.roomSerial);
      axios.get(route)
      .then(response => {
        this.gameRoom = response.data.data;
      })
      .catch(error => {
        console.log(error);
      });

    },
    createGameRoom() {
      if(!this.getGameSerial || !this.createGameRoomEndpoint){
        return;
      }

      const gameSerial = this.getGameSerial();
      axios.post(this.createGameRoomEndpoint, {
        game_serial: gameSerial
      })
      .then(response => {
        this.roomSerial = response.data.data.serial;
        this.gameRoomUrl = this.gameRoomRoute.replace('_serial', this.roomSerial);
        this.handleCreatedRoom(response.data.data, this.gameRoomUrl);
        Vue.nextTick(() => {
          this.rederQRcode('gamemode1-qrcode', this.gameRoomUrl);
        });
      })
      .catch(error => {
        console.log(error);
      });
    },
    registerAfterCloseGameRoom(){
      this.$bus.$on('closeGameRoom', ($event) => {
        this.step = 0;
      });
    },
    updateStep(step){
      this.step = step;
      this.createGameRoom();
    },
    flipOnhover(event){
      const target = event.target.getAttribute('data-target');
      const targetElement = document.getElementById(target);
      if(targetElement){
        targetElement.classList.add('fa-flip');
      }
    },
    flipOffhover(event){
      const target = event.target.getAttribute('data-target');
      const targetElement = document.getElementById(target);
      if(targetElement){
        targetElement.classList.remove('fa-flip');
      }
    },
    rederQRcode(target, text){
      Vue.nextTick(() => {
        const canvas = document.getElementById(target);
        if(canvas){
          QRCode.toCanvas(canvas, text, {
            width: canvas.offsetWidth,
          });
        }
      });
    },
    downloadQrcode(){
      const canvas = document.getElementById('gamemode1-qrcode');
      const link = document.createElement('a');
      link.download = 'game-room-qrcode.png';
      link.href = canvas.toDataURL('image/png');
      link.click();
    }

  }
}
</script>

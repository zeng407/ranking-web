<template>
  <div>
    <button class="btn btn-secondary" data-toggle="modal" data-target="#createGameRoomModal">
      <h5 class="m-0">
        <i class="fa-solid fa-gamepad">多人模式</i>
      </h5>
    </button>

    <!-- modal -->
    <div class="modal fade" id="createGameRoomModal" tabindex="-1" aria-labelledby="createGameRoomModalLabel" aria-hidden="true">
      <div class="modal-dialog">
        <div class="modal-content">
          <div class="modal-header">
            <h5 v-if="step == 0" class="modal-title" id="createGameRoomModalLabel">選擇遊戲模式</h5>
            <h5 v-if="step == 1" class="modal-title" id="createGameRoomModalLabel">邀請朋友加入遊戲</h5>
            <button type="button" class="close" data-dismiss="modal" aria-label="Close">
              <span aria-hidden="true">&times;</span>
            </button>
          </div>
          <div class="modal-body text-center">
            <div class="row" v-show="step == 0">
              <div class="col-12 col-md-6 my-2">
                <button class="btn btn-outline-dark"
                  @click="updateStep(1)" @mouseenter="flipOnhover" @mouseleave="flipOffhover"
                  data-target="flip-item-1" style="height: 350px; width: 100%;">
                  <h2>
                    <i id="flip-item-1" class="fa-solid fa-heart"></i>
                  </h2>
                  <h2>猜喜好</h2>
                  <hr>
                  <p>每回合投票時，讓朋友猜測你的喜好，猜對得分，猜錯扣分</p>
                  <div class="d-flex justify-content-center">
                    <table>
                      <tr>
                        <th class="text-left pr-1">真愛排行榜</th>
                        <th><i class="fa-solid fa-check"></i></th>
                      </tr>
                      <tr>
                        <th class="text-left">黑箱</th>
                        <th><i class="fa-solid fa-check"></i></th>
                      </tr>
                      <tr>
                        <th class="text-left">積分制</th>
                        <th><i class="fa-solid fa-check"></i></th>
                      </tr>
                      <tr>
                        <th class="text-left">連擊Combo!</th>
                        <th><i class="fa-solid fa-check"></i></th>
                      </tr>
                    </table>
                  </div>
                </button>
              </div>
              <div class="col-12 col-md-6 my-2">
                <button class="btn btn-outline-dark disabled position-relative" disabled style="height: 350px;">
                  <div class=" h-100 w-100 position-absolute" style="top: 0; left: 0; background: #FFF7;"></div>
                  <h2>
                    <i id="flip-item-2" class="fa-solid fa-users"></i>
                  </h2>
                  <h2>多數決</h2>
                  <hr>
                  <p>每回合投票時，較多票數的一方獲得大眾積分，較少票數的一方獲得獨特積分</p>
                  <div class="d-flex justify-content-center">
                    <table>
                      <tr>
                        <th class="text-left pr-1">大眾品味排行榜</th>
                        <th><i class="fa-solid fa-check"></i></th>
                      </tr>
                      <tr>
                        <th class="text-left pr-1">獨特品味排行榜</th>
                        <th><i class="fa-solid fa-check"></i></th>
                      </tr>
                      <tr>
                        <th class="text-left">黑箱</th>
                        <i class="fa-solid fa-xmark"></i>
                      </tr>
                      <tr>
                        <th class="text-left">積分制</th>
                        <th><i class="fa-solid fa-check"></i></th>
                      </tr>
                    </table>
                  </div>
                </button>
                <h4 class="position-absolute top-50 left-50 translate-50-50">敬請期待</h4>
              </div>
            </div>
            <div v-show="step == 1">
              <div class="row">
                <div class="col-12 text-left mb-1">
                  <button class="btn btn-outline-dark btn-sm" @click="updateStep(0)">
                    <i class="fa-solid fa-arrow-left"></i>&nbsp;返回
                  </button>
                </div>
              </div>
              <div class="row">
                <div class="col-12">
                  <h4><u>掃描</u>或<u>透過連結</u>加入遊戲</h4>
                  <div class="mb-2">
                    <div class="mb-1">
                      <canvas id="gamemode1-qrcode"></canvas>
                    </div>
                    <div v-if="gameRoomUrl">
                      <h2>
                        <a :href="gameRoomUrl" target="_blank">{{ gameRoomUrl }}</a>
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
